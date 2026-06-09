package audit

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// ActiveLogFileName is the file name used for the rolling audit log
	// while it's the current day's file. Rotated copies use the
	// audit-YYYY-MM-DD.log[.gz] naming.
	ActiveLogFileName = "audit.log"

	// auditDirMode is the directory mode the writer creates if the audit
	// directory doesn't exist yet. 0o750 keeps the log readable by the
	// owning user and group (for log shippers) but not world-readable.
	auditDirMode = 0o750

	// auditFileMode is the file mode used for both the active audit.log
	// and rotated audit-YYYY-MM-DD.log.gz files. Same group-readable,
	// not-world-readable posture as the directory.
	auditFileMode = 0o640
)

// JSONLSink writes one JSON line per audit event to a rolling file.
// Rotation happens lazily on each Write call: if the active file
// was opened on a different UTC day than the current event, the
// active file gets renamed to audit-YYYY-MM-DD.log, gzipped, and a
// fresh audit.log opens for the new day.
//
// All file operations are scoped to a single *os.Root opened on the
// audit directory at construction time. The fixed root constrains
// every Open/Rename/Remove call to stay inside the audit dir, which
// both prevents directory traversal and signals to gosec that the
// I/O surface is bounded.
//
// Writes are serialized by an internal mutex. The sink is safe for
// concurrent use; an explicit Close drains nothing because writes
// are synchronous.
//
// Phase 2a keeps writes synchronous. If real-world latency requires
// it, a later phase can swap in a channel-fed background goroutine
// without changing the Sink contract.
type JSONLSink struct {
	root       *os.Root
	dir        string
	clock      func() time.Time
	file       *os.File
	onWriteErr func(error)
	openDay    string
	mu         sync.Mutex
	closed     bool
}

// JSONLSinkOption configures a JSONLSink at construction time.
// Concrete options: [WithClock], [WithWriteErrorHandler].
type JSONLSinkOption func(*JSONLSink)

// WithClock returns a JSONLSinkOption that overrides the time
// source used for rotation timing. A nil clock is ignored. The
// override exists so reproducible tests can flip the rotation
// trigger without waiting on real wall-clock midnight.
func WithClock(clock func() time.Time) JSONLSinkOption {
	return func(s *JSONLSink) {
		if clock != nil {
			s.clock = clock
		}
	}
}

// WithWriteErrorHandler overrides the write-error handler. The
// default handler emits a slog.Warn so production callers see drops
// in their logs without crashing the server. Tests inject a custom
// handler to assert on the error directly.
func WithWriteErrorHandler(fn func(error)) JSONLSinkOption {
	return func(s *JSONLSink) {
		if fn != nil {
			s.onWriteErr = fn
		}
	}
}

// NewJSONLSink opens the active audit.log under dir. The directory
// is created if it doesn't exist (mode auditDirMode). The active
// file is opened in append mode with mode auditFileMode.
//
// Returns an error if the directory or file can't be opened; the
// caller decides whether that's fatal (typical) or whether to fall
// back to a NoopSink (acceptable when audit isn't load-bearing).
func NewJSONLSink(dir string, opts ...JSONLSinkOption) (*JSONLSink, error) {
	sink := &JSONLSink{
		dir:        dir,
		clock:      func() time.Time { return time.Now().UTC() },
		onWriteErr: defaultJSONLWriteErrorHandler,
	}

	for _, opt := range opts {
		opt(sink)
	}

	if err := os.MkdirAll(dir, auditDirMode); err != nil {
		return nil, fmt.Errorf("audit: mkdir %s: %w", dir, err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("audit: open root %s: %w", dir, err)
	}

	sink.root = root

	if err := sink.openActive(); err != nil {
		_ = root.Close()

		return nil, err
	}

	return sink, nil
}

// Write implements the Sink interface. Lazily rotates the active
// file when the day changes since open. Write errors are passed to
// the configured error handler; the per-call signature has no error
// return because Sink contracts must not block the tool handler.
//
// A done context skips the write. In production the middleware passes
// a cancellation-detached context, so this never fires; the guard
// just honors the contract for any caller that does pass a live,
// cancelable context.
func (s *JSONLSink) Write(ctx context.Context, event *Event) {
	if ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		s.onWriteErr(ErrJSONLSinkClosed)

		return
	}

	currentDay := utcDayString(s.clock())
	if currentDay != s.openDay {
		if err := s.rotateLocked(); err != nil {
			s.onWriteErr(fmt.Errorf("audit: rotate failed: %w", err))
			// On rotate failure, keep writing to the old file so
			// events aren't lost. The next Write retries rotation.
		}
	}

	line, err := json.Marshal(event)
	if err != nil {
		s.onWriteErr(fmt.Errorf("audit: marshal event: %w", err))

		return
	}

	line = append(line, '\n')

	if _, err := s.file.Write(line); err != nil {
		s.onWriteErr(fmt.Errorf("audit: write line: %w", err))
	}
}

// Close finalizes the active file and the underlying root.
// Idempotent: subsequent calls return nil. After Close, Write
// reports ErrJSONLSinkClosed via the error handler and drops the
// event.
func (s *JSONLSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	var errs []error

	if s.file != nil {
		if err := s.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("audit: close active log: %w", err))
		}

		s.file = nil
	}

	if s.root != nil {
		if err := s.root.Close(); err != nil {
			errs = append(errs, fmt.Errorf("audit: close root: %w", err))
		}

		s.root = nil
	}

	if joined := errors.Join(errs...); joined != nil {
		return fmt.Errorf("audit: close jsonl sink: %w", joined)
	}

	return nil
}

// Path returns the absolute path of the active audit.log. Useful
// for tests and for the Phase 2c reader, which needs to know where
// to scan.
func (s *JSONLSink) Path() string {
	return filepath.Join(s.dir, ActiveLogFileName)
}

// openActive opens (or creates) the active audit.log via the
// scoped root. Caller must hold s.mu, or be a constructor that
// hasn't published the sink yet.
func (s *JSONLSink) openActive() error {
	file, err := s.root.OpenFile(ActiveLogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
	if err != nil {
		return fmt.Errorf("audit: open %s: %w", filepath.Join(s.dir, ActiveLogFileName), err)
	}

	s.file = file
	s.openDay = utcDayString(s.clock())

	return nil
}

// rotateLocked closes the active file, renames it to the dated
// form, gzip-compresses the rename target, and opens a fresh
// audit.log. Caller MUST hold s.mu.
//
// The rotated file's date is the OLD openDay value, not today: the
// rotation fires "the day has rolled over" so the file being closed
// is yesterday's data.
func (s *JSONLSink) rotateLocked() error {
	if s.file == nil {
		return s.openActive()
	}

	oldDay := s.openDay
	rotatedName := fmt.Sprintf("audit-%s.log", oldDay)

	if err := s.file.Close(); err != nil {
		return fmt.Errorf("close active: %w", err)
	}

	s.file = nil

	if err := s.root.Rename(ActiveLogFileName, rotatedName); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", ActiveLogFileName, rotatedName, err)
	}

	if err := gzipFileInRoot(s.root, rotatedName); err != nil {
		return fmt.Errorf("gzip %s: %w", rotatedName, err)
	}

	return s.openActive()
}

// utcDayString formats a time as YYYY-MM-DD in UTC. Used as the
// rotation key (same-day-or-not) and the rotated-file name segment.
func utcDayString(timestamp time.Time) string {
	return timestamp.UTC().Format("2006-01-02")
}

// gzipFileInRoot compresses name -> name.gz inside the scoped root,
// deleting the uncompressed source on success. On failure the
// uncompressed source is left in place so the data isn't lost; the
// partially-written .gz file is cleaned up.
func gzipFileInRoot(root *os.Root, name string) error {
	source, err := root.Open(name)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}

	defer func() { _ = source.Close() }()

	dest, err := root.OpenFile(name+".gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, auditFileMode)
	if err != nil {
		return fmt.Errorf("open dest: %w", err)
	}

	gzipWriter := gzip.NewWriter(dest)

	if _, copyErr := io.Copy(gzipWriter, source); copyErr != nil {
		_ = gzipWriter.Close()
		_ = dest.Close()
		_ = root.Remove(name + ".gz")

		return fmt.Errorf("copy: %w", copyErr)
	}

	if err := gzipWriter.Close(); err != nil {
		_ = dest.Close()
		_ = root.Remove(name + ".gz")

		return fmt.Errorf("close gzip writer: %w", err)
	}

	if err := dest.Close(); err != nil {
		_ = root.Remove(name + ".gz")

		return fmt.Errorf("close dest: %w", err)
	}

	if err := source.Close(); err != nil {
		return fmt.Errorf("close source: %w", err)
	}

	if err := root.Remove(name); err != nil {
		return fmt.Errorf("remove uncompressed: %w", err)
	}

	return nil
}

// defaultJSONLWriteErrorHandler emits a slog.Warn for every write
// or rotate error. Audit is a best-effort sink; a single drop should
// not crash the server, but it should leave a breadcrumb.
func defaultJSONLWriteErrorHandler(err error) {
	slog.Warn("audit jsonl sink", "error", err.Error())
}

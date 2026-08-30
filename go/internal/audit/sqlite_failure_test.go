package audit_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// logCapture is a goroutine-safe buffer the retention loop's logger writes
// into. Each write signals the written channel so a test can block on the
// next log line instead of polling.
type logCapture struct {
	written chan struct{}
	buf     bytes.Buffer
	mu      sync.Mutex
}

func newLogCapture() *logCapture {
	// One buffered slot is enough: a signal that arrives while the waiter is
	// re-checking the buffer is kept, and a second one carries no more news.
	return &logCapture{written: make(chan struct{}, 1)}
}

func (c *logCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	written, err := c.buf.Write(p)
	c.mu.Unlock()

	select {
	case c.written <- struct{}{}:
	default:
	}

	if err != nil {
		return written, fmt.Errorf("log capture: %w", err)
	}

	return written, nil
}

func (c *logCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.buf.String()
}

// newLoggerInto returns a text logger that writes into capture.
func newLoggerInto(capture *logCapture) *slog.Logger {
	return slog.New(slog.NewTextHandler(capture, nil))
}

// waitForLog blocks until the capture holds at least count copies of needle,
// failing if the loop under test has not written them within five seconds.
func waitForLog(t *testing.T, capture *logCapture, needle string, count int) {
	t.Helper()

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for strings.Count(capture.String(), needle) < count {
		select {
		case <-capture.written:
		case <-timeout.C:
			t.Fatalf("timed out waiting for %d x %q in the log:\n%s", count, needle, capture.String())
		}
	}
}

// TestNewSQLiteSinkRefusesUnusableStore verifies the constructor fails
// instead of returning a sink that can never insert, for each way the store
// path can be unusable: a parent directory that does not exist, a file that
// is not a SQLite database, and a database whose events table has a
// different shape than the sink's schema.
func TestNewSQLiteSinkRefusesUnusableStore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		prepare func(t *testing.T, dir string) string
		name    string
	}{
		{
			name: "parent directory does not exist",
			prepare: func(t *testing.T, dir string) string {
				t.Helper()

				return filepath.Join(dir, "missing", "audit.db")
			},
		},
		{
			name: "file is not a database",
			prepare: func(t *testing.T, dir string) string {
				t.Helper()

				path := filepath.Join(dir, "audit.db")
				if err := os.WriteFile(path, []byte(notADatabase), 0o600); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return path
			},
		},
		{
			name: "events table lacks the timestamp column",
			prepare: func(t *testing.T, dir string) string {
				t.Helper()

				path := filepath.Join(dir, "audit.db")

				db, err := sql.Open("sqlite", "file:"+path)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				defer func() { _ = db.Close() }()

				if _, err := db.ExecContext(t.Context(), `CREATE TABLE events (event_id TEXT PRIMARY KEY)`); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return path
			},
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			path := tcase.prepare(t, t.TempDir())

			sink, err := audit.NewSQLiteSink(t.Context(), path, 100)
			if err == nil {
				_ = sink.Close()

				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// TestSQLiteSinkDropsEventWithUnencodableArgs verifies an event whose args
// JSON cannot carry is dropped whole: no row lands, so a reader never sees
// an event with an empty or partial args column.
func TestSQLiteSinkDropsEventWithUnencodableArgs(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	evt := makeTestEvent(tcLinodeInstanceList, audit.CapabilityRead, audit.StatusSuccess, day(20, 9))
	evt.EventId = "evt_nan_args"
	evt.Args = map[string]any{"ratio": math.NaN()}

	sink.Write(t.Context(), evt)

	if got := countRows(t, sink); got != 0 {
		t.Errorf("countRows = %d, want 0", got)
	}
}

// TestSQLiteSinkDropsEventWhenStoreTurnsReadOnly verifies a Write against a
// database file that lost write permission after the sink opened is dropped
// rather than crashing the server, and leaves the store readable.
func TestSQLiteSinkDropsEventWhenStoreTurnsReadOnly(t *testing.T) {
	t.Parallel()
	requireNonRoot(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), path, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = sink.Close() })

	// Drop the pooled connection so the next statement opens the file
	// fresh and sees the new mode.
	sink.DB().SetMaxIdleConns(0)

	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	evt := makeTestEvent(tcLinodeInstanceList, audit.CapabilityRead, audit.StatusSuccess, day(20, 9))
	evt.EventId = "evt_readonly"

	sink.Write(t.Context(), evt)

	if got := countRows(t, sink); got != 0 {
		t.Errorf("countRows = %d, want 0", got)
	}
}

// TestSQLiteSweepRetentionAfterCloseReturnsError verifies a sweep against a
// closed sink reports the failure instead of claiming zero rows removed.
func TestSQLiteSweepRetentionAfterCloseReturnsError(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)
	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	removed, err := sink.SweepRetention(t.Context(), time.Now(), 14)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

// TestSQLiteRunRetentionSweepsImmediatelyAndStopsOnCancel verifies the
// loop's first sweep runs before any tick, removing expired rows and
// logging the count, and that canceling the context returns the loop.
func TestSQLiteRunRetentionSweepsImmediatelyAndStopsOnCancel(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	now := time.Now().UTC()
	writeEventAt(t, sink, "evt_expired", now.AddDate(0, 0, -30))
	writeEventAt(t, sink, "evt_fresh", now.AddDate(0, 0, -1))

	capture := newLogCapture()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		sink.RunRetention(ctx, 14, time.Hour, newLoggerInto(capture))
	}()

	waitForLog(t, capture, "removed expired rows", 1)

	cancel()
	<-done

	if got := countRows(t, sink); got != 1 {
		t.Errorf("countRows = %d, want only the fresh row after the immediate sweep", got)
	}

	logged := capture.String()
	if !strings.Contains(logged, "retention started") {
		t.Errorf("log does not announce the loop start:\n%s", logged)
	}

	if !strings.Contains(logged, "rows=1") {
		t.Errorf("log does not report the one removed row:\n%s", logged)
	}
}

// TestSQLiteRunRetentionKeepsTickingAfterSweepFailure verifies a failing
// sweep is logged and the loop keeps running on the next tick rather than
// exiting, so a transient store problem does not silently end retention.
func TestSQLiteRunRetentionKeepsTickingAfterSweepFailure(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)
	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	capture := newLogCapture()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		sink.RunRetention(ctx, 14, 5*time.Millisecond, newLoggerInto(capture))
	}()

	waitForLog(t, capture, "retention sweep failed", 2)

	cancel()
	<-done
}

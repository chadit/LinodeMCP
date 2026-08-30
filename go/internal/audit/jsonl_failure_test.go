package audit_test

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

const (
	// rotatedDay1Log and rotatedDay1Gzip are the names rotation produces for
	// the day the rotation tests close out, so a case can plant a blocker at
	// either step of the rename-then-gzip sequence.
	rotatedDay1Log  = "audit-2026-05-18.log"
	rotatedDay1Gzip = "audit-2026-05-18.log.gz"
	// day1Marker, day2Marker, and day3Marker are result summaries the rotation
	// tests grep for to tell which file an event landed in.
	day1Marker = "day-1-event"
	day2Marker = "day-2-event"
	day3Marker = "day-3-event"
)

// errorCollector gathers every error a sink routes to its handler so a test
// can assert on the whole failure sequence rather than only the last one.
type errorCollector struct {
	errs []error
	mu   sync.Mutex
}

func (c *errorCollector) record(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.errs = append(c.errs, err)
}

func (c *errorCollector) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]string, 0, len(c.errs))
	for _, err := range c.errs {
		out = append(out, err.Error())
	}

	return out
}

// requireNonRoot skips permission-denial cases when the suite runs as root,
// because root ignores mode bits and the denial the case depends on never
// happens.
func requireNonRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission denial is not observable as root")
	}
}

// TestNewJSONLSinkRefusesDirBlockedByRegularFile verifies the constructor
// fails, naming the file that blocks the path and the OS reason, when the
// audit directory cannot be created because a regular file sits in its
// path. Returning a sink here would silently drop every event.
func TestNewJSONLSinkRefusesDirBlockedByRegularFile(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dir := filepath.Join(blocker, "audit")

	_, err := audit.NewJSONLSink(dir)

	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok {
		t.Fatalf("error = %v, want it to wrap the *fs.PathError mkdir produced", err)
	}

	if pathErr.Path != blocker {
		t.Errorf("error names %q, want the blocking file %q", pathErr.Path, blocker)
	}

	if !errors.Is(pathErr, syscall.ENOTDIR) {
		t.Errorf("error = %v, want one wrapping %v", err, syscall.ENOTDIR)
	}
}

// TestNewJSONLSinkRefusesUnreadableDir verifies an existing audit directory
// the process cannot open surfaces as a permission error from the
// constructor rather than from the first Write.
func TestNewJSONLSinkRefusesUnreadableDir(t *testing.T) {
	t.Parallel()
	requireNonRoot(t)

	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Restore the mode so TempDir cleanup can remove the directory.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := audit.NewJSONLSink(dir)
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error = %v, want one wrapping %v", err, fs.ErrPermission)
	}
}

// TestNewJSONLSinkRefusesActiveLogThatIsADirectory verifies that when
// audit.log already exists as a directory the constructor reports that
// file and the OS reason instead of handing back a sink with no file
// behind it.
func TestNewJSONLSinkRefusesActiveLogThatIsADirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	activePath := filepath.Join(dir, audit.ActiveLogFileName)
	if err := os.Mkdir(activePath, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := audit.NewJSONLSink(dir)

	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok {
		t.Fatalf("error = %v, want it to wrap the *fs.PathError the open produced", err)
	}

	if pathErr.Path != audit.ActiveLogFileName {
		t.Errorf("error names %q, want %q", pathErr.Path, audit.ActiveLogFileName)
	}

	if !errors.Is(pathErr, syscall.EISDIR) {
		t.Errorf("error = %v, want one wrapping %v", err, syscall.EISDIR)
	}
}

// TestJSONLSinkSkipsWriteWhenContextIsDone verifies a Write carrying an
// already-canceled context appends nothing and reports nothing: the Sink
// contract says a done context means the caller no longer wants the event.
func TestJSONLSinkSkipsWriteWhenContextIsDone(t *testing.T) {
	t.Parallel()

	collector := &errorCollector{}

	sink, err := audit.NewJSONLSink(t.TempDir(), audit.WithWriteErrorHandler(collector.record))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = sink.Close() }()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	sink.Write(ctx, makeEvent(tcLinodeInstanceList, audit.CapabilityRead))

	if size := fileSizeOf(t, sink.Path()); size != 0 {
		t.Errorf("audit.log size = %d, want 0", size)
	}

	if got := collector.messages(); len(got) != 0 {
		t.Errorf("handler errors = %v, want none", got)
	}
}

// TestJSONLSinkReportsArgsItCannotEncode verifies an event whose args hold
// a value JSON cannot carry is dropped through the error handler and never
// leaves a partial line in the log.
func TestJSONLSinkReportsArgsItCannotEncode(t *testing.T) {
	t.Parallel()

	collector := &errorCollector{}

	sink, err := audit.NewJSONLSink(t.TempDir(), audit.WithWriteErrorHandler(collector.record))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = sink.Close() }()

	event := makeEvent(tcLinodeInstanceList, audit.CapabilityRead)
	event.Args = map[string]any{"ratio": math.NaN()}

	sink.Write(t.Context(), event)

	got := collector.messages()
	if len(got) != 1 {
		t.Fatalf("handler errors = %v, want exactly one", got)
	}

	if !strings.Contains(got[0], "NaN") {
		t.Errorf("handler error %q does not name the unencodable value", got[0])
	}

	if size := fileSizeOf(t, sink.Path()); size != 0 {
		t.Errorf("audit.log size = %d, want 0", size)
	}
}

// TestJSONLSinkRotationFailureKeepsDayOneData verifies the two ways a
// rotation can fail on disk, a blocker at the rename target or at the gzip
// target, both reach the error handler and neither loses the events already
// written for the closed day.
func TestJSONLSinkRotationFailureKeepsDayOneData(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// blocker is the directory planted where rotation wants to write.
		blocker string
		// keptIn is the file the day-1 event must still be readable from.
		keptIn string
	}{
		{
			name:    "rename target taken by a directory keeps the active log intact",
			blocker: rotatedDay1Log,
			keptIn:  audit.ActiveLogFileName,
		},
		{
			name:    "gzip target taken by a directory keeps the uncompressed rotated file",
			blocker: rotatedDay1Gzip,
			keptIn:  rotatedDay1Log,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, tcase.blocker), 0o750); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			day1 := time.Date(2026, time.May, 18, 23, 59, 0, 0, time.UTC)
			day2 := time.Date(2026, time.May, 19, 0, 0, 1, 0, time.UTC)
			clockCalls := []time.Time{day1, day1, day2, day2, day2}
			collector := &errorCollector{}

			sink, err := audit.NewJSONLSink(dir,
				audit.WithClock(makeFixedClock(&clockCalls)),
				audit.WithWriteErrorHandler(collector.record),
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			defer func() { _ = sink.Close() }()

			sink.Write(t.Context(), summarizedEvent(day1Marker))
			sink.Write(t.Context(), summarizedEvent(day2Marker))
			sink.Write(t.Context(), summarizedEvent(day3Marker))

			if !anyContains(collector.messages(), "rotate failed") {
				t.Errorf("handler errors = %v, want one reporting the failed rotation", collector.messages())
			}

			if _, statErr := os.Stat(filepath.Join(dir, rotatedDay1Gzip)); statErr == nil && tcase.blocker != rotatedDay1Gzip {
				t.Errorf("%s exists, want no gzip after a failed rotation", rotatedDay1Gzip)
			}

			if !anyContains(readLines(t, filepath.Join(dir, tcase.keptIn)), day1Marker) {
				t.Errorf("%s lost the day-1 event", tcase.keptIn)
			}

			if !anyContains(readLines(t, sink.Path()), day2Marker) {
				t.Errorf("audit.log lost the event whose Write triggered the failed rotation")
			}

			if !anyContains(readLines(t, sink.Path()), day3Marker) {
				t.Errorf("audit.log does not hold the event written after the failed rotation")
			}
		})
	}
}

// TestJSONLSinkRetriesRotationAfterTheBlockerClears verifies the rotation a
// blocked rename could not finish is retried on the next Write once the
// blocker is gone, and that the events the sink kept appending to audit.log
// meanwhile travel into the dated file the retry produces.
func TestJSONLSinkRetriesRotationAfterTheBlockerClears(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	blocker := filepath.Join(dir, rotatedDay1Log)
	if err := os.Mkdir(blocker, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	day1 := time.Date(2026, time.May, 18, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, time.May, 19, 0, 0, 1, 0, time.UTC)
	clockCalls := []time.Time{day1, day1, day2, day2, day2}
	collector := &errorCollector{}

	sink, err := audit.NewJSONLSink(dir,
		audit.WithClock(makeFixedClock(&clockCalls)),
		audit.WithWriteErrorHandler(collector.record),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = sink.Close() }()

	sink.Write(t.Context(), summarizedEvent(day1Marker))
	sink.Write(t.Context(), summarizedEvent(day2Marker))

	if err := os.Remove(blocker); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink.Write(t.Context(), summarizedEvent(day3Marker))

	body := readGzipBody(t, filepath.Join(dir, rotatedDay1Gzip))
	for _, marker := range []string{day1Marker, day2Marker} {
		if !strings.Contains(body, marker) {
			t.Errorf("%s does not hold %s, so the retried rotation lost it", rotatedDay1Gzip, marker)
		}
	}

	lines := readLines(t, sink.Path())
	if len(lines) != 1 || !strings.Contains(lines[0], day3Marker) {
		t.Errorf("audit.log = %v, want only the event written after the retried rotation", lines)
	}
}

// TestJSONLSinkReportsTheEventItCannotPlace verifies that when a failed
// rotation also cannot reopen audit.log the event is reported through the
// handler rather than dropped in silence, so the log records the gap.
func TestJSONLSinkReportsTheEventItCannotPlace(t *testing.T) {
	t.Parallel()
	requireNonRoot(t)

	dir := t.TempDir()

	day1 := time.Date(2026, time.May, 18, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, time.May, 19, 0, 0, 1, 0, time.UTC)
	clockCalls := []time.Time{day1, day2, day2}
	collector := &errorCollector{}

	sink, err := audit.NewJSONLSink(dir,
		audit.WithClock(makeFixedClock(&clockCalls)),
		audit.WithWriteErrorHandler(collector.record),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = sink.Close() }()

	// With the active log gone the rename has nothing to move, and with the
	// directory read-only the sink cannot create a replacement either.
	if err := os.Remove(filepath.Join(dir, audit.ActiveLogFileName)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	sink.Write(t.Context(), summarizedEvent(day2Marker))

	got := collector.messages()
	if !anyContains(got, "rotate failed") {
		t.Errorf("handler errors = %v, want one reporting the failed rotation", got)
	}

	if !anyContains(got, audit.ErrJSONLSinkNoActiveFile.Error()) {
		t.Errorf("handler errors = %v, want one reporting the event had nowhere to land", got)
	}

	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink.Write(t.Context(), summarizedEvent(day3Marker))

	if !anyContains(readLines(t, sink.Path()), day3Marker) {
		t.Errorf("audit.log lost the event written after the directory took writes again")
	}
}

// readGzipBody returns the decompressed contents of the gzip file at path.
func readGzipBody(t *testing.T, path string) string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = file.Close() }()

	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = reader.Close() }()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return string(body)
}

// summarizedEvent builds a finalized read event whose result summary is
// marker, which the rotation tests use to tell events apart on disk.
func summarizedEvent(marker string) *audit.Event {
	return audit.Finalize(
		makeEvent(tcLinodeInstanceList, audit.CapabilityRead),
		audit.StatusSuccess, time.Millisecond, "", marker,
	)
}

// anyContains reports whether any entry holds needle.
func anyContains(entries []string, needle string) bool {
	for _, entry := range entries {
		if strings.Contains(entry, needle) {
			return true
		}
	}

	return false
}

// fileSizeOf returns the size of the file at path so a test can prove a
// dropped event left no bytes behind, not even a bare newline.
func fileSizeOf(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return info.Size()
}

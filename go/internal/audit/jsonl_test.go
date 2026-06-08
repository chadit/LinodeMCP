package audit_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// testProfile is the profile name reused across audit tests. Lifted
// out so goconst doesn't flag the repetition.
const testProfile = "operator"

// TestJSONLSinkAppendsOneLinePerEvent verifies the writer's primary
// contract: every Write produces exactly one JSON-encoded line in
// audit.log, terminated by a newline so downstream tooling can read
// the file with a line scanner.
func TestJSONLSinkAppendsOneLinePerEvent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	sink, err := audit.NewJSONLSink(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	event1 := makeEvent("linode_instance_list", audit.CapabilityRead)
	event1.Finalize(audit.StatusSuccess, 12*time.Millisecond, "", "5 instances")
	sink.Write(t.Context(), &event1)

	event2 := makeEvent("linode_instance_create", audit.CapabilityWrite)
	event2.Finalize(audit.StatusError, 45*time.Millisecond, "boom", "")
	sink.Write(t.Context(), &event2)

	lines := readLines(t, sink.Path())
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want %d", len(lines), 2)
	}

	var got1, got2 audit.Event

	if err := json.Unmarshal([]byte(lines[0]), &got1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := json.Unmarshal([]byte(lines[1]), &got2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got1.Tool != tcLinodeInstanceList {
		t.Errorf("got1.Tool = %v, want %v", got1.Tool, tcLinodeInstanceList)
	}

	if got1.Status != audit.StatusSuccess {
		t.Errorf("got1.Status = %v, want %v", got1.Status, audit.StatusSuccess)
	}

	if got1.LatencyMS != int64(12) {
		t.Errorf("got1.LatencyMS = %v, want %v", got1.LatencyMS, int64(12))
	}

	if got2.Tool != tcLinodeInstanceCreate {
		t.Errorf("got2.Tool = %v, want %v", got2.Tool, tcLinodeInstanceCreate)
	}

	if got2.Status != audit.StatusError {
		t.Errorf("got2.Status = %v, want %v", got2.Status, audit.StatusError)
	}

	if got2.Error == nil {
		t.Fatal("got2.Error is nil")
	}

	if *got2.Error != tcBoom {
		t.Errorf("*got2.Error = %v, want %v", *got2.Error, tcBoom)
	}
}

// TestJSONLSinkRotatesOnDayBoundary verifies that when the clock
// advances past UTC midnight the active log file is renamed to the
// audit-YYYY-MM-DD.log.gz form for the day that just ended, and a
// fresh audit.log opens for the new day.
func TestJSONLSinkRotatesOnDayBoundary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	day1 := time.Date(2026, time.May, 18, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, time.May, 19, 0, 0, 1, 0, time.UTC)

	clockCalls := []time.Time{day1, day1, day2, day2}
	clock := makeFixedClock(&clockCalls)

	sink, err := audit.NewJSONLSink(dir, audit.WithClock(clock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	day1Event := makeEvent("linode_instance_list", audit.CapabilityRead)
	day1Event.Finalize(audit.StatusSuccess, 10*time.Millisecond, "", "day-1-event")
	sink.Write(t.Context(), &day1Event)

	day2Event := makeEvent("linode_instance_get", audit.CapabilityRead)
	day2Event.Finalize(audit.StatusSuccess, 11*time.Millisecond, "", "day-2-event")
	sink.Write(t.Context(), &day2Event)

	rotatedPath := filepath.Join(dir, "audit-2026-05-18.log.gz")

	rotated, err := os.Open(rotatedPath) //nolint:gosec // path is constructed from test tmp dir
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = rotated.Close() }()

	gzReader, err := gzip.NewReader(rotated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = gzReader.Close() }()

	body, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(body), "day-1-event") {
		t.Errorf("string(body) does not contain %v", "day-1-event")
	}

	if strings.Contains(string(body), "day-2-event") {
		t.Errorf("string(body) should not contain %v", "day-2-event")
	}

	_, err = os.Stat(filepath.Join(dir, "audit-2026-05-18.log"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want %v", err, os.ErrNotExist)
	}

	lines := readLines(t, sink.Path())
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want %d", len(lines), 1)
	}

	if !strings.Contains(lines[0], "day-2-event") {
		t.Errorf("lines[0] does not contain %v", "day-2-event")
	}
}

// TestJSONLSinkWriteAfterCloseDropsEvent verifies the closed-state
// contract: Write after Close emits ErrJSONLSinkClosed via the
// error handler and does not panic or write to disk.
func TestJSONLSinkWriteAfterCloseDropsEvent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var (
		handlerMu  sync.Mutex
		handlerErr error
	)

	handler := func(err error) {
		handlerMu.Lock()
		handlerErr = err
		handlerMu.Unlock()
	}

	sink, err := audit.NewJSONLSink(dir, audit.WithWriteErrorHandler(handler))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := makeEvent("linode_instance_list", audit.CapabilityRead)
	event.Finalize(audit.StatusSuccess, time.Millisecond, "", "")
	sink.Write(t.Context(), &event)

	handlerMu.Lock()
	gotErr := handlerErr
	handlerMu.Unlock()

	if !errors.Is(gotErr, audit.ErrJSONLSinkClosed) {
		t.Errorf("error = %v, want %v", gotErr, audit.ErrJSONLSinkClosed)
	}
}

// TestJSONLSinkPathReturnsActiveLogPath verifies the Path accessor.
// Phase 2c's reader will rely on this to find the active file.
func TestJSONLSinkPathReturnsActiveLogPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	sink, err := audit.NewJSONLSink(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = sink.Close() }()

	expected := filepath.Join(dir, "audit.log")
	if sink.Path() != expected {
		t.Errorf("sink.Path() = %v, want %v", sink.Path(), expected)
	}
}

// TestJSONLSinkCloseReturnsNilWhenHealthy verifies the close
// contract: a happy-path Close returns nil and subsequent calls are
// idempotent.
func TestJSONLSinkCloseReturnsNilWhenHealthy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	sink, err := audit.NewJSONLSink(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// makeEvent builds an Event with the fields tests don't care about
// already populated, so each test only has to pick the tool and
// capability that matter.
func makeEvent(tool string, capability audit.Capability) audit.Event {
	return audit.NewEvent(
		tool,
		capability,
		nil,
		"default",
		testProfile,
		"session-1",
		1,
		"0.1.0",
		false,
	)
}

// readLines reads the file at path and returns its non-empty lines.
// Hides the file-open boilerplate so each test reads cleanly.
func readLines(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path) //nolint:gosec // path comes from test tmp dirs
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = file.Close() }()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			lines = append(lines, text)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return lines
}

// makeFixedClock returns a clock function that walks the provided
// slice on each call. Once the slice is exhausted the function
// returns the last value, which keeps the sink stable after the
// caller has exercised the events it cares about.
func makeFixedClock(times *[]time.Time) func() time.Time {
	var (
		clockMu sync.Mutex
		lastVal time.Time
	)

	return func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()

		if len(*times) == 0 {
			return lastVal
		}

		next := (*times)[0]
		*times = (*times)[1:]
		lastVal = next

		return next
	}
}

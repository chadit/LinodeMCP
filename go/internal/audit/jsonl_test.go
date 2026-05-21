package audit_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewJSONLSink must succeed in a fresh tmp dir")

	defer func() {
		require.NoError(t, sink.Close(), "Close must succeed when sink is healthy")
	}()

	event1 := makeEvent("linode_instance_list", audit.CapabilityRead)
	event1.Finalize(audit.StatusSuccess, 12*time.Millisecond, "", "5 instances")
	sink.Write(t.Context(), &event1)

	event2 := makeEvent("linode_instance_create", audit.CapabilityWrite)
	event2.Finalize(audit.StatusError, 45*time.Millisecond, "boom", "")
	sink.Write(t.Context(), &event2)

	lines := readLines(t, sink.Path())
	require.Len(t, lines, 2, "expected two JSON lines, one per Write")

	var got1, got2 audit.Event

	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got1), "line 1 must be valid JSON")
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &got2), "line 2 must be valid JSON")

	assert.Equal(t, "linode_instance_list", got1.Tool, "event 1 tool round-trips through JSONL")
	assert.Equal(t, audit.StatusSuccess, got1.Status, "event 1 status round-trips through JSONL")
	assert.Equal(t, int64(12), got1.LatencyMS, "event 1 latency_ms round-trips")

	assert.Equal(t, "linode_instance_create", got2.Tool, "event 2 tool round-trips through JSONL")
	assert.Equal(t, audit.StatusError, got2.Status, "event 2 status round-trips through JSONL")
	require.NotNil(t, got2.Error, "event 2 must carry the error message")
	assert.Equal(t, "boom", *got2.Error, "event 2 error message round-trips")
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
	require.NoError(t, err, "NewJSONLSink must accept the injected clock")

	defer func() {
		require.NoError(t, sink.Close(), "Close must succeed after rotation")
	}()

	day1Event := makeEvent("linode_instance_list", audit.CapabilityRead)
	day1Event.Finalize(audit.StatusSuccess, 10*time.Millisecond, "", "day-1-event")
	sink.Write(t.Context(), &day1Event)

	day2Event := makeEvent("linode_instance_get", audit.CapabilityRead)
	day2Event.Finalize(audit.StatusSuccess, 11*time.Millisecond, "", "day-2-event")
	sink.Write(t.Context(), &day2Event)

	rotatedPath := filepath.Join(dir, "audit-2026-05-18.log.gz")

	rotated, err := os.Open(rotatedPath) //nolint:gosec // path is constructed from test tmp dir
	require.NoError(t, err, "rotated gzip must exist at %s", rotatedPath)

	defer func() { _ = rotated.Close() }()

	gzReader, err := gzip.NewReader(rotated)
	require.NoError(t, err, "rotated file must be a valid gzip stream")

	defer func() { _ = gzReader.Close() }()

	body, err := io.ReadAll(gzReader)
	require.NoError(t, err, "rotated gzip body must be readable")
	assert.Contains(t, string(body), "day-1-event", "rotated file must contain day-1 event")
	assert.NotContains(t, string(body), "day-2-event", "rotated file must not contain day-2 event")

	_, err = os.Stat(filepath.Join(dir, "audit-2026-05-18.log"))
	require.ErrorIs(t, err, os.ErrNotExist, "uncompressed rotated file must be removed after gzip")

	lines := readLines(t, sink.Path())
	require.Len(t, lines, 1, "post-rotation audit.log holds one event")
	assert.Contains(t, lines[0], "day-2-event", "post-rotation audit.log must contain day-2 event")
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
	require.NoError(t, err, "NewJSONLSink must succeed in a fresh tmp dir")

	require.NoError(t, sink.Close(), "first Close must succeed")
	require.NoError(t, sink.Close(), "second Close must be idempotent")

	event := makeEvent("linode_instance_list", audit.CapabilityRead)
	event.Finalize(audit.StatusSuccess, time.Millisecond, "", "")
	sink.Write(t.Context(), &event)

	handlerMu.Lock()
	gotErr := handlerErr
	handlerMu.Unlock()

	require.Error(t, gotErr, "Write after Close must invoke the error handler")
	assert.ErrorIs(t, gotErr, audit.ErrJSONLSinkClosed, "handler must receive the sentinel")
}

// TestJSONLSinkPathReturnsActiveLogPath verifies the Path accessor.
// Phase 2c's reader will rely on this to find the active file.
func TestJSONLSinkPathReturnsActiveLogPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	sink, err := audit.NewJSONLSink(dir)
	require.NoError(t, err, "NewJSONLSink must succeed")

	defer func() { _ = sink.Close() }()

	expected := filepath.Join(dir, "audit.log")
	assert.Equal(t, expected, sink.Path(), "Path must point at the active audit.log")
}

// TestJSONLSinkCloseReturnsNilWhenHealthy verifies the close
// contract: a happy-path Close returns nil and subsequent calls are
// idempotent.
func TestJSONLSinkCloseReturnsNilWhenHealthy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	sink, err := audit.NewJSONLSink(dir)
	require.NoError(t, err, "NewJSONLSink must succeed")

	require.NoError(t, sink.Close(), "healthy close returns nil")
	require.NoError(t, sink.Close(), "second close is idempotent and returns nil")
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
	)
}

// readLines reads the file at path and returns its non-empty lines.
// Hides the file-open boilerplate so each test reads cleanly.
func readLines(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path) //nolint:gosec // path comes from test tmp dirs
	require.NoError(t, err, "open %s", path)

	defer func() { _ = file.Close() }()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			lines = append(lines, text)
		}
	}

	require.NoError(t, scanner.Err(), "scan %s", path)

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

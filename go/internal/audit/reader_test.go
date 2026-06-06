package audit_test

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// testYear is the fixed year for reader test timestamps. Extracted so
// the day helper doesn't take a year param that every caller fills
// with the same value.
const testYear = 2026

// makeTestEvent builds an event with just the fields the reader tests
// filter on. The rest stay zero; MarshalJSON normalizes nil maps and
// slices so the lines round-trip.
func makeTestEvent(
	tool string,
	capability audit.Capability,
	status audit.Status,
	timestamp time.Time,
) audit.Event {
	return audit.Event{
		TS:             timestamp,
		TSUnixNS:       timestamp.UnixNano(),
		EventID:        "evt_" + tool,
		Tool:           tool,
		ToolCapability: capability,
		Status:         status,
	}
}

// writeJSONLFile writes events as one JSON line each, oldest-first
// (the append order the sink produces). gzipped controls whether the
// file is gzip-compressed like a rotated log.
func writeJSONLFile(t *testing.T, path string, gzipped bool, events []audit.Event) {
	t.Helper()

	file, err := os.Create(path) //nolint:gosec // path from test tmp dir
	mustNoError(t, err, "create %s", path)

	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)

	if gzipped {
		gzWriter := gzip.NewWriter(file)

		defer func() { mustNoError(t, gzWriter.Close(), "close gzip") }()

		encoder = json.NewEncoder(gzWriter)
	}

	for i := range events {
		mustNoError(t, encoder.Encode(&events[i]), "encode event %d", i)
	}
}

// TestReadRecentNewestFirstAcrossFiles verifies events come back
// newest-first across the active log and rotated gzip files, and that
// the active log (today) sorts ahead of older rotated days.
func TestReadRecentNewestFirstAcrossFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Rotated (older) day, gzipped, two events oldest-first.
	writeJSONLFile(t, filepath.Join(dir, "audit-2026-05-18.log.gz"), true, []audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(18, 8)),
		makeTestEvent("tool_b", audit.CapabilityRead, audit.StatusSuccess, day(18, 9)),
	})

	// Active log (today), two events oldest-first.
	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []audit.Event{
		makeTestEvent("tool_c", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
		makeTestEvent("tool_d", audit.CapabilityRead, audit.StatusSuccess, day(19, 9)),
	})

	events, err := audit.ReadRecent(dir, &audit.RecentQuery{})
	mustNoError(t, err, "read must succeed")

	got := make([]string, 0, len(events))
	for i := range events {
		got = append(got, events[i].Tool)
	}

	checkEqual(
		t,
		[]string{"tool_d", "tool_c", "tool_b", "tool_a"}, got,
		"events must be newest-first: active log (today) then rotated (older), reversed within each file",
	)
}

// TestReadRecentLimitClamp verifies the limit clamps the result and
// defaults apply.
func TestReadRecentLimitClamp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	events := make([]audit.Event, 0, 50)

	for i := range 50 {
		ts := day(19, 0).Add(time.Duration(i) * time.Minute)
		events = append(events, makeTestEvent("tool_x", audit.CapabilityRead, audit.StatusSuccess, ts))
	}

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, events)

	got, err := audit.ReadRecent(dir, &audit.RecentQuery{Limit: 10})
	mustNoError(t, err, "read must succeed")
	checkLen(t, got, 10, "explicit limit caps the result")
	checkEqual(t, day(19, 0).Add(49*time.Minute).Unix(), got[0].TS.Unix(),
		"newest event must come first under a limit")

	defaulted, err := audit.ReadRecent(dir, &audit.RecentQuery{Limit: 0})
	mustNoError(t, err, "read must succeed")
	checkLen(t, defaulted, audit.DefaultRecentLimit, "limit 0 falls back to the default")
}

// TestReadRecentFilters exercises every filter dimension.
func TestReadRecentFilters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []audit.Event{
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
		makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, day(19, 9)),
		makeTestEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, day(19, 10)),
		makeTestEvent("linode_volume_create", audit.CapabilityWrite, audit.StatusSuccess, day(19, 11)),
	})

	t.Run("meta excluded by default", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{})
		mustNoError(t, err)

		for i := range got {
			checkNotEqual(t, audit.CapabilityMeta, got[i].ToolCapability,
				"meta events must be excluded unless include_meta is set")
		}

		checkLen(t, got, 3, "three non-meta events")
	})

	t.Run("meta included when requested", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{IncludeMeta: true})
		mustNoError(t, err)
		checkLen(t, got, 4, "all four events when meta is included")
	})

	t.Run("tool glob", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{Tool: "linode_instance_*"})
		mustNoError(t, err)
		checkLen(t, got, 2, "glob matches the two instance tools")
	})

	t.Run("capability exact", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{Capability: audit.CapabilityDestroy})
		mustNoError(t, err)
		mustLen(t, got, 1)
		checkEqual(t, "linode_instance_delete", got[0].Tool)
	})

	t.Run("status exact", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{Status: audit.StatusError})
		mustNoError(t, err)
		mustLen(t, got, 1)
		checkEqual(t, audit.StatusError, got[0].Status)
	})

	t.Run("since/until window", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{
			Since:       day(19, 9),
			Until:       day(19, 10),
			IncludeMeta: true,
		})
		mustNoError(t, err)
		checkLen(t, got, 2, "inclusive bounds keep the 09:00 and 10:00 events")
	})
}

// TestReadRecentMissingDirReturnsEmpty verifies that querying before
// any audit has been written is a normal empty result, not an error.
func TestReadRecentMissingDirReturnsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	got, err := audit.ReadRecent(missing, &audit.RecentQuery{})
	mustNoError(t, err, "missing dir is not an error")
	checkEmpty(t, got, "missing dir yields no events")
}

// TestReadRecentSkipsCorruptLines verifies a malformed JSON line is
// skipped rather than aborting the scan.
func TestReadRecentSkipsCorruptLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	good := makeTestEvent("tool_ok", audit.CapabilityRead, audit.StatusSuccess, day(19, 8))

	line, err := json.Marshal(&good)
	mustNoError(t, err)

	content := "{ this is not json\n" + string(line) + "\n"
	mustNoError(t, os.WriteFile(path, []byte(content), 0o600), "write mixed file")

	got, err := audit.ReadRecent(dir, &audit.RecentQuery{})
	mustNoError(t, err, "corrupt line must not abort the scan")
	mustLen(t, got, 1, "the one valid event is returned")
	checkEqual(t, "tool_ok", got[0].Tool)
}

// day builds a UTC timestamp in testYear, May, at the given day-of-
// month and hour. Year and month are fixed because the reader tests
// only care about relative ordering within a short window.
func day(dayOfMonth, hour int) time.Time {
	return time.Date(testYear, time.May, dayOfMonth, hour, 0, 0, 0, time.UTC)
}

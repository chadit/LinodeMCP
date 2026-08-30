package audit_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// testYear is the fixed year for reader test timestamps. Extracted so
// the day helper doesn't take a year param that every caller fills
// with the same value.
const testYear = 2026

// makeTestEvent builds an event with just the fields the reader tests
// filter on. The rest stay zero; the record writer normalizes nil maps and
// slices so the lines round-trip.
func makeTestEvent(
	tool string,
	capability audit.Capability,
	status audit.Status,
	timestamp time.Time,
) *audit.Event {
	return &audit.Event{
		Ts:             audit.EventTimestamp(timestamp),
		TsUnixNs:       timestamp.UnixNano(),
		EventId:        "evt_" + tool,
		Tool:           tool,
		ToolCapability: string(capability),
		Status:         string(status),
	}
}

// writeJSONLFile writes events as one record per line, oldest-first (the
// append order the sink produces). gzipped controls whether the file is
// gzip-compressed like a rotated log.
//
// The lines go through the package's own NDJSON encoder rather than through a
// second spelling, so a fixture log carries the format the sink writes.
func writeJSONLFile(t *testing.T, path string, gzipped bool, events []*audit.Event) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = file.Close() }()

	var out io.Writer = file

	if gzipped {
		gzWriter := gzip.NewWriter(file)

		defer func() {
			if err := gzWriter.Close(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		out = gzWriter
	}

	if err := audit.EncodeEvents(out, events, audit.ExportFormatNDJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestReadRecentNewestFirstAcrossFiles verifies events come back
// newest-first across the active log and rotated gzip files, and that
// the active log (today) sorts ahead of older rotated days.
func TestReadRecentNewestFirstAcrossFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Rotated (older) day, gzipped, two events oldest-first.
	writeJSONLFile(t, filepath.Join(dir, "audit-2026-05-18.log.gz"), true, []*audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(18, 8)),
		makeTestEvent("tool_b", audit.CapabilityRead, audit.StatusSuccess, day(18, 9)),
	})

	// Active log (today), two events oldest-first.
	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent("tool_c", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
		makeTestEvent("tool_d", audit.CapabilityRead, audit.StatusSuccess, day(19, 9)),
	})

	events, err := audit.ReadRecent(dir, &audit.RecentQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make([]string, 0, len(events))
	for i := range events {
		got = append(got, events[i].Tool)
	}

	if !reflect.DeepEqual(got, []string{"tool_d", "tool_c", "tool_b", "tool_a"}) {
		t.Errorf("got = %v, want %v", got, []string{"tool_d", "tool_c", "tool_b", "tool_a"})
	}
}

// TestReadRecentLimitClamp verifies the limit clamps the result and
// defaults apply.
func TestReadRecentLimitClamp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	events := make([]*audit.Event, 0, 50)

	for i := range 50 {
		ts := day(19, 0).Add(time.Duration(i) * time.Minute)
		events = append(events, makeTestEvent("tool_x", audit.CapabilityRead, audit.StatusSuccess, ts))
	}

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, events)

	got, err := audit.ReadRecent(dir, &audit.RecentQuery{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 10 {
		t.Errorf("len(got) = %d, want %d", len(got), 10)
	}

	if got[0].TsUnixNs != day(19, 0).Add(49*time.Minute).UnixNano() {
		t.Errorf("got[0].TsUnixNs = %v, want %v",
			got[0].TsUnixNs, day(19, 0).Add(49*time.Minute).UnixNano())
	}

	defaulted, err := audit.ReadRecent(dir, &audit.RecentQuery{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defaulted) != audit.DefaultRecentLimit {
		t.Errorf("len(defaulted) = %d, want %d", len(defaulted), audit.DefaultRecentLimit)
	}
}

// TestReadRecentFilters exercises every filter dimension.
func readRecentFilterFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
		makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, day(19, 9)),
		makeTestEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, day(19, 10)),
		makeTestEvent("linode_volume_create", audit.CapabilityWrite, audit.StatusSuccess, day(19, 11)),
	})

	return dir
}

func TestReadRecentFilters(t *testing.T) {
	t.Parallel()

	dir := readRecentFilterFixture(t)

	t.Run("meta excluded by default", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for i := range got {
			if got[i].ToolCapability == string(audit.CapabilityMeta) {
				t.Errorf("got[i].ToolCapability = %v, do not want %v", got[i].ToolCapability, audit.CapabilityMeta)
			}
		}

		if len(got) != 3 {
			t.Errorf("len(got) = %d, want %d", len(got), 3)
		}
	})

	t.Run("meta included when requested", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{IncludeMeta: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 4 {
			t.Errorf("len(got) = %d, want %d", len(got), 4)
		}
	})

	t.Run("tool glob", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{Tool: "linode_instance_*"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 2 {
			t.Errorf("len(got) = %d, want %d", len(got), 2)
		}
	})
}

func TestReadRecentFiltersByField(t *testing.T) {
	t.Parallel()

	dir := readRecentFilterFixture(t)

	t.Run("capability exact", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{Capability: audit.CapabilityDestroy})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want %d", len(got), 1)
		}

		if got[0].Tool != tcLinodeInstanceDelete {
			t.Errorf("got[0].Tool = %v, want %v", got[0].Tool, tcLinodeInstanceDelete)
		}
	})

	t.Run("status exact", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{Status: audit.StatusError})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want %d", len(got), 1)
		}

		if got[0].Status != string(audit.StatusError) {
			t.Errorf("got[0].Status = %v, want %v", got[0].Status, audit.StatusError)
		}
	})

	t.Run("since/until window", func(t *testing.T) {
		t.Parallel()

		got, err := audit.ReadRecent(dir, &audit.RecentQuery{
			Since:       day(19, 9),
			Until:       day(19, 10),
			IncludeMeta: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 2 {
			t.Errorf("len(got) = %d, want %d", len(got), 2)
		}
	})
}

// TestReadRecentMissingDirReturnsEmpty verifies that querying before
// any audit has been written is a normal empty result, not an error.
func TestReadRecentMissingDirReturnsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	got, err := audit.ReadRecent(missing, &audit.RecentQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got = %v, want empty", got)
	}
}

// TestReadRecentSkipsCorruptLines verifies a malformed JSON line is
// skipped rather than aborting the scan.
func TestReadRecentSkipsCorruptLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	good := makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8))

	line, err := genlocal.RecordAuditEvent(good, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := "{ this is not json\n" + string(line) + "\n"
	if writeErr := os.WriteFile(path, []byte(content), 0o600); writeErr != nil {
		t.Fatalf("unexpected error: %v", writeErr)
	}

	got, err := audit.ReadRecent(dir, &audit.RecentQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].Tool != toolOK {
		t.Errorf("got[0].Tool = %v, want %v", got[0].Tool, toolOK)
	}
}

// TestReadRecentSkipsUnopenableFile verifies a listed file the sandboxed
// root refuses to open is skipped, keeping open failures distinct from
// the read failures that abort the scan.
func TestReadRecentSkipsUnopenableFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeJSONLFile(t, filepath.Join(dir, audit.ActiveLogFileName), false, []*audit.Event{
		makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})

	// os.Root refuses a symlink that escapes the audit dir, so opening
	// this listed rotated name fails without permission tricks.
	if err := os.Symlink(filepath.Join(dir, "..", "outside.log"), filepath.Join(dir, "audit-2026-05-18.log")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := audit.ReadRecent(dir, &audit.RecentQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0].Tool != toolOK {
		t.Errorf("got = %v, want one tool_ok event", got)
	}
}

// TestReadRecentSkipsCorruptGzipHeader verifies a rotated .gz whose
// header does not parse is skipped like an open failure, while other
// files still contribute events.
func TestReadRecentSkipsCorruptGzipHeader(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeJSONLFile(t, filepath.Join(dir, audit.ActiveLogFileName), false, []*audit.Event{
		makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})

	if err := os.WriteFile(filepath.Join(dir, "audit-2026-05-18.log.gz"), []byte("not gzip data"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := audit.ReadRecent(dir, &audit.RecentQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0].Tool != toolOK {
		t.Errorf("got = %v, want one tool_ok event", got)
	}
}

// TestReadRecentSurfacesReadError verifies a line past the scanner's
// token cap fails the scan, since a truncated read must not pass for a
// complete one.
func TestReadRecentSurfacesReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 1<<21 is 2MB, past the reader's 1MB scanner token cap.
	oversized := bytes.Repeat([]byte("x"), 1<<21)
	oversized = append(oversized, '\n')

	if err := os.WriteFile(filepath.Join(dir, audit.ActiveLogFileName), oversized, 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := audit.ReadRecent(dir, &audit.RecentQuery{}); !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("err = %v, want %v", err, bufio.ErrTooLong)
	}
}

// TestReadRecentBoundsOutsideTheNanosecondRange verifies a window bound past
// the years a Unix-nanosecond count can hold keeps its meaning instead of
// wrapping: a since that far out matches nothing and an until that far out
// matches everything, with the far past mirroring both.
//
// Reading the bound's own UnixNano wrapped a year-2999 since into the past,
// where it matched the very events the caller asked to exclude.
func TestReadRecentBoundsOutsideTheNanosecondRange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})

	// Both sit outside the 1678-2262 window time.Time.UnixNano can represent.
	farFuture := farFutureBound()
	farPast := time.Date(1000, time.January, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		query *audit.RecentQuery
		name  string
		want  int
	}{
		{name: "since past the range excludes the seeded event", query: &audit.RecentQuery{Since: farFuture}, want: 0},
		{name: "until past the range keeps the seeded event", query: &audit.RecentQuery{Until: farFuture}, want: 1},
		{name: "since before the range keeps the seeded event", query: &audit.RecentQuery{Since: farPast}, want: 1},
		{name: "until before the range excludes the seeded event", query: &audit.RecentQuery{Until: farPast}, want: 0},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			got, err := audit.ReadRecent(dir, tcase.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tcase.want {
				t.Errorf("len(got) = %d, want %d", len(got), tcase.want)
			}
		})
	}
}

// day builds a UTC timestamp in testYear, May, at the given day-of-
// month and hour. Year and month are fixed because the reader tests
// only care about relative ordering within a short window.
func day(dayOfMonth, hour int) time.Time {
	return time.Date(testYear, time.May, dayOfMonth, hour, 0, 0, 0, time.UTC)
}

// farFutureBound is a query bound past the years a Unix-nanosecond count fits
// in an int64, which is the input the saturating-bound cases send.
func farFutureBound() time.Time {
	return time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)
}

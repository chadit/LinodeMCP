package audit_test

import (
	"math"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// writeEventAt writes a minimal event timestamped at ts so retention
// tests can place rows on either side of the cutoff.
func writeEventAt(t *testing.T, sink *audit.SQLiteSink, eventID string, ts time.Time) {
	t.Helper()

	evt := makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, ts)
	evt.EventId = eventID
	evt.TsUnixNs = ts.UnixNano()
	sink.Write(t.Context(), evt)
}

// countRows returns the total number of audit rows in the sink's DB.
func countRows(t *testing.T, sink *audit.SQLiteSink) int {
	t.Helper()

	var count int

	row := sink.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM events`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return count
}

// TestSQLiteSweepRetentionRemovesExpiredKeepsRecent verifies the
// cutoff boundary: rows older than now - retentionDays are deleted,
// recent rows are kept, and the returned count matches.
func TestSQLiteSweepRetentionRemovesExpiredKeepsRecent(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	now := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)

	// retentionDays=14 → cutoff 2026-05-06T12:00Z.
	writeEventAt(t, sink, "evt_old_1", now.AddDate(0, 0, -30))
	writeEventAt(t, sink, "evt_old_2", now.AddDate(0, 0, -15))
	writeEventAt(t, sink, "evt_recent", now.AddDate(0, 0, -1))

	removed, err := sink.SweepRetention(t.Context(), now, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removed != int64(2) {
		t.Errorf("removed = %v, want %v", removed, int64(2))
	}

	if countRows(t, sink) != 1 {
		t.Errorf("countRows(t, sink) = %v, want %v", countRows(t, sink), 1)
	}
}

// TestSQLiteSweepRetentionDisabledWhenZero verifies retentionDays<=0
// is a no-op even with very old rows present.
func TestSQLiteSweepRetentionDisabledWhenZero(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	now := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)
	writeEventAt(t, sink, "evt_ancient", now.AddDate(-5, 0, 0))

	removed, err := sink.SweepRetention(t.Context(), now, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removed != int64(0) {
		t.Errorf("removed = %v, want %v", removed, int64(0))
	}

	if countRows(t, sink) != 1 {
		t.Errorf("countRows(t, sink) = %v, want %v", countRows(t, sink), 1)
	}
}

// TestSQLiteSweepRetentionHugeWindowKeepsEverything pins the saturating
// cutoff. A window whose start predates the earliest representable
// nanosecond keeps every row, because nothing in the table is older than a
// cutoff no row can carry. Before saturation the cutoff wrapped, and a
// wrapped cutoff landed in the future where it matched the whole table.
func TestSQLiteSweepRetentionHugeWindowKeepsEverything(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		retentionDays int
	}{
		// Wider than the ~585 years a nanosecond count covers, so UnixNano
		// is the half that wraps.
		{name: "window wider than the nanosecond range", retentionDays: 200_000_000},
		// Wide enough that AddDate's own day subtraction overflows first.
		{name: "window wider than the day count", retentionDays: math.MaxInt},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			sink := openTestSQLiteSink(t)

			now := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)
			writeEventAt(t, sink, "evt_old", now.AddDate(0, 0, -30))
			writeEventAt(t, sink, "evt_recent", now.AddDate(0, 0, -1))

			removed, err := sink.SweepRetention(t.Context(), now, testCase.retentionDays)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if removed != int64(0) {
				t.Errorf("removed = %v, want %v", removed, int64(0))
			}

			if countRows(t, sink) != 2 {
				t.Errorf("countRows(t, sink) = %v, want %v", countRows(t, sink), 2)
			}
		})
	}
}

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// writeEventAt writes a minimal event timestamped at ts so retention
// tests can place rows on either side of the cutoff.
func writeEventAt(t *testing.T, sink *audit.SQLiteSink, eventID string, ts time.Time) {
	t.Helper()

	evt := makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, ts)
	evt.EventID = eventID
	evt.TSUnixNS = ts.UnixNano()
	sink.Write(t.Context(), &evt)
}

// countRows returns the total number of audit rows in the sink's DB.
func countRows(t *testing.T, sink *audit.SQLiteSink) int {
	t.Helper()

	var count int

	row := sink.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM events`)
	require.NoError(t, row.Scan(&count))

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
	require.NoError(t, err, "sweep must succeed")
	assert.Equal(t, int64(2), removed, "the two pre-cutoff rows must be removed")
	assert.Equal(t, 1, countRows(t, sink), "only the recent row remains")
}

// TestSQLiteSweepRetentionDisabledWhenZero verifies retentionDays<=0
// is a no-op even with very old rows present.
func TestSQLiteSweepRetentionDisabledWhenZero(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	now := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)
	writeEventAt(t, sink, "evt_ancient", now.AddDate(-5, 0, 0))

	removed, err := sink.SweepRetention(t.Context(), now, 0)
	require.NoError(t, err, "disabled sweep must not error")
	assert.Equal(t, int64(0), removed, "retention=0 disables deletion")
	assert.Equal(t, 1, countRows(t, sink), "retention=0 keeps even ancient rows")
}

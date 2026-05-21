package audit_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// TestCollectHealthJSONL verifies the JSONL portion of the report:
// active-log detection, rotated-file count and oldest date, disk
// usage, and that SQLite is absent when no path is given.
func TestCollectHealthJSONL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})
	writeJSONLFile(t, filepath.Join(dir, "audit-2026-05-18.log.gz"), true, []audit.Event{
		makeTestEvent("tool_b", audit.CapabilityRead, audit.StatusSuccess, day(18, 8)),
	})

	report, err := audit.CollectHealth(t.Context(), "", dir)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(dir, "audit.log"), report.JSONLPath)
	assert.True(t, report.ActiveLogExists, "active audit.log must be detected")
	assert.Equal(t, 1, report.RotatedFileCount, "one rotated file present")
	assert.Equal(t, "2026-05-18", report.OldestRotatedDate)
	assert.Positive(t, report.DiskBytes, "disk usage must account for the written files")
	assert.Zero(t, report.DroppedEvents, "synchronous sinks never drop")
	assert.Nil(t, report.SQLite, "SQLite section absent when no path given")
}

// TestCollectHealthSQLite verifies the SQLite portion: row count,
// oldest event timestamp, and a non-zero database size.
func TestCollectHealthSQLite(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	require.NoError(t, err)

	oldest := time.Date(2026, time.May, 18, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, time.May, 20, 8, 0, 0, 0, time.UTC)

	for _, ts := range []time.Time{oldest, newer} {
		evt := makeTestEvent("tool_x", audit.CapabilityRead, audit.StatusSuccess, ts)
		evt.EventID = "evt_" + ts.Format("20060102")
		evt.TSUnixNS = ts.UnixNano()
		sink.Write(t.Context(), &evt)
	}

	require.NoError(t, sink.Close())

	report, err := audit.CollectHealth(t.Context(), dbPath, t.TempDir())
	require.NoError(t, err)

	require.NotNil(t, report.SQLite, "SQLite section present when path given")
	assert.Equal(t, int64(2), report.SQLite.EventCount)
	assert.Equal(t, oldest.UnixNano(), report.SQLite.OldestEventUnixNS, "oldest event timestamp")
	assert.Positive(t, report.SQLite.DBBytes, "database file has non-zero size")
	assert.Equal(t, dbPath, report.SQLite.Path)
}

// TestCollectHealthMissingDirIsEmpty verifies an absent JSONL directory
// reports zero values rather than erroring.
func TestCollectHealthMissingDirIsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	report, err := audit.CollectHealth(t.Context(), "", missing)
	require.NoError(t, err, "missing dir is not an error")

	assert.False(t, report.ActiveLogExists)
	assert.Zero(t, report.RotatedFileCount)
	assert.Empty(t, report.OldestRotatedDate)
	assert.Zero(t, report.DiskBytes)
}

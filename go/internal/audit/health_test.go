package audit_test

import (
	"path/filepath"
	"testing"
	"time"

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
	mustNoError(t, err)

	checkEqual(t, filepath.Join(dir, "audit.log"), report.JSONLPath)
	checkTrue(t, report.ActiveLogExists, "active audit.log must be detected")
	checkEqual(t, 1, report.RotatedFileCount, "one rotated file present")
	checkEqual(t, "2026-05-18", report.OldestRotatedDate)
	checkPositive(t, report.DiskBytes, "disk usage must account for the written files")
	checkZero(t, report.DroppedEvents, "synchronous sinks never drop")
	checkNil(t, report.SQLite, "SQLite section absent when no path given")
}

// TestCollectHealthSQLite verifies the SQLite portion: row count,
// oldest event timestamp, and a non-zero database size.
func TestCollectHealthSQLite(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	mustNoError(t, err)

	oldest := time.Date(2026, time.May, 18, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, time.May, 20, 8, 0, 0, 0, time.UTC)

	for _, ts := range []time.Time{oldest, newer} {
		evt := makeTestEvent("tool_x", audit.CapabilityRead, audit.StatusSuccess, ts)
		evt.EventID = "evt_" + ts.Format("20060102")
		evt.TSUnixNS = ts.UnixNano()
		sink.Write(t.Context(), &evt)
	}

	mustNoError(t, sink.Close())

	report, err := audit.CollectHealth(t.Context(), dbPath, t.TempDir())
	mustNoError(t, err)

	mustNotNil(t, report.SQLite, "SQLite section present when path given")
	checkEqual(t, int64(2), report.SQLite.EventCount)
	checkEqual(t, oldest.UnixNano(), report.SQLite.OldestEventUnixNS, "oldest event timestamp")
	checkPositive(t, report.SQLite.DBBytes, "database file has non-zero size")
	checkEqual(t, dbPath, report.SQLite.Path)
}

// TestCollectHealthMissingDirIsEmpty verifies an absent JSONL directory
// reports zero values rather than erroring.
func TestCollectHealthMissingDirIsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	report, err := audit.CollectHealth(t.Context(), "", missing)
	mustNoError(t, err, "missing dir is not an error")

	checkFalse(t, report.ActiveLogExists)
	checkZero(t, report.RotatedFileCount)
	checkEmpty(t, report.OldestRotatedDate)
	checkZero(t, report.DiskBytes)
}

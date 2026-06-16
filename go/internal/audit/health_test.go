package audit_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.JSONLPath != filepath.Join(dir, "audit.log") {
		t.Errorf("report.JSONLPath = %v, want %v", report.JSONLPath, filepath.Join(dir, "audit.log"))
	}

	if !report.ActiveLogExists {
		t.Error("report.ActiveLogExists = false, want true")
	}

	if report.RotatedFileCount != 1 {
		t.Errorf("report.RotatedFileCount = %v, want %v", report.RotatedFileCount, 1)
	}

	if report.OldestRotatedDate != "2026-05-18" {
		t.Errorf("report.OldestRotatedDate = %v, want %v", report.OldestRotatedDate, "2026-05-18")
	}

	if report.DiskBytes <= 0 {
		t.Errorf("report.DiskBytes = %v, want a positive value", report.DiskBytes)
	}

	if report.DroppedEvents != 0 {
		t.Errorf("report.DroppedEvents = %v, want zero", report.DroppedEvents)
	}

	if report.SQLite != nil {
		t.Errorf("report.SQLite = %v, want nil", report.SQLite)
	}
}

// TestCollectHealthSQLite verifies the SQLite portion: row count,
// oldest event timestamp, and a non-zero database size.
func TestCollectHealthSQLite(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oldest := time.Date(2026, time.May, 18, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, time.May, 20, 8, 0, 0, 0, time.UTC)

	for _, ts := range []time.Time{oldest, newer} {
		evt := makeTestEvent("tool_x", audit.CapabilityRead, audit.StatusSuccess, ts)
		evt.EventID = "evt_" + ts.Format("20060102")
		evt.TSUnixNS = ts.UnixNano()
		sink.Write(t.Context(), &evt)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := audit.CollectHealth(t.Context(), dbPath, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.SQLite == nil {
		t.Fatal("report.SQLite is nil")
	}

	if report.SQLite.EventCount != int64(2) {
		t.Errorf("report.SQLite.EventCount = %v, want %v", report.SQLite.EventCount, int64(2))
	}

	if report.SQLite.OldestEventUnixNS != oldest.UnixNano() {
		t.Errorf("report.SQLite.OldestEventUnixNS = %v, want %v", report.SQLite.OldestEventUnixNS, oldest.UnixNano())
	}

	if report.SQLite.DBBytes <= 0 {
		t.Errorf("report.SQLite.DBBytes = %v, want a positive value", report.SQLite.DBBytes)
	}

	if report.SQLite.Path != dbPath {
		t.Errorf("report.SQLite.Path = %v, want %v", report.SQLite.Path, dbPath)
	}
}

// TestCollectHealthMissingDirIsEmpty verifies an absent JSONL directory
// reports zero values rather than erroring.
func TestCollectHealthMissingDirIsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	report, err := audit.CollectHealth(t.Context(), "", missing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ActiveLogExists {
		t.Error("report.ActiveLogExists = true, want false")
	}

	if report.RotatedFileCount != 0 {
		t.Errorf("report.RotatedFileCount = %v, want zero", report.RotatedFileCount)
	}

	if report.OldestRotatedDate != "" {
		t.Errorf("report.OldestRotatedDate = %v, want empty", report.OldestRotatedDate)
	}

	if report.DiskBytes != 0 {
		t.Errorf("report.DiskBytes = %v, want zero", report.DiskBytes)
	}
}

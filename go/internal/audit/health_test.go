package audit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// TestHealthReadsTheJSONLHalf verifies the JSONL portion of the report:
// active-log detection, rotated-file count and oldest date, disk
// usage, and that SQLite is absent when no path is given.
func TestHealthReadsTheJSONLHalf(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})
	writeJSONLFile(t, filepath.Join(dir, "audit-2026-05-18.log.gz"), true, []*audit.Event{
		makeTestEvent("tool_b", audit.CapabilityRead, audit.StatusSuccess, day(18, 8)),
	})

	report, err := audit.Health(t.Context(), "", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.JsonlPath != filepath.Join(dir, "audit.log") {
		t.Errorf("report.JsonlPath = %v, want %v", report.JsonlPath, filepath.Join(dir, "audit.log"))
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

	if report.Sqlite != nil {
		t.Errorf("report.Sqlite = %v, want nil", report.Sqlite)
	}
}

// TestHealthReadsTheConfiguredStore verifies the SQLite portion: row count,
// oldest event timestamp, and a non-zero database size.
func TestHealthReadsTheConfiguredStore(t *testing.T) {
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
		evt.EventId = "evt_" + ts.Format("20060102")
		evt.TsUnixNs = ts.UnixNano()
		sink.Write(t.Context(), evt)
	}

	if closeErr := sink.Close(); closeErr != nil {
		t.Fatalf("unexpected error: %v", closeErr)
	}

	report, err := audit.Health(t.Context(), dbPath, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Sqlite == nil {
		t.Fatal("report.Sqlite is nil")
	}

	if report.Sqlite.EventCount != int64(2) {
		t.Errorf("report.Sqlite.EventCount = %v, want %v", report.Sqlite.EventCount, int64(2))
	}

	if report.Sqlite.OldestEventUnixNs != oldest.UnixNano() {
		t.Errorf("report.Sqlite.OldestEventUnixNs = %v, want %v", report.Sqlite.OldestEventUnixNs, oldest.UnixNano())
	}

	if report.Sqlite.DbBytes <= 0 {
		t.Errorf("report.Sqlite.DbBytes = %v, want a positive value", report.Sqlite.DbBytes)
	}

	if report.Sqlite.Path != dbPath {
		t.Errorf("report.Sqlite.Path = %v, want %v", report.Sqlite.Path, dbPath)
	}
}

// TestHealthReadsAMissingDirectoryAsEmpty verifies an absent JSONL directory
// reports zero values rather than erroring.
func TestHealthReadsAMissingDirectoryAsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	report, err := audit.Health(t.Context(), "", missing)
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

// The store's own degrade rule, which moved into this package with Health: the
// answer changes data source only when it says so, and refuses only when
// nothing readable answers at all.

// sealDirectory takes every permission off a directory and puts them back when
// the case ends, so the temp tree can still be removed.
func sealDirectory(t *testing.T, dir string) {
	t.Helper()

	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
}

// TestHealthDegradesToTheLogBehindAWarning covers a configured database that
// will not open: the JSONL half still answers and the warning names the store.
func TestHealthDegradesToTheLogBehindAWarning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})

	dbPath := filepath.Join(dir, "audit.db")
	if err := os.WriteFile(dbPath, []byte(notADatabase), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := audit.Health(t.Context(), dbPath, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Sqlite != nil {
		t.Errorf("report.Sqlite = %v, want none once the store did not answer", report.Sqlite)
	}

	if len(report.Warnings) != 1 {
		t.Fatalf("report.Warnings = %v, want one line naming the store", report.Warnings)
	}

	if !strings.Contains(report.Warnings[0], dbPath) {
		t.Errorf("report.Warnings[0] = %v, want it to name %v", report.Warnings[0], dbPath)
	}

	if !report.ActiveLogExists {
		t.Error("report.ActiveLogExists = false, want the log still answering")
	}
}

// TestHealthReportsADirectoryItCannotRead is the only condition Health answers:
// nothing readable behind either sink.
func TestHealthReportsADirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sealDirectory(t, dir)

	if _, err := audit.Health(t.Context(), "", dir); err == nil {
		t.Error("err = nil, want the unreadable directory reported")
	}
}

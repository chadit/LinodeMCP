package audit_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// TestValidateGroupByDefaultsToToolStatus verifies an empty request
// falls back to the documented default grouping.
func TestValidateGroupByDefaultsToToolStatus(t *testing.T) {
	t.Parallel()

	got, err := audit.ValidateGroupBy(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, []string{colTool, colStatus}) {
		t.Errorf("got = %v, want %v", got, []string{colTool, colStatus})
	}
}

// TestValidateGroupByAcceptsAllowed verifies allowlisted columns pass
// through in order.
func TestValidateGroupByAcceptsAllowed(t *testing.T) {
	t.Parallel()

	got, err := audit.ValidateGroupBy([]string{tcCapability, tcProfile, tcEnvironment})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, []string{tcCapability, tcProfile, tcEnvironment}) {
		t.Errorf("got = %v, want %v", got, []string{tcCapability, tcProfile, tcEnvironment})
	}
}

// TestValidateGroupByRejectsUnknown verifies an unknown column is a
// typed error rather than a silent empty grouping.
func TestValidateGroupByRejectsUnknown(t *testing.T) {
	t.Parallel()

	_, err := audit.ValidateGroupBy([]string{colTool, "bogus"})
	if !errors.Is(err, audit.ErrUnknownGroupByColumn) {
		t.Errorf("error = %v, want %v", err, audit.ErrUnknownGroupByColumn)
	}
}

// TestSummarizeCountsByGroup verifies bucketing and count-descending
// ordering.
func TestSummarizeCountsByGroup(t *testing.T) {
	t.Parallel()

	events := []audit.Event{
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 9)),
		makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, day(20, 10)),
	}

	rows := audit.Summarize(events, []string{colTool, colStatus})

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want %d", len(rows), 2)
	}

	if rows[0].Groups[colTool] != tcLinodeInstanceList {
		t.Errorf("rows[0].Groups[colTool] = %v, want %v", rows[0].Groups[colTool], tcLinodeInstanceList)
	}

	if rows[0].Groups[colStatus] != "success" {
		t.Errorf("rows[0].Groups[colStatus] = %v, want %v", rows[0].Groups[colStatus], "success")
	}

	if rows[0].Count != 2 {
		t.Errorf("rows[0].Count = %v, want %v", rows[0].Count, 2)
	}

	if rows[1].Groups[colTool] != tcLinodeInstanceDelete {
		t.Errorf("rows[1].Groups[colTool] = %v, want %v", rows[1].Groups[colTool], tcLinodeInstanceDelete)
	}

	if rows[1].Count != 1 {
		t.Errorf("rows[1].Count = %v, want %v", rows[1].Count, 1)
	}
}

// TestLoadWindowJSONLAndSQLiteAgree verifies both sources return the
// same windowed events (and thus the same summary) for identical
// input.
func TestLoadWindowJSONLAndSQLiteAgree(t *testing.T) {
	t.Parallel()

	events := []audit.Event{
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
		makeTestEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, day(20, 9)),
		makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, day(20, 10)),
	}

	// JSONL source.
	jsonlDir := t.TempDir()
	writeJSONLFile(t, filepath.Join(jsonlDir, "audit.log"), false, events)

	jsonlEvents, err := audit.LoadWindow(t.Context(), "", jsonlDir, time.Time{}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jsonlEvents) != 3 {
		t.Errorf("len(jsonlEvents) = %d, want %d", len(jsonlEvents), 3)
	}

	// SQLite source.
	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for idx := range events {
		sink.Write(t.Context(), &events[idx])
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sqliteEvents, err := audit.LoadWindow(t.Context(), dbPath, "", time.Time{}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sqliteEvents) != 3 {
		t.Errorf("len(sqliteEvents) = %d, want %d", len(sqliteEvents), 3)
	}

	// Both produce the same summary.
	jsonlRows := audit.Summarize(jsonlEvents, []string{colTool})

	sqliteRows := audit.Summarize(sqliteEvents, []string{colTool})
	if !reflect.DeepEqual(sqliteRows, jsonlRows) {
		t.Errorf("sqliteRows = %v, want %v", sqliteRows, jsonlRows)
	}
}

// TestLoadWindowExcludesMetaByDefault verifies include_meta=false
// drops meta events from both sources.
func TestLoadWindowExcludesMetaByDefault(t *testing.T) {
	t.Parallel()

	events := []audit.Event{
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
		makeTestEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, day(20, 9)),
	}

	jsonlDir := t.TempDir()
	writeJSONLFile(t, filepath.Join(jsonlDir, "audit.log"), false, events)

	got, err := audit.LoadWindow(t.Context(), "", jsonlDir, time.Time{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].Tool != tcLinodeInstanceList {
		t.Errorf("got[0].Tool = %v, want %v", got[0].Tool, tcLinodeInstanceList)
	}
}

// TestLoadWindowMissingDirReturnsEmpty verifies querying before any
// audit exists is empty, not an error.
func TestLoadWindowMissingDirReturnsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	got, err := audit.LoadWindow(t.Context(), "", missing, time.Time{}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got = %v, want empty", got)
	}
}

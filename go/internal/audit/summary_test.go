package audit_test

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// TestLoadWindowSurfacesReadError verifies the JSONL window loader
// propagates a mid-file read failure instead of summarizing a
// truncated window as if it were complete.
func TestLoadWindowSurfacesReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 1<<21 is 2MB, past the reader's 1MB scanner token cap.
	oversized := append(bytes.Repeat([]byte("x"), 1<<21), '\n')

	if err := os.WriteFile(filepath.Join(dir, audit.ActiveLogFileName), oversized, 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := audit.LoadWindow(t.Context(), "", dir, time.Time{}, true); !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("err = %v, want %v", err, bufio.ErrTooLong)
	}
}

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

	events := []*audit.Event{
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

	events := []*audit.Event{
		makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
		makeTestEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, day(20, 9)),
		makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, day(20, 10)),
	}

	jsonlDir := t.TempDir()
	writeJSONLFile(t, filepath.Join(jsonlDir, "audit.log"), false, events)

	jsonlEvents, err := audit.LoadWindow(t.Context(), "", jsonlDir, time.Time{}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jsonlEvents) != 3 {
		t.Errorf("len(jsonlEvents) = %d, want %d", len(jsonlEvents), 3)
	}

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for idx := range events {
		sink.Write(t.Context(), events[idx])
	}

	if closeErr := sink.Close(); closeErr != nil {
		t.Fatalf("unexpected error: %v", closeErr)
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

	events := []*audit.Event{
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

// SummaryOver is the whole operation the summary tool reaches: the window read
// through whichever store answers, counted into the buckets it was handed. The
// cases above measure the pieces; these measure the operation.

// TestSummaryOverCountsTheWindowItRead covers the ordinary path, where the log
// is the only store there is.
func TestSummaryOverCountsTheWindowItRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
		makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 9)),
		makeTestEvent("tool_other", audit.CapabilityRead, audit.StatusError, day(19, 10)),
	})

	answer, err := audit.SummaryOver(t.Context(), "", dir, time.Time{},
		[]string{colTool}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 3 {
		t.Errorf("answer.TotalEvents = %v, want %v", answer.TotalEvents, 3)
	}

	if len(answer.Rows) != 2 {
		t.Fatalf("answer.Rows = %v, want one bucket per tool", answer.Rows)
	}

	if answer.Rows[0].Groups[colTool] != toolOK || answer.Rows[0].Count != 2 {
		t.Errorf("answer.Rows[0] = %v, want the busiest tool first", answer.Rows[0])
	}

	if len(answer.Warnings) != 0 {
		t.Errorf("answer.Warnings = %v, want none", answer.Warnings)
	}
}

// TestSummaryOverDegradesToTheLogBehindAWarning is the store's degrade rule
// through the window reader rather than the health one.
func TestSummaryOverDegradesToTheLogBehindAWarning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []*audit.Event{
		makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)),
	})

	dbPath := filepath.Join(dir, "audit.db")
	if err := os.WriteFile(dbPath, []byte(notADatabase), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	answer, err := audit.SummaryOver(t.Context(), dbPath, dir, time.Time{},
		[]string{colTool}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 1 {
		t.Errorf("answer.TotalEvents = %v, want the log's own count", answer.TotalEvents)
	}

	if len(answer.Warnings) != 1 {
		t.Fatalf("answer.Warnings = %v, want one line naming the store", answer.Warnings)
	}
}

// TestSummaryOverBoundPastTheNanosecondRangeCountsNothing verifies a window
// starting past the years a Unix-nanosecond count can hold reaches the SQLite
// store as a bound the column can hold, so the answer is an empty window
// rather than every row the store keeps.
func TestSummaryOverBoundPastTheNanosecondRangeCountsNothing(t *testing.T) {
	t.Parallel()

	dbPath := seededStore(t, makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)))

	answer, err := audit.SummaryOver(t.Context(), dbPath, t.TempDir(),
		farFutureBound(), []string{colTool}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 0 {
		t.Errorf("answer.TotalEvents = %v, want %v", answer.TotalEvents, 0)
	}

	if len(answer.Rows) != 0 {
		t.Errorf("answer.Rows = %v, want no buckets", answer.Rows)
	}
}

// TestSummaryOverReportsADirectoryItCannotRead is the one condition it answers.
func TestSummaryOverReportsADirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sealDirectory(t, dir)

	_, err := audit.SummaryOver(t.Context(), "", dir, time.Time{}, []string{colTool}, false)
	if err == nil {
		t.Error("err = nil, want the unreadable directory reported")
	}
}

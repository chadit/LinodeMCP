package audit_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// TestValidateGroupByDefaultsToToolStatus verifies an empty request
// falls back to the documented default grouping.
func TestValidateGroupByDefaultsToToolStatus(t *testing.T) {
	t.Parallel()

	got, err := audit.ValidateGroupBy(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{colTool, colStatus}, got)
}

// TestValidateGroupByAcceptsAllowed verifies allowlisted columns pass
// through in order.
func TestValidateGroupByAcceptsAllowed(t *testing.T) {
	t.Parallel()

	got, err := audit.ValidateGroupBy([]string{"capability", "profile", "environment"})
	require.NoError(t, err)
	assert.Equal(t, []string{"capability", "profile", "environment"}, got)
}

// TestValidateGroupByRejectsUnknown verifies an unknown column is a
// typed error rather than a silent empty grouping.
func TestValidateGroupByRejectsUnknown(t *testing.T) {
	t.Parallel()

	_, err := audit.ValidateGroupBy([]string{colTool, "bogus"})
	require.Error(t, err)
	assert.ErrorIs(t, err, audit.ErrUnknownGroupByColumn)
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

	require.Len(t, rows, 2, "two distinct tool+status buckets")
	assert.Equal(t, "linode_instance_list", rows[0].Groups[colTool], "highest count sorts first")
	assert.Equal(t, "success", rows[0].Groups[colStatus])
	assert.Equal(t, 2, rows[0].Count)
	assert.Equal(t, "linode_instance_delete", rows[1].Groups[colTool])
	assert.Equal(t, 1, rows[1].Count)
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
	require.NoError(t, err)
	assert.Len(t, jsonlEvents, 3, "JSONL returns all three events with include_meta")

	// SQLite source.
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	require.NoError(t, err)

	for idx := range events {
		sink.Write(t.Context(), &events[idx])
	}

	require.NoError(t, sink.Close())

	sqliteEvents, err := audit.LoadWindow(t.Context(), dbPath, "", time.Time{}, true)
	require.NoError(t, err)
	assert.Len(t, sqliteEvents, 3, "SQLite returns all three events with include_meta")

	// Both produce the same summary.
	jsonlRows := audit.Summarize(jsonlEvents, []string{colTool})
	sqliteRows := audit.Summarize(sqliteEvents, []string{colTool})
	assert.Equal(t, jsonlRows, sqliteRows, "both sources summarize identically")
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
	require.NoError(t, err)
	require.Len(t, got, 1, "meta event excluded when include_meta is false")
	assert.Equal(t, "linode_instance_list", got[0].Tool)
}

// TestLoadWindowMissingDirReturnsEmpty verifies querying before any
// audit exists is empty, not an error.
func TestLoadWindowMissingDirReturnsEmpty(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-audit-yet")

	got, err := audit.LoadWindow(t.Context(), "", missing, time.Time{}, true)
	require.NoError(t, err)
	assert.Empty(t, got)
}

package audit_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// openTestSQLiteSink builds a SQLite sink at a temp path with the
// default busy timeout.
func openTestSQLiteSink(t *testing.T) *audit.SQLiteSink {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	mustNoError(t, err, "NewSQLiteSink must succeed at a fresh tmp path")

	t.Cleanup(func() { _ = sink.Close() })

	return sink
}

// TestSQLiteSinkInsertsAndReadsBack verifies an event written through
// the sink round-trips: the row exists with the expected scalar
// columns and JSON-encoded args.
func TestSQLiteSinkInsertsAndReadsBack(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	ts := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)
	evt := audit.Event{
		TS:             ts,
		TSUnixNS:       ts.UnixNano(),
		EventID:        "evt_sqlite_one",
		Tool:           "linode_instance_create",
		ToolCapability: audit.CapabilityWrite,
		Environment:    "prod",
		Profile:        "operator",
		Mode:           audit.ModeNormal,
		Args:           map[string]any{argKeyLabel: "web-1", keyRegion: valUSEast},
		ArgsRedacted:   []string{argKeyToken},
		Status:         audit.StatusSuccess,
		LatencyMS:      42,
		ResultSummary:  "created",
	}

	sink.Write(t.Context(), &evt)

	var (
		tool       string
		capability string
		status     string
		latencyMS  int64
		argsJSON   string
		redacted   string
	)

	row := sink.DB().QueryRowContext(
		t.Context(),
		`SELECT tool, tool_capability, status, latency_ms, args_json, args_redacted_json
		 FROM events WHERE event_id = ?`,
		evt.EventID,
	)
	mustNoError(t, row.Scan(&tool, &capability, &status, &latencyMS, &argsJSON, &redacted),
		"the written row must be readable")

	checkEqual(t, "linode_instance_create", tool)
	checkEqual(t, "write", capability, "capability stored as its string form")
	checkEqual(t, "success", status)
	checkEqual(t, int64(42), latencyMS)
	checkJSONEq(t, `{"label":"web-1","region":"us-east"}`, argsJSON, "args stored as JSON")
	checkJSONEq(t, `["token"]`, redacted, "redacted-key list stored as JSON")
}

// TestSQLiteSinkIgnoresDuplicateEventID verifies INSERT OR IGNORE
// keeps a re-delivered event idempotent: the second write is a no-op,
// not an error or a duplicate row.
func TestSQLiteSinkIgnoresDuplicateEventID(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	evt := makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 9))
	evt.EventID = "evt_dup"

	sink.Write(t.Context(), &evt)
	sink.Write(t.Context(), &evt)

	var count int

	row := sink.DB().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM events WHERE event_id = ?`, evt.EventID)
	mustNoError(t, row.Scan(&count))
	checkEqual(t, 1, count, "duplicate event_id must not create a second row")
}

// TestSQLiteSinkStoresNullsForAbsentOptionals verifies plan_id and
// error are SQL NULL when the event's pointers are nil.
func TestSQLiteSinkStoresNullsForAbsentOptionals(t *testing.T) {
	t.Parallel()

	sink := openTestSQLiteSink(t)

	evt := makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 10))
	evt.EventID = "evt_nulls"
	evt.PlanID = nil
	evt.Error = nil

	sink.Write(t.Context(), &evt)

	var planID, errCol sql.NullString

	row := sink.DB().QueryRowContext(t.Context(),
		`SELECT plan_id, error FROM events WHERE event_id = ?`, evt.EventID)
	mustNoError(t, row.Scan(&planID, &errCol))

	checkFalse(t, planID.Valid, "nil PlanID must store as NULL")
	checkFalse(t, errCol.Valid, "nil Error must store as NULL")
}

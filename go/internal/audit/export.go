package audit

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// Export format names accepted by EncodeEvents.
const (
	ExportFormatJSON   = "json"
	ExportFormatCSV    = "csv"
	ExportFormatNDJSON = "ndjson"
)

// DefaultExportMaxRecords bounds an export when the caller does not ask
// for a specific cap. MaxExportRecords is the hard ceiling so a single
// call cannot pull an unbounded slice into memory.
const (
	DefaultExportMaxRecords = 10000
	MaxExportRecords        = 100000
)

// exportColumns names the full SQLite column list an export reads, in
// the order exportFromSQLite scans them.
const exportColumns = `event_id, ts_unix_ns, tool, tool_capability, environment, profile,
	mode, plan_id, status, latency_ms, result_summary, error,
	linodemcp_version, session_id, credential_generation,
	args_json, args_redacted_json`

// ExportEvents loads up to query.Limit matching events for export,
// newest first. It reads the SQLite store when sqlitePath is non-empty
// (full-row reconstruction including args), otherwise it scans the
// JSONL directory. Unlike LoadWindow, this returns complete events so
// the export carries the full record, not just the summary columns.
func ExportEvents(ctx context.Context, sqlitePath, jsonlDir string, query *RecentQuery) ([]*Event, error) {
	if sqlitePath != "" {
		return exportFromSQLite(ctx, sqlitePath, query)
	}

	return scanMatching(jsonlDir, query, query.Limit)
}

// exportFromSQLite reads full event rows from the SQLite store. The
// SELECT lists fixed columns with a parameterized lower-bound on
// ts_unix_ns; the remaining filters (until, tool glob, capability,
// status, meta) are applied in Go via query.matches so the statement
// stays static. Rows come newest-first and the scan stops at the
// query's limit.
func exportFromSQLite(ctx context.Context, path string, query *RecentQuery) ([]*Event, error) {
	db, err := sql.Open(sqliteDriverName, "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("audit: open sqlite %s: %w", path, err)
	}

	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(
		ctx,
		"SELECT "+exportColumns+" FROM events WHERE ts_unix_ns >= ? ORDER BY ts_unix_ns DESC",
		sinceUnixNano(query.Since),
	)
	if err != nil {
		return nil, fmt.Errorf("audit: sqlite export query: %w", err)
	}

	defer func() { _ = rows.Close() }()

	events := make([]*Event, 0, query.Limit)

	for rows.Next() {
		event, scanErr := scanExportRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		if !query.matches(event) {
			continue
		}

		events = append(events, event)
		if len(events) >= query.Limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: sqlite export rows: %w", err)
	}

	return events, nil
}

// scanExportRow reconstructs one full Event from an export row,
// rebuilding the timestamp text from ts_unix_ns and decoding the
// args/args_redacted JSON columns. Nullable plan_id, result_summary,
// and error are handled via sql.NullString.
//
// The store keeps only the nanosecond count, so the row's timestamp text is
// written back in the record's own spelling rather than in a second one.
func scanExportRow(rows *sql.Rows) (*Event, error) {
	var (
		event                            Event
		tsUnixNS                         int64
		planID, resultSummary, errorText sql.NullString
		argsJSON, redactedJSON           string
	)

	if err := rows.Scan(
		&event.EventId, &tsUnixNS, &event.Tool, &event.ToolCapability,
		&event.Environment, &event.Profile, &event.Mode, &planID,
		&event.Status, &event.LatencyMs, &resultSummary, &errorText,
		&event.LinodemcpVersion, &event.SessionId, &event.CredentialGeneration,
		&argsJSON, &redactedJSON,
	); err != nil {
		return nil, fmt.Errorf("audit: sqlite export scan: %w", err)
	}

	event.TsUnixNs = tsUnixNS
	event.Ts = EventTimestamp(time.Unix(0, tsUnixNS))
	event.ResultSummary = resultSummary.String

	if planID.Valid {
		event.PlanId = &planID.String
	}

	if errorText.Valid {
		event.Error = &errorText.String
	}

	if err := decodeStoredJSON(argsJSON, &event.Args); err != nil {
		return nil, fmt.Errorf("audit: sqlite export decode args %s: %w", event.EventId, err)
	}

	if err := decodeStoredJSON(redactedJSON, &event.ArgsRedacted); err != nil {
		return nil, fmt.Errorf("audit: sqlite export decode args_redacted %s: %w", event.EventId, err)
	}

	return &event, nil
}

// decodeStoredJSON reads one of the store's own JSON columns with the number
// handling the record reader uses.
//
// Without it a whole number the log wrote comes back off the store as a double
// and is re-exported in a second spelling, so the same event would leave the
// two stores differently.
func decodeStoredJSON(text string, into any) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()

	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("decode stored json: %w", err)
	}

	return nil
}

// exportIndent is one level of indentation in the JSON document, which is the
// two spaces that format has always carried.
const exportIndent = "  "

// exportRecordBudget is roughly the bytes one written record runs to, which is
// all a document builder needs to size itself once.
const exportRecordBudget = 1024

// EncodeEvents writes events to w in the named format. JSON produces a
// single indented array; NDJSON one compact record per line; CSV a
// header row plus one row per event (the args map and args_redacted
// list are encoded as JSON text in their cells). An unknown format is
// ErrUnknownExportFormat.
//
// Every record goes through the contract's own writer, so an export written
// here and one written by another language carry the same bytes rather than
// the same values in two spellings.
func EncodeEvents(out io.Writer, events []*Event, format string) error {
	switch format {
	case ExportFormatJSON:
		return encodeJSONDocument(out, events)
	case ExportFormatNDJSON:
		return encodeNDJSON(out, events)
	case ExportFormatCSV:
		return encodeCSV(out, events)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownExportFormat, format)
	}
}

// encodeJSONDocument writes the records as one indented array closed by a
// newline, which is the document shape this format has always had.
func encodeJSONDocument(out io.Writer, events []*Event) error {
	document := make([]byte, 0, len(events)*exportRecordBudget)
	document = append(document, '[')

	for index, event := range events {
		if index > 0 {
			document = append(document, ',')
		}

		record, err := genlocal.RecordAuditEvent(event, exportIndent, exportIndent)
		if err != nil {
			return fmt.Errorf("audit: encode json export: %w", err)
		}

		document = append(document, '\n')
		document = append(document, exportIndent...)
		document = append(document, record...)
	}

	if len(events) > 0 {
		document = append(document, '\n')
	}

	document = append(document, ']', '\n')

	if _, err := out.Write(document); err != nil {
		return fmt.Errorf("audit: write json export: %w", err)
	}

	return nil
}

// encodeNDJSON writes one compact record per line. An empty slice
// yields empty output (zero lines), which is a valid NDJSON document.
func encodeNDJSON(out io.Writer, events []*Event) error {
	for _, event := range events {
		record, err := genlocal.RecordAuditEvent(event, "", "")
		if err != nil {
			return fmt.Errorf("audit: encode ndjson export: %w", err)
		}

		if _, err := out.Write(append(record, '\n')); err != nil {
			return fmt.Errorf("audit: write ndjson export: %w", err)
		}
	}

	return nil
}

// exportCSVHeader returns the CSV column order, mirrored by
// exportCSVRow. A function (not a package var) keeps the column
// contract free of global state.
func exportCSVHeader() []string {
	return []string{
		"ts", "event_id", columnTool, "tool_capability", columnStatus, "environment",
		"profile", "mode", "latency_ms", "result_summary", "error", "plan_id",
		"session_id", "credential_generation", "args_redacted", "args",
	}
}

// encodeCSV writes a header row then one row per event. The nested
// args map and args_redacted list are JSON-encoded into single cells.
//
// The writer ends a row with a bare newline, which is the terminator this
// format carries in every language rather than the one a spreadsheet dialect
// would pick.
func encodeCSV(out io.Writer, events []*Event) error {
	writer := csv.NewWriter(out)

	if err := writer.Write(exportCSVHeader()); err != nil {
		return fmt.Errorf("audit: write csv header: %w", err)
	}

	for _, event := range events {
		row, err := exportCSVRow(event)
		if err != nil {
			return err
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("audit: write csv row: %w", err)
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return fmt.Errorf("audit: flush csv export: %w", err)
	}

	return nil
}

// exportCSVRow flattens an event into CSV cells in exportCSVHeader
// order. Nullable plan_id/error render as empty cells; args and
// args_redacted are compact JSON.
func exportCSVRow(event *Event) ([]string, error) {
	// The record's own value spelling rather than this language's default, so
	// a cell holding a map comes out the same in every language. Both cells go
	// through one report because only the arguments can carry a value no
	// encoder writes, and a second branch for the redaction list would be one
	// nothing reaches.
	cells := make([]string, 0, len(exportJSONCells(event)))

	for _, value := range exportJSONCells(event) {
		encoded, err := genlocal.RecordValue(value, "", "")
		if err != nil {
			return nil, fmt.Errorf("audit: encode csv cell %s: %w", event.EventId, err)
		}

		cells = append(cells, string(encoded))
	}

	return []string{
		event.Ts,
		event.EventId,
		event.Tool,
		event.ToolCapability,
		event.Status,
		event.Environment,
		event.Profile,
		event.Mode,
		strconv.FormatInt(event.LatencyMs, 10),
		event.ResultSummary,
		derefString(event.Error),
		derefString(event.PlanId),
		event.SessionId,
		strconv.FormatUint(event.CredentialGeneration, 10),
		cells[0],
		cells[1],
	}, nil
}

// exportJSONCells is the two members a CSV row carries as JSON text, in the
// order exportCSVHeader ends on.
func exportJSONCells(event *Event) []any {
	return []any{event.ArgsRedacted, event.Args}
}

// derefString returns the pointed-to string, or empty for a nil
// pointer (the nullable plan_id and error columns).
func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

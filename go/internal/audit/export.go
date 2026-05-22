package audit

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
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
func ExportEvents(ctx context.Context, sqlitePath, jsonlDir string, query *RecentQuery) ([]Event, error) {
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
func exportFromSQLite(ctx context.Context, path string, query *RecentQuery) ([]Event, error) {
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

	events := make([]Event, 0, query.Limit)

	for rows.Next() {
		event, scanErr := scanExportRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		if !query.matches(&event) {
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
// rebuilding TS from ts_unix_ns and decoding the args/args_redacted
// JSON columns. Nullable plan_id, result_summary, and error are
// handled via sql.NullString.
func scanExportRow(rows *sql.Rows) (Event, error) {
	var (
		event                            Event
		tsUnixNS                         int64
		planID, resultSummary, errorText sql.NullString
		argsJSON, redactedJSON           string
	)

	if err := rows.Scan(
		&event.EventID, &tsUnixNS, &event.Tool, &event.ToolCapability,
		&event.Environment, &event.Profile, &event.Mode, &planID,
		&event.Status, &event.LatencyMS, &resultSummary, &errorText,
		&event.LinodemcpVersion, &event.SessionID, &event.CredentialGeneration,
		&argsJSON, &redactedJSON,
	); err != nil {
		return Event{}, fmt.Errorf("audit: sqlite export scan: %w", err)
	}

	event.TSUnixNS = tsUnixNS
	event.TS = time.Unix(0, tsUnixNS).UTC()
	event.ResultSummary = resultSummary.String

	if planID.Valid {
		event.PlanID = &planID.String
	}

	if errorText.Valid {
		event.Error = &errorText.String
	}

	if err := json.Unmarshal([]byte(argsJSON), &event.Args); err != nil {
		return Event{}, fmt.Errorf("audit: sqlite export decode args %s: %w", event.EventID, err)
	}

	if err := json.Unmarshal([]byte(redactedJSON), &event.ArgsRedacted); err != nil {
		return Event{}, fmt.Errorf("audit: sqlite export decode args_redacted %s: %w", event.EventID, err)
	}

	return event, nil
}

// EncodeEvents writes events to w in the named format. JSON produces a
// single indented array; NDJSON one compact object per line; CSV a
// header row plus one row per event (the args map and args_redacted
// list are encoded as JSON text in their cells). An unknown format is
// ErrUnknownExportFormat.
func EncodeEvents(out io.Writer, events []Event, format string) error {
	switch format {
	case ExportFormatJSON:
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(events); err != nil {
			return fmt.Errorf("audit: encode json export: %w", err)
		}

		return nil
	case ExportFormatNDJSON:
		return encodeNDJSON(out, events)
	case ExportFormatCSV:
		return encodeCSV(out, events)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownExportFormat, format)
	}
}

// encodeNDJSON writes one compact JSON object per line. An empty slice
// yields empty output (zero lines), which is a valid NDJSON document.
func encodeNDJSON(out io.Writer, events []Event) error {
	encoder := json.NewEncoder(out)
	for idx := range events {
		if err := encoder.Encode(&events[idx]); err != nil {
			return fmt.Errorf("audit: encode ndjson export: %w", err)
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
func encodeCSV(out io.Writer, events []Event) error {
	writer := csv.NewWriter(out)

	if err := writer.Write(exportCSVHeader()); err != nil {
		return fmt.Errorf("audit: write csv header: %w", err)
	}

	for idx := range events {
		row, err := exportCSVRow(&events[idx])
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
	argsCell, err := json.Marshal(event.Args)
	if err != nil {
		return nil, fmt.Errorf("audit: encode csv args %s: %w", event.EventID, err)
	}

	redactedCell, err := json.Marshal(event.ArgsRedacted)
	if err != nil {
		return nil, fmt.Errorf("audit: encode csv args_redacted %s: %w", event.EventID, err)
	}

	return []string{
		event.TS.Format(time.RFC3339Nano),
		event.EventID,
		event.Tool,
		string(event.ToolCapability),
		string(event.Status),
		event.Environment,
		event.Profile,
		string(event.Mode),
		strconv.FormatInt(event.LatencyMS, 10),
		event.ResultSummary,
		derefString(event.Error),
		derefString(event.PlanID),
		event.SessionID,
		strconv.FormatUint(event.CredentialGeneration, 10),
		string(redactedCell),
		string(argsCell),
	}, nil
}

// derefString returns the pointed-to string, or empty for a nil
// pointer (the nullable plan_id and error columns).
func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

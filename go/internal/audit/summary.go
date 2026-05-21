package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SummaryQuery filters and groups a summary aggregation. Since bounds
// the lower edge of the window (zero = no bound). GroupBy names the
// columns to count by; empty defaults to {tool, status}. Meta events
// are excluded unless IncludeMeta is set.
type SummaryQuery struct {
	Since       time.Time
	GroupBy     []string
	IncludeMeta bool
}

// SummaryRow is one aggregated bucket: the grouped column values and
// the count of events in that bucket.
type SummaryRow struct {
	Groups map[string]string `json:"groups"`
	Count  int               `json:"count"`
}

// summaryColumnAccessor returns the field extractor for a groupable
// column name, plus whether the name is allowed. A switch (rather than
// a package-level map) keeps the allowlist free of global state.
func summaryColumnAccessor(name string) (func(*Event) string, bool) {
	switch name {
	case "tool":
		return func(e *Event) string { return e.Tool }, true
	case "status":
		return func(e *Event) string { return string(e.Status) }, true
	case "capability":
		return func(e *Event) string { return string(e.ToolCapability) }, true
	case "profile":
		return func(e *Event) string { return e.Profile }, true
	case "environment":
		return func(e *Event) string { return e.Environment }, true
	default:
		return nil, false
	}
}

// ValidateGroupBy checks every requested column against the allowlist
// and returns the effective list. An empty request defaults to
// {tool, status}. An unknown column is an error so a typo surfaces
// rather than silently producing an empty grouping.
func ValidateGroupBy(groupBy []string) ([]string, error) {
	if len(groupBy) == 0 {
		return []string{"tool", "status"}, nil
	}

	for _, name := range groupBy {
		if _, ok := summaryColumnAccessor(name); !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownGroupByColumn, name)
		}
	}

	return groupBy, nil
}

// Summarize aggregates events into per-bucket counts grouped by the
// given columns. groupBy must already be validated. Rows are sorted by
// count descending, then by their grouped values, for deterministic
// output.
func Summarize(events []Event, groupBy []string) []SummaryRow {
	counts := make(map[string]int, len(events))
	groupsByKey := make(map[string]map[string]string, len(events))

	for idx := range events {
		values := make([]string, len(groupBy))
		groups := make(map[string]string, len(groupBy))

		for col, name := range groupBy {
			accessor, _ := summaryColumnAccessor(name)
			value := accessor(&events[idx])
			values[col] = value
			groups[name] = value
		}

		key := strings.Join(values, "\x00")
		counts[key]++

		if _, seen := groupsByKey[key]; !seen {
			groupsByKey[key] = groups
		}
	}

	rows := make([]SummaryRow, 0, len(counts))
	for key, count := range counts {
		rows = append(rows, SummaryRow{Groups: groupsByKey[key], Count: count})
	}

	sortSummaryRows(rows, groupBy)

	return rows
}

// sortSummaryRows orders rows by count descending, breaking ties by
// the grouped column values in groupBy order so output is stable.
func sortSummaryRows(rows []SummaryRow, groupBy []string) {
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].Count != rows[right].Count {
			return rows[left].Count > rows[right].Count
		}

		for _, name := range groupBy {
			if rows[left].Groups[name] != rows[right].Groups[name] {
				return rows[left].Groups[name] < rows[right].Groups[name]
			}
		}

		return false
	})
}

// LoadWindow returns events at or after since (zero = all), honoring
// includeMeta. When sqlitePath is non-empty it reads from the SQLite
// database (a fresh read-only-style connection opened and closed per
// call); otherwise it scans the JSONL directory. Shared by the summary
// and export query tools.
func LoadWindow(ctx context.Context, sqlitePath, jsonlDir string, since time.Time, includeMeta bool) ([]Event, error) {
	if sqlitePath != "" {
		return loadWindowSQLite(ctx, sqlitePath, since, includeMeta)
	}

	return loadWindowJSONL(jsonlDir, since, includeMeta)
}

// loadWindowSQLite reads windowed events from the SQLite store. The
// SELECT lists fixed columns with a parameterized lower-bound on
// ts_unix_ns, so there is no dynamic SQL. Meta filtering happens in Go
// to keep the statement static.
func loadWindowSQLite(ctx context.Context, path string, since time.Time, includeMeta bool) ([]Event, error) {
	db, err := sql.Open(sqliteDriverName, "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("audit: open sqlite %s: %w", path, err)
	}

	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(
		ctx,
		`SELECT tool, tool_capability, status, profile, environment, ts_unix_ns
		 FROM events WHERE ts_unix_ns >= ? ORDER BY ts_unix_ns DESC`,
		sinceUnixNano(since),
	)
	if err != nil {
		return nil, fmt.Errorf("audit: sqlite window query: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var events []Event

	for rows.Next() {
		var (
			event    Event
			tsUnixNS int64
		)

		if err := rows.Scan(
			&event.Tool, &event.ToolCapability, &event.Status,
			&event.Profile, &event.Environment, &tsUnixNS,
		); err != nil {
			return nil, fmt.Errorf("audit: sqlite window scan: %w", err)
		}

		event.TSUnixNS = tsUnixNS
		if !includeMeta && event.ToolCapability == CapabilityMeta {
			continue
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: sqlite window rows: %w", err)
	}

	return events, nil
}

// loadWindowJSONL scans the audit directory for events at or after
// since, honoring includeMeta. Reuses the reader's file ordering and
// per-file decode; unlike ReadRecent it applies no count limit because
// aggregation needs the whole window.
func loadWindowJSONL(dir string, since time.Time, includeMeta bool) ([]Event, error) {
	root, err := openReadRoot(dir)
	if err != nil {
		if errors.Is(err, errAuditDirMissing) {
			return []Event{}, nil
		}

		return nil, err
	}

	defer func() { _ = root.Close() }()

	files, err := orderedAuditFiles(root)
	if err != nil {
		return nil, fmt.Errorf("audit: list audit dir %s: %w", dir, err)
	}

	var events []Event

	for _, name := range files {
		fileEvents := readEventsFromFile(root, name)
		for idx := range fileEvents {
			event := &fileEvents[idx]
			if !includeMeta && event.ToolCapability == CapabilityMeta {
				continue
			}

			if !since.IsZero() && event.TS.Before(since) {
				continue
			}

			events = append(events, *event)
		}
	}

	return events, nil
}

// sinceUnixNano converts a since bound to a Unix-nanosecond cutoff,
// returning 0 (match everything) for the zero time.
func sinceUnixNano(since time.Time) int64 {
	if since.IsZero() {
		return 0
	}

	return since.UnixNano()
}

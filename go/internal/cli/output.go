package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// Output formats the `call` command supports. JSON prints the tool's
// result payload verbatim (the scripting contract); table renders a
// list-of-objects or a single object as aligned columns for humans.
const (
	outputJSON  = "json"
	outputTable = "table"
)

// tabwriter layout constants for the table renderer. Extracted so the
// numbers aren't bare literals at the call site.
const (
	tableMinWidth = 0
	tableTabWidth = 2
	tablePadding  = 2
	tablePadChar  = ' '
)

// renderOutput writes a tool's text payload to stdout in the requested
// format. JSON passes the payload straight through (it already is the
// tool's JSON, or a plain message for the meta tools). Table attempts a
// structured render and falls back to the raw payload when the JSON
// isn't a shape the table renderer understands.
func renderOutput(stdout io.Writer, payload, format string) {
	if format == outputTable {
		if rendered, ok := renderTable(payload); ok {
			writef(stdout, "%s", rendered)

			return
		}
	}

	writeln(stdout, payload)
}

// renderTable tries to turn a JSON payload into an aligned text table.
// It handles two shapes: a JSON array of objects (one row per element,
// columns unioned across rows) and a single JSON object (a two-column
// key/value table). Anything else returns ok=false so the caller prints
// the raw payload instead of guessing.
func renderTable(payload string) (string, bool) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return "", false
	}

	switch trimmed[0] {
	case '[':
		return renderArrayTable(trimmed)
	case '{':
		return renderObjectTable(trimmed)
	default:
		return "", false
	}
}

// renderArrayTable renders a JSON array of objects as a row table. Rows
// that aren't objects sink the render (ok=false) so a list of scalars
// falls back to raw JSON rather than producing a confusing one-column
// dump. Columns are the union of all row keys, sorted for stable output.
func renderArrayTable(payload string) (string, bool) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(payload), &rows); err != nil {
		return "", false
	}

	if len(rows) == 0 {
		return "", false
	}

	columns := unionKeys(rows)

	var buf strings.Builder

	writer := newTableWriter(&buf)
	writef(writer, "%s\n", strings.Join(columns, "\t"))

	for _, row := range rows {
		cells := make([]string, len(columns))
		for i, col := range columns {
			cells[i] = cellString(row[col])
		}

		writef(writer, "%s\n", strings.Join(cells, "\t"))
	}

	if err := writer.Flush(); err != nil {
		return "", false
	}

	return buf.String(), true
}

// renderObjectTable renders a single JSON object as a two-column
// key/value table with keys sorted for stable output.
func renderObjectTable(payload string) (string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(payload), &obj); err != nil {
		return "", false
	}

	if len(obj) == 0 {
		return "", false
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var buf strings.Builder

	writer := newTableWriter(&buf)
	writef(writer, "FIELD\tVALUE\n")

	for _, key := range keys {
		writef(writer, "%s\t%s\n", key, cellString(obj[key]))
	}

	if err := writer.Flush(); err != nil {
		return "", false
	}

	return buf.String(), true
}

// unionKeys collects the distinct keys across all rows, sorted, so a
// table covers every field even when rows are ragged.
func unionKeys(rows []map[string]any) []string {
	seen := make(map[string]struct{}, len(rows))

	for _, row := range rows {
		for key := range row {
			seen[key] = struct{}{}
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// cellString renders one table cell value. Scalars print naturally;
// nested objects and arrays print as compact JSON so a cell never spans
// lines and breaks column alignment.
func cellString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return formatNumber(typed)
	default:
		compact, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}

		return string(compact)
	}
}

// formatNumber prints a JSON number without a trailing ".0" for whole
// values, so an integer field reads as "123" rather than "123.000000".
func formatNumber(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}

	return strconv.FormatFloat(value, 'g', -1, 64)
}

// newTableWriter builds a tabwriter with the shared column layout.
func newTableWriter(buf *strings.Builder) *tabwriter.Writer {
	return tabwriter.NewWriter(buf, tableMinWidth, tableTabWidth, tablePadding, tablePadChar, 0)
}

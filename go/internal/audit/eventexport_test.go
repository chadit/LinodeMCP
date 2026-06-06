package audit_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// TestExportEventsJSONL confirms the JSONL-backed export honors the
// tool-glob filter and the max-records cap, returning full events
// newest-first.
func TestExportEventsJSONL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	keep := makeTestEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 8))
	keep.Args = map[string]any{keyRegion: valUSEast}
	drop := makeTestEvent("linode_volume_list", audit.CapabilityRead, audit.StatusSuccess, day(20, 9))

	writeJSONLFile(t, filepath.Join(dir, "audit.log"), false, []audit.Event{keep, drop})

	query := &audit.RecentQuery{Limit: audit.DefaultExportMaxRecords, Tool: "linode_instance_*"}

	events, err := audit.ExportEvents(t.Context(), "", dir, query)
	mustNoError(t, err)

	mustLen(t, events, 1, "glob excludes the volume event")
	checkEqual(t, "linode_instance_list", events[0].Tool)
	checkEqual(t, valUSEast, events[0].Args[keyRegion], "JSONL export carries the full args")
}

// TestExportEventsSQLiteFullRecord confirms the SQLite-backed export
// reconstructs the complete event, including the args map and a
// nullable error, not just the summary columns.
func TestExportEventsSQLiteFullRecord(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	mustNoError(t, err)

	evt := makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusSuccess, day(20, 8))
	evt.Args = map[string]any{argLinodeID: float64(123), "confirm": true}
	evt.ArgsRedacted = []string{argKeyToken}
	errText := "boom"
	evt.Error = &errText

	sink.Write(t.Context(), &evt)
	mustNoError(t, sink.Close())

	query := &audit.RecentQuery{Limit: audit.DefaultExportMaxRecords, IncludeMeta: true}

	events, err := audit.ExportEvents(t.Context(), dbPath, t.TempDir(), query)
	mustNoError(t, err)

	mustLen(t, events, 1)
	got := events[0]
	checkEqual(t, "linode_instance_delete", got.Tool)
	checkInDelta(t, float64(123), got.Args[argLinodeID], 0)
	checkEqual(t, true, got.Args["confirm"])
	checkEqual(t, []string{argKeyToken}, got.ArgsRedacted)
	mustNotNil(t, got.Error)
	checkEqual(t, "boom", *got.Error)
}

// TestEncodeEventsJSON checks the JSON format round-trips to the same
// events.
func TestEncodeEventsJSON(t *testing.T) {
	t.Parallel()

	events := []audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
	}

	var buf bytes.Buffer

	mustNoError(t, audit.EncodeEvents(&buf, events, audit.ExportFormatJSON))

	var decoded []audit.Event

	mustNoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	mustLen(t, decoded, 1)
	checkEqual(t, "tool_a", decoded[0].Tool)
}

// TestEncodeEventsNDJSON checks one JSON object per line, one line per
// event.
func TestEncodeEventsNDJSON(t *testing.T) {
	t.Parallel()

	events := []audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
		makeTestEvent("tool_b", audit.CapabilityRead, audit.StatusError, day(20, 9)),
	}

	var buf bytes.Buffer

	mustNoError(t, audit.EncodeEvents(&buf, events, audit.ExportFormatNDJSON))

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	mustLen(t, lines, 2, "one line per event")

	var first audit.Event

	mustNoError(t, json.Unmarshal([]byte(lines[0]), &first))
	checkEqual(t, "tool_a", first.Tool)
}

// TestEncodeEventsCSV checks the header plus a data row, and that the
// args map lands in its cell as JSON.
func TestEncodeEventsCSV(t *testing.T) {
	t.Parallel()

	evt := makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(20, 8))
	evt.Args = map[string]any{keyRegion: valUSEast}

	var buf bytes.Buffer

	mustNoError(t, audit.EncodeEvents(&buf, []audit.Event{evt}, audit.ExportFormatCSV))

	records, err := csv.NewReader(&buf).ReadAll()
	mustNoError(t, err)
	mustLen(t, records, 2, "header plus one data row")
	checkEqual(t, colTool, records[0][2], "header column order")
	checkEqual(t, "tool_a", records[1][2])
	checkContains(t, records[1][len(records[1])-1], valUSEast, "args cell is JSON")
}

// TestEncodeEventsUnknownFormat surfaces a bad format as the sentinel.
func TestEncodeEventsUnknownFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := audit.EncodeEvents(&buf, nil, "xml")
	mustError(t, err)
	checkErrorIs(t, err, audit.ErrUnknownExportFormat)
}

// TestEncodeEventsJSONEmptyIsArray confirms an empty export renders as
// an empty JSON array, not null.
func TestEncodeEventsJSONEmptyIsArray(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	mustNoError(t, audit.EncodeEvents(&buf, []audit.Event{}, audit.ExportFormatJSON))
	checkEqual(t, "[]", strings.TrimSpace(buf.String()))
}

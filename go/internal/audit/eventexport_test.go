package audit_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"reflect"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want %d", len(events), 1)
	}

	if events[0].Tool != tcLinodeInstanceList {
		t.Errorf("events[0].Tool = %v, want %v", events[0].Tool, tcLinodeInstanceList)
	}

	if !reflect.DeepEqual(events[0].Args[keyRegion], valUSEast) {
		t.Errorf("events[0].Args[keyRegion] = %v, want %v", events[0].Args[keyRegion], valUSEast)
	}
}

// TestExportEventsSQLiteFullRecord confirms the SQLite-backed export
// reconstructs the complete event, including the args map and a
// nullable error, not just the summary columns.
func TestExportEventsSQLiteFullRecord(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evt := makeTestEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusSuccess, day(20, 8))
	evt.Args = map[string]any{argLinodeID: float64(123), "confirm": true}
	evt.ArgsRedacted = []string{argKeyToken}
	errText := "boom"
	evt.Error = &errText

	sink.Write(t.Context(), &evt)

	if err := sink.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	query := &audit.RecentQuery{Limit: audit.DefaultExportMaxRecords, IncludeMeta: true}

	events, err := audit.ExportEvents(t.Context(), dbPath, t.TempDir(), query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want %d", len(events), 1)
	}

	got := events[0]
	if got.Tool != tcLinodeInstanceDelete {
		t.Errorf("got.Tool = %v, want %v", got.Tool, tcLinodeInstanceDelete)
	}

	if numF, numOK := got.Args[argLinodeID].(float64); !numOK || math.Abs(numF-float64(float64(123))) > 0 {
		t.Errorf("got %v, want %v", got.Args[argLinodeID], float64(123))
	}

	if !reflect.DeepEqual(got.Args["confirm"], true) {
		t.Errorf("got %v, want %v", got.Args["confirm"], true)
	}

	if !reflect.DeepEqual(got.ArgsRedacted, []string{argKeyToken}) {
		t.Errorf("got.ArgsRedacted = %v, want %v", got.ArgsRedacted, []string{argKeyToken})
	}

	if got.Error == nil {
		t.Fatal("got.Error is nil")
	}

	if *got.Error != tcBoom {
		t.Errorf("*got.Error = %v, want %v", *got.Error, tcBoom)
	}
}

// TestEncodeEventsJSON checks the JSON format round-trips to the same
// events.
func TestEncodeEventsJSON(t *testing.T) {
	t.Parallel()

	events := []audit.Event{
		makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(20, 8)),
	}

	var buf bytes.Buffer

	if err := audit.EncodeEvents(&buf, events, audit.ExportFormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []audit.Event

	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), 1)
	}

	if decoded[0].Tool != tcToolA {
		t.Errorf("decoded[0].Tool = %v, want %v", decoded[0].Tool, tcToolA)
	}
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

	if err := audit.EncodeEvents(&buf, events, audit.ExportFormatNDJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want %d", len(lines), 2)
	}

	var first audit.Event

	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.Tool != tcToolA {
		t.Errorf("first.Tool = %v, want %v", first.Tool, tcToolA)
	}
}

// TestEncodeEventsCSV checks the header plus a data row, and that the
// args map lands in its cell as JSON.
func TestEncodeEventsCSV(t *testing.T) {
	t.Parallel()

	evt := makeTestEvent("tool_a", audit.CapabilityRead, audit.StatusSuccess, day(20, 8))
	evt.Args = map[string]any{keyRegion: valUSEast}

	var buf bytes.Buffer

	if err := audit.EncodeEvents(&buf, []audit.Event{evt}, audit.ExportFormatCSV); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want %d", len(records), 2)
	}

	if records[0][2] != colTool {
		t.Errorf("records[0][2] = %v, want %v", records[0][2], colTool)
	}

	if records[1][2] != tcToolA {
		t.Errorf("records[1][2] = %v, want %v", records[1][2], tcToolA)
	}

	if !strings.Contains(records[1][len(records[1])-1], valUSEast) {
		t.Errorf("collection does not contain %v", valUSEast)
	}
}

// TestEncodeEventsUnknownFormat surfaces a bad format as the sentinel.
func TestEncodeEventsUnknownFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := audit.EncodeEvents(&buf, nil, "xml")
	if !errors.Is(err, audit.ErrUnknownExportFormat) {
		t.Errorf("error = %v, want %v", err, audit.ErrUnknownExportFormat)
	}
}

// TestEncodeEventsJSONEmptyIsArray confirms an empty export renders as
// an empty JSON array, not null.
func TestEncodeEventsJSONEmptyIsArray(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	if err := audit.EncodeEvents(&buf, []audit.Event{}, audit.ExportFormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("strings.TrimSpace(buf.String()) = %v, want %v", strings.TrimSpace(buf.String()), "[]")
	}
}

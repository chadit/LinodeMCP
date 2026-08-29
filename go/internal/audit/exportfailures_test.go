package audit_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// The failures an export answers rather than writes through.
//
// The exported document is a durable file, so a value the encoder will not
// write and a sink that refuses mid-document both have to be reported instead
// of leaving a half-written export behind. Neither is reachable from a log or a
// store, so each is reached here through a value or a writer the case builds.

// errWriterRefused is what the refusing writer answers, so a case can name the
// failure it planted rather than matching on text.
var errWriterRefused = errors.New("writer refused")

// refusingWriter fails every write, which is the sink an export meets when the
// disk fills part way through a document.
type refusingWriter struct{}

func (refusingWriter) Write([]byte) (int, error) {
	return 0, errWriterRefused
}

// eventWithUnwritableArgument is one event whose arguments carry a value no
// JSON encoder writes. A channel is the smallest such value; the arguments are
// declared free-form, so nothing upstream stops one reaching here.
func eventWithUnwritableArgument(t *testing.T) *audit.Event {
	t.Helper()

	event := makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8))
	event.Args = map[string]any{"unwritable": make(chan int)}

	return event
}

// openTestStore is a database at path, opened through the driver the sink
// registers so a case can seed a row the sink itself would not write.
func openTestStore(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

// seededStore is a store carrying one row the sink wrote, so the schema is the
// shipped one, plus whatever the case then edits into it.
func seededStore(t *testing.T, event *audit.Event) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), path, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink.Write(t.Context(), event)

	if closeErr := sink.Close(); closeErr != nil {
		t.Fatalf("unexpected error: %v", closeErr)
	}

	return path
}

// exportQuery is the unfiltered read every case here makes.
func exportQuery() *audit.RecentQuery {
	return &audit.RecentQuery{Limit: audit.DefaultExportMaxRecords, IncludeMeta: true}
}

// TestExportCarriesThePlanTheStoreHeld covers the nullable plan column on the
// way back out: an event written under plan or apply carries its plan ID, and
// the export is where a caller reads it.
func TestExportCarriesThePlanTheStoreHeld(t *testing.T) {
	t.Parallel()

	planned := audit.SetMode(
		makeTestEvent(toolOK, audit.CapabilityWrite, audit.StatusSuccess, day(19, 8)),
		audit.ModeApply, "plan_01HQXY3ZKQ8M7VRBNP4W5T2J9A",
	)

	events, err := audit.ExportEvents(t.Context(), seededStore(t, planned), "", exportQuery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want %d", len(events), 1)
	}

	if events[0].PlanId == nil {
		t.Fatal("events[0].PlanId is nil")
	}

	if *events[0].PlanId != "plan_01HQXY3ZKQ8M7VRBNP4W5T2J9A" {
		t.Errorf("*events[0].PlanId = %v, want %v",
			*events[0].PlanId, "plan_01HQXY3ZKQ8M7VRBNP4W5T2J9A")
	}
}

// TestExportReportsARowItCannotRead covers the three ways a stored row refuses
// to become a record: a column the scan cannot take, and either JSON column
// holding something that is not JSON at all.
func TestExportReportsARowItCannotRead(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		update string
	}{
		{
			// SQLite keeps whatever a column is handed, so a counter column can
			// hold text the scan has no number to make of it.
			name:   "a column the scan cannot take",
			update: "UPDATE events SET credential_generation = 'not a number'",
		},
		{
			name:   "arguments that are not json",
			update: "UPDATE events SET args_json = 'not json at all'",
		},
		{
			name:   "a redaction list that is not json",
			update: "UPDATE events SET args_redacted_json = 'not json at all'",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := seededStore(t,
				makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8)))

			// The control: before the edit the same store exports its one row,
			// so what the case measures is the edit and not the fixture.
			before, err := audit.ExportEvents(t.Context(), path, "", exportQuery())
			if err != nil || len(before) != 1 {
				t.Fatalf("seeded export = %d events, %v; want 1, nil", len(before), err)
			}

			if _, execErr := openTestStore(t, path).ExecContext(t.Context(), testCase.update); execErr != nil {
				t.Fatalf("unexpected error: %v", execErr)
			}

			after, err := audit.ExportEvents(t.Context(), path, "", exportQuery())
			if err == nil {
				t.Fatal("err is nil, want the row reported rather than skipped")
			}

			if after != nil {
				t.Errorf("events = %v, want none beside the report", after)
			}
		})
	}
}

// TestEncodeReportsAValueItCannotWrite holds every format to reporting a value
// the encoder refuses rather than writing a document without it.
func TestEncodeReportsAValueItCannotWrite(t *testing.T) {
	t.Parallel()

	formats := []string{audit.ExportFormatJSON, audit.ExportFormatNDJSON, audit.ExportFormatCSV}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var written strings.Builder

			err := audit.EncodeEvents(&written, []*audit.Event{eventWithUnwritableArgument(t)}, format)
			if _, refused := errors.AsType[*json.UnsupportedTypeError](err); !refused {
				t.Errorf("err = %v, want the value it would not write named", err)
			}
		})
	}
}

// TestEncodeReportsASinkThatRefuses holds the two formats that write the
// document themselves to reporting a sink that refuses, rather than answering
// as though the export had landed.
func TestEncodeReportsASinkThatRefuses(t *testing.T) {
	t.Parallel()

	formats := []string{audit.ExportFormatJSON, audit.ExportFormatNDJSON}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			event := makeTestEvent(toolOK, audit.CapabilityRead, audit.StatusSuccess, day(19, 8))

			err := audit.EncodeEvents(refusingWriter{}, []*audit.Event{event}, format)
			if !errors.Is(err, errWriterRefused) {
				t.Errorf("err = %v, want %v", err, errWriterRefused)
			}
		})
	}
}

// TestSummarizeGroupsByEveryColumnItAllows reads the accessor per column, which
// the summary tools reach one at a time and no case here covered together.
func TestSummarizeGroupsByEveryColumnItAllows(t *testing.T) {
	t.Parallel()

	event := makeTestEvent(toolOK, audit.CapabilityWrite, audit.StatusSuccess, day(19, 8))
	event.Profile = tcProfile
	event.Environment = tcEnvironment

	columns := []string{colTool, colStatus, tcCapability, tcProfile, tcEnvironment}

	rows := audit.Summarize([]*audit.Event{event}, columns)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want %d", len(rows), 1)
	}

	want := map[string]string{
		colTool:       toolOK,
		colStatus:     string(audit.StatusSuccess),
		tcCapability:  string(audit.CapabilityWrite),
		tcProfile:     tcProfile,
		tcEnvironment: tcEnvironment,
	}

	for column, value := range want {
		if rows[0].Groups[column] != value {
			t.Errorf("rows[0].Groups[%q] = %v, want %v", column, rows[0].Groups[column], value)
		}
	}
}

package audit_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// The two durable formats, held to the bytes the shared fixture carries.
//
// A log line and an export are files an operator's own tooling reads, and until
// now each language wrote them in its own spelling: one member order for the
// log, a different line terminator for the CSV, a trailing newline on one JSON
// document and not the other. The fixture under testdata/audit/record-bytes is
// the one answer both languages are held to, so a tenth language has bytes to
// match rather than two spellings to choose between.
//
// What the fixture cannot hold is written down rather than hidden: the free-form
// args member is each language's own JSON for values the contract declares
// free-form, so a value carrying a backspace or a form feed comes out differently
// (Go writes the escape a code point takes, Python writes the short one) and
// no declaration says which is right.
// Every value the fixture carries is inside what both spell alike.

// recordBytesDir is where the shared fixture lives, relative to this package.
func recordBytesDir() string {
	return filepath.Join("..", "..", "..", "testdata", "audit", "record-bytes")
}

// readRecordFixture is one fixture file's bytes.
func readRecordFixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(recordBytesDir(), name))
	if err != nil {
		t.Fatalf("read shared fixture %s: %v", name, err)
	}

	return raw
}

// fixtureEvents is the shared source read through this language's own record
// reader, which is what keeps a number's written spelling rather than the one a
// plain decode would widen it to.
func fixtureEvents(t *testing.T) []*audit.Event {
	t.Helper()

	var source struct {
		Events []json.RawMessage `json:"events"`
	}

	if err := json.Unmarshal(readRecordFixture(t, "events.json"), &source); err != nil {
		t.Fatalf("parse shared fixture: %v", err)
	}

	events := make([]*audit.Event, 0, len(source.Events))

	for index, raw := range source.Events {
		event, err := genlocal.AuditEventFromRecord(raw)
		if err != nil {
			t.Fatalf("read fixture event %d: %v", index, err)
		}

		events = append(events, event)
	}

	return events
}

// TestRecordWritesTheSharedLogBytes holds the JSONL sink's own line to the
// fixture. The member order is the contract's, so a language writing its struct
// order instead fails here rather than at whatever reads the log later.
func TestRecordWritesTheSharedLogBytes(t *testing.T) {
	t.Parallel()

	var written bytes.Buffer

	for _, event := range fixtureEvents(t) {
		line, err := genlocal.RecordAuditEvent(event, "", "")
		if err != nil {
			t.Fatalf("write record: %v", err)
		}

		written.Write(line)
		written.WriteByte('\n')
	}

	want := readRecordFixture(t, "log.jsonl")
	if !bytes.Equal(written.Bytes(), want) {
		t.Errorf("log bytes:\n got %s\nwant %s", written.Bytes(), want)
	}
}

// TestExportWritesTheSharedDocumentBytes holds all three export documents to
// the fixture: the CSV terminator, the JSON document's trailing newline and the
// member order inside every record.
func TestExportWritesTheSharedDocumentBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		format string
		file   string
	}{
		{format: audit.ExportFormatJSON, file: "export.json"},
		{format: audit.ExportFormatCSV, file: "export.csv"},
		{format: audit.ExportFormatNDJSON, file: "export.ndjson"},
	}

	events := fixtureEvents(t)

	for _, testCase := range cases {
		t.Run(testCase.format, func(t *testing.T) {
			t.Parallel()

			var written bytes.Buffer
			if err := audit.EncodeEvents(&written, events, testCase.format); err != nil {
				t.Fatalf("encode %s: %v", testCase.format, err)
			}

			want := readRecordFixture(t, testCase.file)
			if !bytes.Equal(written.Bytes(), want) {
				t.Errorf("%s bytes:\n got %s\nwant %s", testCase.format, written.Bytes(), want)
			}
		})
	}
}

// TestRecordReadsBackWhatItWrote is the round trip the readers depend on: a
// line the writer produced hydrates into the record it was written from.
func TestRecordReadsBackWhatItWrote(t *testing.T) {
	t.Parallel()

	for index, event := range fixtureEvents(t) {
		line, err := genlocal.RecordAuditEvent(event, "", "")
		if err != nil {
			t.Fatalf("write record %d: %v", index, err)
		}

		read, err := genlocal.AuditEventFromRecord(line)
		if err != nil {
			t.Fatalf("read record %d: %v", index, err)
		}

		again, err := genlocal.RecordAuditEvent(read, "", "")
		if err != nil {
			t.Fatalf("rewrite record %d: %v", index, err)
		}

		if !bytes.Equal(line, again) {
			t.Errorf("record %d round trip:\n got %s\nwant %s", index, again, line)
		}
	}
}

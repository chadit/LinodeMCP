package audit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// TestEventFieldsMatchSharedFixture pins the JSONL wire schema to
// testdata/audit/event_fields.json, the fixture the Python suite asserts
// against too. The audit readers and any external log pipeline parse these
// keys, so a field renamed or added in one language would fork the on-disk
// schema; this test turns that into a per-language failure instead.
func TestEventFieldsMatchSharedFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "testdata", "audit", "event_fields.json",
	))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}

	var fixture struct {
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse shared fixture: %v", err)
	}

	marshaled, err := json.Marshal(audit.Event{})
	if err != nil {
		t.Fatalf("marshal zero event: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(marshaled, &decoded); err != nil {
		t.Fatalf("decode marshaled event: %v", err)
	}

	got := make([]string, 0, len(decoded))
	for key := range decoded {
		got = append(got, key)
	}

	sort.Strings(got)
	sort.Strings(fixture.Fields)

	if len(got) != len(fixture.Fields) {
		t.Fatalf("event marshals %d fields, fixture pins %d:\n got %v\nwant %v",
			len(got), len(fixture.Fields), got, fixture.Fields)
	}

	for index, key := range fixture.Fields {
		if got[index] != key {
			t.Errorf("field %d: event marshals %q, fixture pins %q", index, got[index], key)
		}
	}
}

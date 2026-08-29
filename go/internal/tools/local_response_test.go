package tools_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The projection every declared local answer ends in: an operation answers a
// plain body naming no message, and the tool's own declared message is filled
// from it here. A body that disagrees with the message is reported rather than
// reshaped, which is the whole reason this step exists.

// The members a discard answers, and the sentences a body that disagrees with
// ProfileDraftDiscardResponse is refused with.
const (
	discardName    = "seam-draft"
	discardMember  = "discarded"
	discardMessage = "linode.mcp.v1.ProfileDraftDiscardResponse"

	discardUnfilled       = discardMessage + ": the answer leaves " + discardMember + " unfilled"
	discardUndeclared     = discardMessage + ": the answer names stray, which the message does not declare"
	discardUnfit          = discardMessage + ": the answer does not fit"
	discardUnserializable = discardMessage + ": the answer does not serialize"
)

func TestLocalResponseFillsTheDeclaredMessage(t *testing.T) {
	t.Parallel()

	result, err := tools.LocalResponse(&linodev1.ProfileDraftDiscardResponse{}, map[string]any{
		managedContactNameParam: discardName, discardMember: true,
	})
	if err != nil {
		t.Fatalf("LocalResponse: %v", err)
	}

	answer := resultText(t, result)

	var got map[string]any
	if decodeErr := json.Unmarshal([]byte(answer), &got); decodeErr != nil {
		t.Fatalf("answer is not JSON: %v: %s", decodeErr, answer)
	}

	if got[managedContactNameParam] != discardName || got[discardMember] != true {
		t.Errorf("answer = %s, want the body's own members", answer)
	}
}

// TestLocalResponseRefusesABodyTheMessageDisagreesWith is the loud half: every
// disagreement is answered as an error rather than dropped, in both directions.
func TestLocalResponseRefusesABodyTheMessageDisagreesWith(t *testing.T) {
	t.Parallel()

	cases := []struct {
		body map[string]any
		name string
		want string
	}{
		{
			name: "a member the body leaves out",
			body: map[string]any{managedContactNameParam: discardName},
			want: discardUnfilled,
		},
		{
			name: "a member the message does not declare",
			body: map[string]any{managedContactNameParam: discardName, discardMember: true, "stray": 1},
			want: discardUndeclared,
		},
		{
			name: "a member the message declares under another type",
			body: map[string]any{managedContactNameParam: discardName, discardMember: phase2NonBoolean},
			want: discardUnfit,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := tools.LocalResponse(&linodev1.ProfileDraftDiscardResponse{}, testCase.body)
			if err != nil {
				t.Fatalf("LocalResponse: %v", err)
			}

			answer := resultText(t, result)
			if !strings.HasPrefix(answer, testCase.want) {
				t.Errorf("answer = %q, want it to open with %q", answer, testCase.want)
			}
		})
	}
}

// TestLocalResponseRefusesABodyNothingCanSerialize covers the one disagreement
// the message itself cannot describe: a value no JSON encoder can carry.
func TestLocalResponseRefusesABodyNothingCanSerialize(t *testing.T) {
	t.Parallel()

	result, err := tools.LocalResponse(&linodev1.ProfileDraftDiscardResponse{}, map[string]any{
		managedContactNameParam: discardName, discardMember: make(chan int),
	})
	if err != nil {
		t.Fatalf("LocalResponse: %v", err)
	}

	answer := resultText(t, result)
	if !strings.HasPrefix(answer, discardUnserializable) {
		t.Errorf("answer = %q, want it to open with %q", answer, discardUnserializable)
	}
}

// TestLocalBodyJSONRefusesABodyTheMessageDisagreesWith is the same rule on the
// path a surface outside MCP prints through, which is where the CLI version
// verb reads its bytes.
func TestLocalBodyJSONRefusesABodyTheMessageDisagreesWith(t *testing.T) {
	t.Parallel()

	printed, err := tools.LocalBodyJSON(&linodev1.ProfileDraftDiscardResponse{}, map[string]any{
		managedContactNameParam: discardName,
	})
	if !errors.Is(err, tools.ErrLocalBody) {
		t.Fatalf("LocalBodyJSON printed %s and answered %v, want a refusal carrying %v",
			printed, err, tools.ErrLocalBody)
	}

	want := tools.ErrLocalBody.Error() + ": " + discardUnfilled
	if err.Error() != want {
		t.Errorf("refusal = %q, want %q", err, want)
	}
}

// TestVersionResponseJSONAnswersTheBuildInfoBody holds the CLI verb's bytes to
// the body the tool answers from, which is what keeps the two surfaces from
// drifting a member apart.
func TestVersionResponseJSONAnswersTheBuildInfoBody(t *testing.T) {
	t.Parallel()

	printed, err := tools.VersionResponseJSON()
	if err != nil {
		t.Fatalf("VersionResponseJSON: %v", err)
	}

	var got map[string]any
	if decodeErr := json.Unmarshal(printed, &got); decodeErr != nil {
		t.Fatalf("printed version is not JSON: %v: %s", decodeErr, printed)
	}

	body := tools.VersionResponseBody()
	if len(got) != len(body) {
		t.Fatalf("printed %d member(s), the body carries %d: %s", len(got), len(body), printed)
	}

	for member, value := range body {
		if got[member] != value {
			t.Errorf("printed %s = %v, the body answers %v", member, got[member], value)
		}
	}
}

// The nested half of the same rule. A record a member carries is a message of
// its own, and protojson reads a member that record leaves out as that member's
// zero, so an incomplete record answers a value nothing computed and reports
// success. These hold the walk that closes it.

// The messages and members a nested case names.
const (
	auditEventMessage  = "linode.mcp.v1.AuditEvent"
	auditSQLiteMessage = "linode.mcp.v1.AuditHealthSQLite"

	auditMemberEvents  = "events"
	auditMemberCount   = "count"
	auditMemberSQLite  = "sqlite"
	auditMemberDBBytes = "db_bytes"

	auditEventUnfilled   = auditEventMessage + ": the answer leaves session_id unfilled"
	auditEventUndeclared = auditEventMessage +
		": the answer names stray, which the message does not declare"
	auditSQLiteUnfilled = auditSQLiteMessage + ": the answer leaves " +
		auditMemberDBBytes + " unfilled"
	auditRecentUnfit = "linode.mcp.v1.AuditRecentResponse: the answer does not fit"
)

// auditEventRecord is one event as the engine hands it over, spelled out rather
// than read off the descriptor: a case reading the message it is meant to hold
// the record to would pass whatever that message said.
func auditEventRecord() map[string]any {
	return map[string]any{
		"ts":                    "2026-05-20T00:00:01Z",
		"ts_unix_ns":            1779235201000000000,
		"event_id":              "evt_nested",
		"tool":                  canRunReadTool,
		"tool_capability":       capabilityRead,
		"environment":           envKeyDefault,
		"profile":               profileOperator,
		"mode":                  "normal",
		"plan_id":               nil,
		"args":                  map[string]any{},
		"args_redacted":         []any{},
		keyStatus:               "success",
		"latency_ms":            0,
		"result_summary":        "",
		"error":                 nil,
		"linodemcp_version":     "0.1.0",
		"session_id":            "sess-nested",
		"credential_generation": 0,
	}
}

// auditHealthBody is one health answer whose store section is filled, which is
// the singular nested member the rule walks.
func auditHealthBody(sqlite map[string]any) map[string]any {
	return map[string]any{
		"jsonl_path":          "audit-dir/audit.log",
		"active_log_exists":   true,
		"rotated_file_count":  0,
		"oldest_rotated_date": "",
		"disk_bytes":          0,
		"dropped_events":      0,
		auditMemberSQLite:     sqlite,
		"warnings":            []any{},
	}
}

// TestLocalResponseFillsANestedRecord is the control the refusing cases need:
// a complete record reaches the message and answers its own members.
func TestLocalResponseFillsANestedRecord(t *testing.T) {
	t.Parallel()

	result, err := tools.LocalResponse(&linodev1.AuditRecentResponse{}, map[string]any{
		auditMemberCount: 1, auditMemberEvents: []any{auditEventRecord()},
	})
	if err != nil {
		t.Fatalf("LocalResponse: %v", err)
	}

	answer := resultText(t, result)

	var got map[string]any
	if decodeErr := json.Unmarshal([]byte(answer), &got); decodeErr != nil {
		t.Fatalf("answer is not JSON: %v: %s", decodeErr, answer)
	}

	events, ok := got[auditMemberEvents].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("answer = %s, want one event", answer)
	}
}

// TestLocalResponseRefusesANestedRecordTheMessageDisagreesWith is the rule the
// top-level walk alone cannot reach, in both directions and at both shapes: a
// list of records and the single record a member carries.
func TestLocalResponseRefusesANestedRecordTheMessageDisagreesWith(t *testing.T) {
	t.Parallel()

	incomplete := auditEventRecord()
	delete(incomplete, "session_id")

	undeclared := auditEventRecord()
	undeclared["stray"] = 1

	shortStore := map[string]any{
		"path": "audit-dir/audit.db", "event_count": 0, "oldest_event_unix_ns": 0,
	}

	cases := []struct {
		message proto.Message
		body    map[string]any
		name    string
		want    string
	}{
		{
			name:    "a listed record leaving a member out",
			message: &linodev1.AuditRecentResponse{},
			body:    map[string]any{auditMemberCount: 1, auditMemberEvents: []any{incomplete}},
			want:    auditEventUnfilled,
		},
		{
			name:    "a listed record naming a member the message does not declare",
			message: &linodev1.AuditRecentResponse{},
			body:    map[string]any{auditMemberCount: 1, auditMemberEvents: []any{undeclared}},
			want:    auditEventUndeclared,
		},
		{
			name:    "the single record a member carries leaving a member out",
			message: &linodev1.AuditHealthResponse{},
			body:    auditHealthBody(shortStore),
			want:    auditSQLiteUnfilled,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := tools.LocalResponse(testCase.message, testCase.body)
			if err != nil {
				t.Fatalf("LocalResponse: %v", err)
			}

			answer := resultText(t, result)
			if !strings.HasPrefix(answer, testCase.want) {
				t.Errorf("answer = %q, want it to open with %q", answer, testCase.want)
			}
		})
	}
}

// TestLocalResponseReadsAListedValueThatIsNoRecord covers the one shape the
// walk cannot hold: a listed value that is not a record at all is left to the
// fill below, which reports the type disagreement in its own words.
func TestLocalResponseReadsAListedValueThatIsNoRecord(t *testing.T) {
	t.Parallel()

	result, err := tools.LocalResponse(&linodev1.AuditRecentResponse{}, map[string]any{
		auditMemberCount: 1, auditMemberEvents: []any{"not a record"},
	})
	if err != nil {
		t.Fatalf("LocalResponse: %v", err)
	}

	answer := resultText(t, result)
	if !strings.HasPrefix(answer, auditRecentUnfit) {
		t.Errorf("answer = %q, want it to open with %q", answer, auditRecentUnfit)
	}
}

// TestLocalResponseReadsAnAbsentNestedRecordAsAbsent covers the member the
// projection leaves nil: a store section nothing filled is not a record that
// left every member out.
func TestLocalResponseReadsAnAbsentNestedRecordAsAbsent(t *testing.T) {
	t.Parallel()

	result, err := tools.LocalResponse(&linodev1.AuditHealthResponse{}, auditHealthBody(nil))
	if err != nil {
		t.Fatalf("LocalResponse: %v", err)
	}

	answer := resultText(t, result)

	var got map[string]any
	if decodeErr := json.Unmarshal([]byte(answer), &got); decodeErr != nil {
		t.Fatalf("answer is not JSON: %v: %s", decodeErr, answer)
	}

	if _, filled := got[auditMemberSQLite]; filled {
		t.Errorf("answer = %s, want no store section", answer)
	}
}

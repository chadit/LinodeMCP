package main_test

import (
	"errors"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals the answer projection and the record directions raise.
//
// The projection is emitted from the response descriptor, so what it can refuse
// is a shape the vocabulary has no spelling for. A record adds two more: a
// record carrying another record, which no serializer here spells, and a
// message declaring itself one that no operation answers with. All of them are
// stated through a probe, since the shipped declarations ride on the contract's
// own members and a case can perturb none of them.

// theRecordMessage is the one message the contract declares a record today,
// named here so the reached case can leave it out of a stated set.
const theRecordMessage = "linode.mcp.v1.AuditEvent"

// The shape names the ordering cases state. They stand for nothing in the
// contract: the ordering is what they exercise, not any message.
const (
	probeAlone  = "Alone"
	probeFirst  = "First"
	probeSecond = "Second"
	probeMiddle = "Middle"
	probeLeaf   = "Leaf"
)

// TestRefusesARecordCarryingAnotherRecord holds a record shape to members a
// record direction can spell. A record's whole point is that its member ORDER
// is the message's own, so a nested record would need its order held too, in a
// serializer that recursed and a reader that knew where to stop.
func TestRefusesARecordCarryingAnotherRecord(t *testing.T) {
	t.Parallel()

	cases := []struct {
		shape proto.Message
		name  string
	}{
		{name: "a record carrying a list of records", shape: &linodev1.AuditReportResponse{}},
		{name: "a record carrying one record", shape: &linodev1.AuditHealthResponse{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeAnswerRecord(testCase.shape)

			if want := refusalNamed(t, "errLocalRecordNested"); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want errLocalRecordNested (%v)", err, want)
			}
		})
	}
}

// TestAcceptsARecordCarryingValuesAlone is what says the nested case measures
// something: the shape the contract really declares a record clears the same
// check the two above fail.
func TestAcceptsARecordCarryingValuesAlone(t *testing.T) {
	t.Parallel()

	if err := toolgen.ProbeAnswerRecord(&linodev1.AuditEvent{}); err != nil {
		t.Errorf("record check = %v, want nil", err)
	}
}

// TestRefusesARecordNoOperationAnswersWith holds every message declaring
// local_record to the answer surface. Outside it the message gets neither
// direction emitted, and the declaration would read as met while the engine
// that persists the record still spelled it by hand.
func TestRefusesARecordNoOperationAnswersWith(t *testing.T) {
	t.Parallel()

	cases := []struct {
		shapes map[string]bool
		name   string
	}{
		{name: "no shape at all", shapes: nil},
		{
			name:   "the message reached, and not as a record",
			shapes: map[string]bool{theRecordMessage: false},
		},
		{
			name:   "another record reached instead",
			shapes: map[string]bool{"linode.mcp.v1.AuditSummaryRow": true},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeRecordsReached(testCase.shapes)

			if want := refusalNamed(t, "errLocalRecordUnreached"); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want errLocalRecordUnreached (%v)", err, want)
			}
		})
	}
}

// TestTheContractDeclaresTheAuditEventARecord is the control the reached cases
// perturb. Without it a contract that declared no record at all would pass
// every case above by having nothing to miss.
func TestTheContractDeclaresTheAuditEventARecord(t *testing.T) {
	t.Parallel()

	declared := toolgen.ProbeDeclaredRecords()

	if !slices.Equal(declared, []string{theRecordMessage}) {
		t.Errorf("declared records = %v, want %v", declared, []string{theRecordMessage})
	}

	if err := toolgen.ProbeRecordsReached(map[string]bool{theRecordMessage: true}); err != nil {
		t.Errorf("reached check = %v, want nil", err)
	}
}

// TestRefusesAMemberTheProjectionCannotBuild is the answer half of the
// typed-parameter guard. A member whose type the vocabulary has no spelling for
// would reach one language as a value the other could not be held to, which is
// the drift the projection exists to remove. Account carries a double, which
// nothing in the closed set spells.
func TestRefusesAMemberTheProjectionCannotBuild(t *testing.T) {
	t.Parallel()

	err := toolgen.ProbeAnswerShapes([]toolgen.ProbeAnswerShape{{
		Shape: &linodev1.Account{},
	}})

	if want := refusalNamed(t, "errLocalShapeMemberKind"); !errors.Is(err, want) {
		t.Errorf("refusal = %v, want errLocalShapeMemberKind (%v)", err, want)
	}
}

// TestRefusesTwoMessagesOneLanguageSpellsAlike holds the shape table's own key
// to one message. Two messages landing on one type name would emit whichever
// was read first, and the run would not say which.
func TestRefusesTwoMessagesOneLanguageSpellsAlike(t *testing.T) {
	t.Parallel()

	err := toolgen.ProbeAnswerCollision(probeAlone, "one.v1.Alone", "other.v1.Alone")

	if want := refusalNamed(t, "errLocalShapeShared"); !errors.Is(err, want) {
		t.Errorf("refusal = %v, want errLocalShapeShared (%v)", err, want)
	}
}

// TestAcceptsOneMessageReachedTwice is what says the collision case measures
// something: a shape reached from two declarations is built from the descriptor
// alone, so the second reading is the first one again.
func TestAcceptsOneMessageReachedTwice(t *testing.T) {
	t.Parallel()

	if err := toolgen.ProbeAnswerCollision(probeAlone, "one.v1.Alone", "one.v1.Alone"); err != nil {
		t.Errorf("collision check = %v, want nil", err)
	}
}

// TestRefusesShapesThatReachEachOther covers the ordering: a language emitting
// value types in dependency order cannot write a pair that names each other, so
// the ordering refuses rather than emitting one of them first and hoping.
func TestRefusesShapesThatReachEachOther(t *testing.T) {
	t.Parallel()

	cases := []struct {
		reaches map[string][]string
		name    string
	}{
		{name: "a shape naming itself", reaches: map[string][]string{probeAlone: {probeAlone}}},
		{
			name:    "two shapes naming each other",
			reaches: map[string][]string{probeFirst: {probeSecond}, probeSecond: {probeFirst}},
		},
		{
			name: "a pair reached from a shape that would place",
			reaches: map[string][]string{
				"Root": {probeFirst}, probeFirst: {probeSecond}, probeSecond: {probeFirst},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := toolgen.ProbeAnswerOrdering(testCase.reaches)

			if want := refusalNamed(t, "errLocalShapeCycle"); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want errLocalShapeCycle (%v)", err, want)
			}
		})
	}
}

// TestOrdersEveryNestedShapeAheadOfItsNamer pins what the ordering is for. A
// language declaring a type before it is named reads the emitted list straight
// through, so a shape landing after the one naming it would not compile there.
func TestOrdersEveryNestedShapeAheadOfItsNamer(t *testing.T) {
	t.Parallel()

	reaches := map[string][]string{
		"Outer":     {probeMiddle, probeLeaf},
		probeMiddle: {probeLeaf},
		probeLeaf:   nil,
		"Other":     nil,
	}

	ordered, err := toolgen.ProbeAnswerOrdering(reaches)
	if err != nil {
		t.Fatalf("ordering: %v", err)
	}

	for namer, nested := range reaches {
		for _, reached := range nested {
			if slices.Index(ordered, reached) > slices.Index(ordered, namer) {
				t.Errorf("%s lands after %s, which names it: %v", reached, namer, ordered)
			}
		}
	}
}

// TestProjectsEveryMemberTheShapeDeclares pins the member set the body carries.
// A projection short one member would answer a caller a zero nothing computed,
// which is the divergence the nested case is here to close: the sqlite section
// used to be spelled by hand in one language and reflected over in the other.
func TestProjectsEveryMemberTheShapeDeclares(t *testing.T) {
	t.Parallel()

	cases := []struct {
		shape   proto.Message
		name    string
		members []string
	}{
		{
			name:  "the nested sqlite section, member for member",
			shape: &linodev1.AuditHealthSQLite{},
			members: []string{
				"path", "event_count", "oldest_event_unix_ns", "db_bytes",
			},
		},
		{
			name:  "the health answer around it",
			shape: &linodev1.AuditHealthResponse{},
			members: []string{
				"jsonl_path", "active_log_exists", "rotated_file_count",
				"oldest_rotated_date", "disk_bytes", "dropped_events", "sqlite",
				"warnings",
			},
		},
		{
			name:  "the audit record, member for member in its own order",
			shape: &linodev1.AuditEvent{},
			members: []string{
				"ts", "ts_unix_ns", "event_id", probeToolGlobArg, "tool_capability",
				"environment", "profile", "mode", "plan_id", "args",
				"args_redacted", "status", "latency_ms", "result_summary",
				"error", "linodemcp_version", "session_id",
				"credential_generation",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			members, err := toolgen.ProbeAnswerBody(testCase.shape)
			if err != nil {
				t.Fatalf("shape: %v", err)
			}

			if !slices.Equal(members, testCase.members) {
				t.Errorf("members = %v, want %v", members, testCase.members)
			}
		})
	}
}

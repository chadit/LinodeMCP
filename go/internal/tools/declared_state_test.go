package tools_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The Python twin is test_declared_state.py: the same projection rule and the
// same refusal sentence.

// projected is the state a declared fetch reports for one body and one read.
func projected(t *testing.T, body string, msg proto.Message) tools.DeclaredState {
	t.Helper()

	state, err := tools.ProjectDeclaredState(json.RawMessage(body), msg)
	if err != nil {
		t.Fatalf("ProjectDeclaredState: %v", err)
	}

	declared, err := tools.DeclaredStateOf(state)
	if err != nil {
		t.Fatalf("DeclaredStateOf: %v", err)
	}

	return declared
}

// stateJSON is what a preview reports and a plan hashes, which is the state
// marshaled the way both of them reach it.
func stateJSON(t *testing.T, state tools.DeclaredState) string {
	t.Helper()

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	return string(data)
}

// TestProjectionKeepsOnlyWhatTheReadModels: a member the API sent that the
// read's message does not carry is dropped, so the preview stays inside the
// shape the read tool answers with.
func TestProjectionKeepsOnlyWhatTheReadModels(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":42,"label":"audit log sink","unknown_member":"dropped"}`,
		&linodev1.MonitorStreamDestination{})

	if want := `{"id":42,"label":"audit log sink"}`; stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestProjectionInventsNoMemberTheAPILeftOut: the serializer emits every
// declared member at its zero, which reads as a resource carrying values the
// API never mentioned.
func TestProjectionInventsNoMemberTheAPILeftOut(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":42}`, &linodev1.Volume{})

	if want := `{"id":42}`; stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestProjectionReportsTheNullsTheAPISent: a key sent as null is the API saying
// there is none, which a missing key cannot say, and a plan hashes the two
// differently.
func TestProjectionReportsTheNullsTheAPISent(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":42,"linode_id":null}`, &linodev1.Volume{})

	if want := `{"id":42,"linode_id":null}`; stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestProjectionReportsAnEmptyListTheAPISentAsNull: protojson writes [] for a
// repeated field whatever the body said, so a documented null came back as an
// empty list and no declaration could restore it.
func TestProjectionReportsAnEmptyListTheAPISentAsNull(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":42,"tags":null}`, &linodev1.LKECluster{})

	if want := `{"id":42,"tags":null}`; stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestProjectionReachesANullUnderANestedMember: restoration reached one level,
// so a null under a sub-message came back missing however it was declared.
func TestProjectionReachesANullUnderANestedMember(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":8,"entity":{"id":5,"label":"web-01","parent_entity":null}}`,
		&linodev1.FirewallDevice{})

	want := `{"entity":{"id":5,"label":"web-01","parent_entity":null},"id":8}`
	if stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestProjectionReadsEachElementOfARepeatedMember: a pool's nodes are objects
// of their own, and a walk reads them the way it reads the resource.
func TestProjectionReadsEachElementOfARepeatedMember(t *testing.T) {
	t.Parallel()

	state := projected(t,
		`{"id":7,"nodes":[{"id":"node-a","instance_id":11,"unknown":"dropped"},{"id":"node-b","instance_id":22}]}`,
		&linodev1.LKENodePool{})

	nodes := state.Objects("nodes")
	if len(nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(nodes))
	}

	if got, _ := nodes[0].Number("instance_id"); got != 11 {
		t.Errorf("instance_id = %d, want 11", got)
	}

	if want := `{"id":7,"nodes":[{"id":"node-a","instance_id":11},{"id":"node-b","instance_id":22}]}`; stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestProjectionPassesAFreeFormMemberWhole: a Struct member holds the API's own
// object, so descending into it with the wrapper's descriptor would empty it.
func TestProjectionPassesAFreeFormMemberWhole(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":42,"details":{"bucket_name":"audit-logs","host":"us-east-1.example.com"}}`,
		&linodev1.MonitorStreamDestination{})

	want := `{"details":{"bucket_name":"audit-logs","host":"us-east-1.example.com"},"id":42}`
	if stateJSON(t, state) != want {
		t.Errorf("state = %s, want %s", stateJSON(t, state), want)
	}
}

// TestAccessorsReadTheMembersAWalkNames: the three readers are the whole set
// the walks need.
func TestAccessorsReadTheMembersAWalkNames(t *testing.T) {
	t.Parallel()

	state := projected(t,
		`{"id":333,"linode_id":9,"linode_label":`+strconv.Quote(firewallDeviceLabelFixture)+`}`,
		&linodev1.Volume{})

	id, carried := state.Number("linode_id")
	if !carried || id != 9 {
		t.Errorf("linode_id = %d, %v, want 9, true", id, carried)
	}

	if got := state.Text("linode_label"); got != firewallDeviceLabelFixture {
		t.Errorf("linode_label = %q, want %q", got, firewallDeviceLabelFixture)
	}
}

// TestNumberAnswersNothingForAMemberThatIsNotAWholeNumber: an absent member and
// one the API sent as null both read as no value, and a bool would otherwise
// name instance 1 as a dependency.
func TestNumberAnswersNothingForAMemberThatIsNotAWholeNumber(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":333,"linode_id":null}`, &linodev1.Volume{})

	if _, carried := state.Number("linode_id"); carried {
		t.Error("a null member read as a number")
	}

	if _, carried := state.Number("size"); carried {
		t.Error("a member the API never sent read as a number")
	}

	if got := state.Text("label"); got != "" {
		t.Errorf("label = %q, want empty for a member the API never sent", got)
	}

	if got := state.Objects("tags"); got != nil {
		t.Errorf("tags = %v, want nothing for a member the API never sent", got)
	}
}

// TestProjectionKeepsAMemberTheAPISentUnderTheWrongShape: the API is what
// decides the shape, so a member arriving as something the contract does not
// model reaches the preview as it came rather than as an empty object.
func TestProjectionKeepsAMemberTheAPISentUnderTheWrongShape(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":8,"entity":5,"rules":7}`, &linodev1.FirewallDevice{})

	if got := stateJSON(t, state); got != `{"entity":5,"id":8}` {
		t.Errorf("state = %s, want the members as the API sent them", got)
	}
}

// TestProjectionKeepsAListElementThatIsNotAnObject: a repeated message member
// whose elements arrive as scalars is the same case one level down.
func TestProjectionKeepsAListElementThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":7,"nodes":[5,{"id":"node-b"}]}`, &linodev1.LKENodePool{})

	if got := stateJSON(t, state); got != `{"id":7,"nodes":[5,{"id":"node-b"}]}` {
		t.Errorf("state = %s, want the element as the API sent it", got)
	}
}

// TestProjectionKeepsARepeatedMemberThatIsNotAList: same again for the member
// itself, which a caller reads rather than a walk.
func TestProjectionKeepsARepeatedMemberThatIsNotAList(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":7,"nodes":5}`, &linodev1.LKENodePool{})

	if got := stateJSON(t, state); got != `{"id":7,"nodes":5}` {
		t.Errorf("state = %s, want the member as the API sent it", got)
	}
}

// TestNumberAnswersNothingForANumberThatIsNotWhole: a fractional id is not an
// id, and rounding one would name a resource nobody addressed.
func TestNumberAnswersNothingForANumberThatIsNotWhole(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":333,"linode_id":1.5}`, &linodev1.Volume{})

	if _, carried := state.Number("linode_id"); carried {
		t.Error("a fractional member read as a whole number")
	}
}

// TestObjectsSkipsAnElementThatIsNotAnObject: a walk reading a list of objects
// gets the objects, and nothing else in that list becomes one.
func TestObjectsSkipsAnElementThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	state := projected(t, `{"id":7,"nodes":[5,{"id":"node-b"}]}`, &linodev1.LKENodePool{})

	nodes := state.Objects("nodes")
	if len(nodes) != 1 || nodes[0].Text("id") != "node-b" {
		t.Errorf("nodes = %v, want the one object the list carried", nodes)
	}
}

// TestDeclaredStateOfRefusesWhatNoDeclaredFetchProduced: a walk paired with a
// hand-written fetch fails where the report names it, rather than reporting no
// dependencies and looking like a resource with none.
func TestDeclaredStateOfRefusesWhatNoDeclaredFetchProduced(t *testing.T) {
	t.Parallel()

	for name, state := range map[string]any{
		"a typed resource": &linodev1.Volume{Id: 333},
		"a bare map":       map[string]any{"id": 333},
		"nothing at all":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := tools.DeclaredStateOf(state); !errors.Is(err, tools.ErrStateNotDeclared) {
				t.Errorf("DeclaredStateOf(%s) = %v, want %v", name, err, tools.ErrStateNotDeclared)
			}
		})
	}
}

// TestProjectionRefusesABodyThatIsNotAnObject: a bare array carries no member
// to project, and answering an empty state would preview a resource nobody
// could read.
func TestProjectionRefusesABodyThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	if _, err := tools.ProjectDeclaredState(json.RawMessage(`[{"id":42}]`), &linodev1.Volume{}); err == nil {
		t.Error("a bare array projected as a resource")
	}

	if _, err := tools.ProjectDeclaredState(json.RawMessage(`null`), &linodev1.Volume{}); err == nil {
		t.Error("a null body projected as a resource")
	}
}

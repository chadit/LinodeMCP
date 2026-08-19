package tools_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// keyDryRun is the dry-run argument name in MCP request maps. Local
// to this file because the package-internal `paramDryRun` constant
// in helpers.go isn't reachable from `package tools_test`.
const keyDryRun = "dry_run"

// TestIsDryRunTrue covers the happy path: a request whose dry_run
// argument is the literal JSON boolean true.
func TestIsDryRunTrue(t *testing.T) {
	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	if !tools.IsDryRun(&req) {
		t.Error("expected condition to be true")
	}
}

// TestIsDryRunFalse covers explicit opt-out.
func TestIsDryRunFalse(t *testing.T) {
	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{keyDryRun: false})

	if tools.IsDryRun(&req) {
		t.Error("expected condition to be false")
	}
}

// TestIsDryRunMissing locks the default-false rule: a request that
// omits dry_run is the same as dry_run:false. The dry-run spec is
// explicit that omitting the parameter means "execute the call".
func TestIsDryRunMissing(t *testing.T) {
	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{})

	if tools.IsDryRun(&req) {
		t.Error("expected condition to be false")
	}
}

// TestIsDryRunWrongType locks the strict-bool rule: only the literal
// JSON boolean true counts as dry-run. A string "true", numeric 1, or
// nil all degrade to false. This is a defensive default for a case
// that MCP schema validation already prevents (the tool registers
// dry_run as a boolean param), so a wrong-type value reaching the
// handler implies a schema or client-library bug upstream. The
// strict-bool path keeps behavior predictable: same input shape,
// same output, no string-truthiness surprises.
func TestIsDryRunWrongType(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"string-true":  boolStringTrue,
		"string-false": caseFalse,
		"numeric-one":  1,
		"numeric-zero": 0,
		"nil":          nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, map[string]any{keyDryRun: value})

			if tools.IsDryRun(&req) {
				t.Error("expected condition to be false")
			}
		})
	}
}

// TestBuildDryRunResponseShape locks the v0 wire shape that Phase 2
// will elevate. The test decodes back into a generic map so a future
// struct field reorder doesn't accidentally rename a JSON key without
// the test catching it.
func TestBuildDryRunResponseShape(t *testing.T) {
	t.Parallel()

	currentState := map[string]any{
		keyBetaID: 12345,
		keyLabel:  firewallDeviceLabelFixture,
		keyStatus: statusRunning,
	}

	result, err := tools.BuildDryRunResponse(
		"linode_instance_delete",
		canRunEnvProd,
		"DELETE",
		"/linode/instances/12345",
		currentState,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	body := dryRunResultText(t, result)

	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for key, want := range map[string]any{
		keyDryRun:     true,
		canRunKeyTool: "linode_instance_delete",
		canRunKeyEnv:  canRunEnvProd,
	} {
		if !reflect.DeepEqual(got[key], want) {
			t.Errorf("got[%v] = %v, want %v", key, got[key], want)
		}
	}

	would, isObject := got["would_execute"].(map[string]any)
	if !isObject {
		t.Fatal("isObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/linode/instances/12345") {
		t.Errorf("got %v, want %v", would["path"], "/linode/instances/12345")
	}

	state, stateIsObject := got["current_state"].(map[string]any)
	if !stateIsObject {
		t.Fatal("stateIsObject = false, want true")
	}

	for key, want := range map[string]any{
		keyBetaID: float64(12345),
		keyLabel:  firewallDeviceLabelFixture,
		keyStatus: statusRunning,
	} {
		if !reflect.DeepEqual(state[key], want) {
			t.Errorf("state[%v] = %v, want %v", key, state[key], want)
		}
	}
}

// TestBuildDryRunResponseOmitsEmptyEnvironment guards the omitempty
// JSON tag on Environment. A tool that didn't accept an environment
// argument should produce a response without the canRunKeyEnv key,
// not with an empty string. Models reading the wire shape can then
// distinguish absent from present-but-empty.
func TestBuildDryRunResponseOmitsEmptyEnvironment(t *testing.T) {
	t.Parallel()

	result, err := tools.BuildDryRunResponse(
		"linode_audit_health",
		"",
		"GET",
		"/linode/audit/health",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := dryRunResultText(t, result)

	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, hasEnv := got[canRunKeyEnv]
	if hasEnv {
		t.Error("hasEnv = true, want false")
	}
}

// TestBuildDryRunResponseRejectsUndecodableState covers the second half of the
// current_state conversion: the value encodes to JSON cleanly and then fails to
// decode back. json.Number carries the literal text through the encoder
// untouched, so a magnitude no float64 can hold only blows up on the way back
// in. The preview has to fail rather than ship a current_state the operator
// would read as the real resource before approving a destructive call.
func TestBuildDryRunResponseRejectsUndecodableState(t *testing.T) {
	t.Parallel()

	result, err := tools.BuildDryRunResponse(
		"linode_instance_delete",
		"",
		"DELETE",
		"/linode/instances/123",
		map[string]any{"transfer_bytes": json.Number("1e1000")},
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %+v, want nil", result)
	}
}

// assertDryRunRequest checks the would_execute preview of a decoded dry-run or
// plan body carries the expected method and path. The dry-run and plan
// envelopes serialize through protojson now, which varies colon spacing, so the
// tests that used byte-exact `"method": "POST"` substrings decode the JSON and
// assert on values through this helper instead.
func assertDryRunRequest(t *testing.T, body map[string]any, method, path string) {
	t.Helper()

	would, ok := body["would_execute"].(map[string]any)
	if !ok {
		t.Fatalf("would_execute is not an object: %v", body["would_execute"])
	}

	if would["method"] != method {
		t.Errorf("would_execute.method = %v, want %v", would["method"], method)
	}

	if would["path"] != path {
		t.Errorf("would_execute.path = %v, want %v", would["path"], path)
	}
}

// dryRunResultText pulls the first text-content body off an MCP tool
// result. Local to this test file rather than shared with
// tools_test.go because the tests here decode JSON; the existing
// inline-cast pattern in tools_test.go does substring assertions.
func dryRunResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	return text.Text
}

// TestProtoStateListSerializesThroughTheDescriptor pins the preview convention
// the two firewall replacements read their state under: every member the
// descriptor declares, including the ones sitting at their zero, so the state
// reads the way the elements in the tool's own answer do.
func TestProtoStateListSerializesThroughTheDescriptor(t *testing.T) {
	t.Parallel()

	state, err := tools.ProtoStateList([]*linodev1.Firewall{
		{Id: 9, Label: "edge", Status: statusEnabled},
	}, nil)
	if err != nil {
		t.Fatalf("ProtoStateList: %v", err)
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	var got, want any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode state: %v", err)
	}

	if err := json.Unmarshal([]byte(
		`[{"id":9,"label":"edge","status":"enabled","tags":[],"created":"","updated":""}]`,
	), &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("state = %s, want the descriptor's own shape", encoded)
	}
}

// TestProtoStateListCarriesTheFetchFailure covers the read that never
// answered: the preview has no state to report and must say so rather than
// report an empty collection as the current one.
func TestProtoStateListCarriesTheFetchFailure(t *testing.T) {
	t.Parallel()

	state, err := tools.ProtoStateList([]*linodev1.Firewall{}, errStateFetch)
	if !errors.Is(err, errStateFetch) {
		t.Errorf("err = %v, want the fetch failure", err)
	}

	if state != nil {
		t.Errorf("state = %v, want none", state)
	}
}

// TestProtoStateListRefusesAnUnserializableElement covers the element the
// descriptor cannot spell: a proto3 string field holds UTF-8, so a label that
// is not reaches the marshaller as a failure rather than as bytes no client
// could read.
func TestProtoStateListRefusesAnUnserializableElement(t *testing.T) {
	t.Parallel()

	state, err := tools.ProtoStateList([]*linodev1.Firewall{{Id: 9, Label: "\xff"}}, nil)
	if err == nil {
		t.Fatal("err = nil, want the invalid element refused")
	}

	if state != nil {
		t.Errorf("state = %v, want none", state)
	}
}

// errStateFetch stands in for a read that failed before the preview could
// serialize anything.
var errStateFetch = errors.New("fetch failed")

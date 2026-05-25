package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/tools"
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

	assert.True(t, tools.IsDryRun(&req))
}

// TestIsDryRunFalse covers explicit opt-out.
func TestIsDryRunFalse(t *testing.T) {
	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{keyDryRun: false})

	assert.False(t, tools.IsDryRun(&req))
}

// TestIsDryRunMissing locks the default-false rule: a request that
// omits dry_run is the same as dry_run:false. The dry-run spec is
// explicit that omitting the parameter means "execute the call".
func TestIsDryRunMissing(t *testing.T) {
	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{})

	assert.False(t, tools.IsDryRun(&req),
		"omitted dry_run must default to false (execute the call)")
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

			assert.False(t, tools.IsDryRun(&req),
				"non-bool %v must not satisfy dry_run", name)
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
		keyLabel:  "web-01",
		keyStatus: "running",
	}

	result, err := tools.BuildDryRunResponse(
		"linode_instance_delete",
		"prod",
		"DELETE",
		"/linode/instances/12345",
		currentState,
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	body := dryRunResultText(t, result)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &got))

	assert.Equal(t, true, got[keyDryRun])
	assert.Equal(t, "linode_instance_delete", got["tool"])
	assert.Equal(t, "prod", got["environment"])

	would, isObject := got["would_execute"].(map[string]any)
	require.True(t, isObject, "would_execute must be a JSON object")
	assert.Equal(t, "DELETE", would["method"])
	assert.Equal(t, "/linode/instances/12345", would["path"])

	state, stateIsObject := got["current_state"].(map[string]any)
	require.True(t, stateIsObject, "current_state must round-trip as an object")
	assert.InDelta(t, 12345, state[keyBetaID], 0)
	assert.Equal(t, "web-01", state[keyLabel])
	assert.Equal(t, "running", state[keyStatus])
}

// TestBuildDryRunResponseOmitsEmptyEnvironment guards the omitempty
// JSON tag on Environment. A tool that didn't accept an environment
// argument should produce a response without the "environment" key,
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
	require.NoError(t, err)

	body := dryRunResultText(t, result)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &got))

	_, hasEnv := got["environment"]
	assert.False(t, hasEnv,
		"environment must be omitted when empty, not serialized as empty string")
}

// dryRunResultText pulls the first text-content body off an MCP tool
// result. Local to this test file rather than shared with
// tools_test.go because the tests here decode JSON; the existing
// inline-cast pattern in tools_test.go does substring assertions.
func dryRunResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	require.NotEmpty(t, result.Content, "result must carry at least one content block")

	text, isText := result.Content[0].(mcp.TextContent)
	require.True(t, isText, "first content block must be TextContent, got %T", result.Content[0])

	return text.Text
}

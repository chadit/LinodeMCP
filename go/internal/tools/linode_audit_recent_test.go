package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// auditRecentResult mirrors the tool's JSON response so the test can
// decode and assert on it.
type auditRecentResult struct {
	Count  int           `json:"count"`
	Events []audit.Event `json:"events"`
}

// TestLinodeAuditRecentDefinition pins the tool's identity: name,
// CapMeta tag, and the documented filter parameters.
func TestLinodeAuditRecentDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	assert.Equal(t, "linode_audit_recent", tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapMeta, capability, "audit query is CapMeta so every profile can read it")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	for _, param := range []string{"limit", keySince, "until", "tool", "capability", "status", "include_meta"} {
		assert.Contains(t, props, param, "schema should declare the %q filter", param)
	}

	assert.NotContains(t, props, "confirm", "a read-only query must not declare confirm")
}

// TestLinodeAuditRecentReturnsEvents drives the handler end-to-end
// against a temp audit directory (pointed at via XDG_STATE_HOME). It
// confirms the response envelope, newest-first order, and the
// default meta exclusion.
func TestLinodeAuditRecentReturnsEvents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	require.NoError(t, os.MkdirAll(auditDir, 0o750), "create audit dir")

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, 2),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 3),
	})

	_, _, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	require.NoError(t, err, "handler must not error")
	require.NotNil(t, result, "result must not be nil")
	assert.False(t, result.IsError, "default query must succeed")

	decoded := decodeAuditResult(t, result)
	assert.Equal(t, 2, decoded.Count, "meta event excluded by default leaves two")
	require.Len(t, decoded.Events, 2, "two events returned")
	assert.Equal(t, "linode_instance_delete", decoded.Events[0].Tool,
		"newest event (written last) must come first")

	for i := range decoded.Events {
		assert.NotEqual(t, audit.CapabilityMeta, decoded.Events[i].ToolCapability,
			"meta events must be excluded without include_meta")
	}
}

// TestLinodeAuditRecentInvalidSince verifies a malformed timestamp
// surfaces as an error result rather than being silently ignored.
func TestLinodeAuditRecentInvalidSince(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySince: "not-a-timestamp"}))
	require.NoError(t, err, "handler returns the error in the result, not as a Go error")
	require.NotNil(t, result, "result must not be nil")
	assert.True(t, result.IsError, "a malformed since must produce an error result")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, "since", "error should name the bad parameter")
}

// auditEvent builds an event at second `seq` of a fixed minute, so a
// caller passing increasing seq values gets events whose timestamps
// match their write order.
func auditEvent(tool string, capability audit.Capability, status audit.Status, seq int) audit.Event {
	ts := time.Date(2026, time.May, 20, 0, 0, seq, 0, time.UTC)

	return audit.Event{
		TS:             ts,
		TSUnixNS:       ts.UnixNano(),
		EventID:        "evt_" + tool,
		Tool:           tool,
		ToolCapability: capability,
		Status:         status,
	}
}

// writeAuditLog writes events as one JSON line each, in slice order.
func writeAuditLog(t *testing.T, path string, events []audit.Event) {
	t.Helper()

	file, err := os.Create(path) //nolint:gosec // path from test tmp dir
	require.NoError(t, err, "create %s", path)

	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	for i := range events {
		require.NoError(t, encoder.Encode(&events[i]), "encode event %d", i)
	}
}

// decodeAuditResult extracts and JSON-decodes the tool's text result.
func decodeAuditResult(t *testing.T, result *mcp.CallToolResult) auditRecentResult {
	t.Helper()

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")

	var decoded auditRecentResult

	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &decoded), "response must be valid JSON")

	return decoded
}

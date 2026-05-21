package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// healthResult mirrors the subset of the linode_audit_health JSON
// response the test asserts on.
type healthResult struct {
	JSONLPath        string `json:"jsonl_path"`
	ActiveLogExists  bool   `json:"active_log_exists"`
	RotatedFileCount int    `json:"rotated_file_count"`
	DroppedEvents    int64  `json:"dropped_events"`
}

// TestLinodeAuditHealthDefinition pins the tool identity.
func TestLinodeAuditHealthDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAuditHealthTool(&config.Config{})

	assert.Equal(t, "linode_audit_health", tool.Name)
	assert.Equal(t, profiles.CapMeta, capability, "health is CapMeta so every profile can read it")
	require.NotNil(t, handler)
	assert.NotContains(t, tool.InputSchema.Properties, "confirm", "a read-only query must not declare confirm")
}

// TestLinodeAuditHealthReportsJSONL drives the handler against a temp
// JSONL log (SQLite disabled) and confirms the report reflects the
// active log.
func TestLinodeAuditHealthReportsJSONL(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	require.NoError(t, os.MkdirAll(auditDir, 0o750))

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	_, _, handler := tools.NewLinodeAuditHealthTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	var decoded healthResult

	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &decoded))
	assert.Equal(t, filepath.Join(auditDir, "audit.log"), decoded.JSONLPath)
	assert.True(t, decoded.ActiveLogExists, "active log must be reported")
	assert.Zero(t, decoded.RotatedFileCount, "no rotated files written")
	assert.Zero(t, decoded.DroppedEvents)
}

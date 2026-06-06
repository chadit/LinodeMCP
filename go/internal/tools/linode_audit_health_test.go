package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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

	checkEqual(t, "linode_audit_health", tool.Name)
	checkEqual(t, profiles.CapMeta, capability, "health is CapMeta so every profile can read it")
	requireNotNil(t, handler)
	checkNoConfirm(t, tool.InputSchema.Properties, "a read-only query must not declare confirm")
}

// TestLinodeAuditHealthReportsJSONL drives the handler against a temp
// JSONL log (SQLite disabled) and confirms the report reflects the
// active log.
func TestLinodeAuditHealthReportsJSONL(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	requireNoError(t, os.MkdirAll(auditDir, 0o750))

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	_, _, handler := tools.NewLinodeAuditHealthTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	requireNoError(t, err)
	requireNotNil(t, result)
	checkFalse(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok)

	var decoded healthResult

	requireNoError(t, json.Unmarshal([]byte(textContent.Text), &decoded))
	checkEqual(t, filepath.Join(auditDir, "audit.log"), decoded.JSONLPath)
	checkTrue(t, decoded.ActiveLogExists, "active log must be reported")
	checkZero(t, decoded.RotatedFileCount, "no rotated files written")
	checkZero(t, decoded.DroppedEvents)
}

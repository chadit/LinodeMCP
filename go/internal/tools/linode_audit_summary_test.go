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

// summaryResult mirrors the linode_audit_summary JSON response.
type summaryResult struct {
	TotalEvents int `json:"total_events"`
	Rows        []struct {
		Groups map[string]string `json:"groups"`
		Count  int               `json:"count"`
	} `json:"rows"`
}

// TestLinodeAuditSummaryDefinition pins the tool identity and schema.
func TestLinodeAuditSummaryDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAuditSummaryTool(&config.Config{})

	checkEqual(t, "linode_audit_summary", tool.Name)
	checkEqual(t, profiles.CapMeta, capability, "summary is CapMeta so every profile can read it")
	requireNotNil(t, handler)

	props := tool.InputSchema.Properties
	for _, param := range []string{keySince, "group_by", "include_meta"} {
		checkContains(t, props, param, "schema should declare %q", param)
	}
}

// TestLinodeAuditSummaryCountsByToolStatus drives the handler against
// a temp JSONL log (SQLite disabled), confirming the default grouping
// and the meta exclusion.
func TestLinodeAuditSummaryCountsByToolStatus(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	requireNoError(t, os.MkdirAll(auditDir, 0o750))

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 2),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 3),
		auditEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, 4),
	})

	_, _, handler := tools.NewLinodeAuditSummaryTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	requireNoError(t, err)
	requireNotNil(t, result)
	checkFalse(t, result.IsError)

	decoded := decodeSummaryResult(t, result)
	checkEqual(t, 3, decoded.TotalEvents, "meta event excluded by default")
	requireLen(t, decoded.Rows, 2, "two tool+status buckets among non-meta events")
	checkEqual(t, "linode_instance_list", decoded.Rows[0].Groups["tool"], "highest count first")
	checkEqual(t, 2, decoded.Rows[0].Count)
}

// TestLinodeAuditSummaryInvalidGroupBy verifies an unknown group_by
// column surfaces as an error result.
func TestLinodeAuditSummaryInvalidGroupBy(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditSummaryTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		"group_by": []any{"bogus"},
	}))
	requireNoError(t, err)
	requireNotNil(t, result)
	checkTrue(t, result.IsError, "unknown group_by column must produce an error result")

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok)
	checkContains(t, textContent.Text, "group_by")
}

// decodeSummaryResult extracts and decodes the tool's JSON response.
func decodeSummaryResult(t *testing.T, result *mcp.CallToolResult) summaryResult {
	t.Helper()

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok, "content should be TextContent")

	var decoded summaryResult

	requireNoError(t, json.Unmarshal([]byte(textContent.Text), &decoded))

	return decoded
}

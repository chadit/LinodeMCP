package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// auditGroupByRefused is a group_by column no audit store has.
const auditGroupByRefused = "bogus"

// summaryResult mirrors the linode_audit_summary JSON response.
type summaryResult struct {
	Rows []struct {
		Groups map[string]string `json:"groups"`
		Count  int               `json:"count"`
	} `json:"rows"`
	TotalEvents int `json:"total_events"`
}

// TestLinodeAuditSummaryCountsByToolStatus drives the handler against
// a temp JSONL log (SQLite disabled), confirming the default grouping
// and the meta exclusion.
func TestLinodeAuditSummaryCountsByToolStatus(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 2),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 3),
		auditEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, 4),
	})

	_, _, handler := gentools.NewLinodeAuditSummaryTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	decoded := decodeSummaryResult(t, result)
	if decoded.TotalEvents != 3 {
		t.Errorf("decoded.TotalEvents = %v, want %v", decoded.TotalEvents, 3)
	}

	if len(decoded.Rows) != 2 {
		t.Fatalf("len(decoded.Rows) = %d, want %d", len(decoded.Rows), 2)
	}

	if decoded.Rows[0].Groups["tool"] != canRunReadTool {
		t.Errorf("got %v, want %v", decoded.Rows[0].Groups["tool"], canRunReadTool)
	}

	if decoded.Rows[0].Count != 2 {
		t.Errorf("decoded.Rows[0].Count = %v, want %v", decoded.Rows[0].Count, 2)
	}
}

// TestLinodeAuditSummaryInvalidGroupBy verifies an unknown group_by
// column surfaces as an error result.
func TestLinodeAuditSummaryInvalidGroupBy(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeAuditSummaryTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		"group_by": []any{auditGroupByRefused},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "group_by") {
		t.Errorf("textContent.Text does not contain %v", "group_by")
	}
}

// decodeSummaryResult extracts and decodes the tool's JSON response.
func decodeSummaryResult(t *testing.T, result *mcp.CallToolResult) summaryResult {
	t.Helper()

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded summaryResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return decoded
}

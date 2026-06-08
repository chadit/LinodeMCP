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

	if tool.Name != "linode_audit_health" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_audit_health")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if _, ok := tool.InputSchema.Properties["confirm"]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", "confirm")
	}
}

// TestLinodeAuditHealthReportsJSONL drives the handler against a temp
// JSONL log (SQLite disabled) and confirms the report reflects the
// active log.
func TestLinodeAuditHealthReportsJSONL(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	_, _, handler := tools.NewLinodeAuditHealthTool(&config.Config{})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded healthResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.JSONLPath != filepath.Join(auditDir, "audit.log") {
		t.Errorf("decoded.JSONLPath = %v, want %v", decoded.JSONLPath, filepath.Join(auditDir, "audit.log"))
	}

	if !decoded.ActiveLogExists {
		t.Error("decoded.ActiveLogExists = false, want true")
	}

	if decoded.RotatedFileCount != 0 {
		t.Errorf("decoded.RotatedFileCount = %v, want zero", decoded.RotatedFileCount)
	}

	if decoded.DroppedEvents != 0 {
		t.Errorf("decoded.DroppedEvents = %v, want zero", decoded.DroppedEvents)
	}
}

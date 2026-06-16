package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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

	if tool.Name != "linode_audit_recent" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_audit_recent")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	for _, param := range []string{"limit", keySince, "until", "tool", "capability", "status", "include_meta"} {
		if _, ok := props[param]; !ok {
			t.Errorf("props missing key %v", param)
		}
	}

	if _, ok := props["confirm"]; ok {
		t.Errorf("props has unexpected key %v", "confirm")
	}
}

// TestLinodeAuditRecentReturnsEvents drives the handler end-to-end
// against a temp audit directory (pointed at via XDG_STATE_HOME). It
// confirms the response envelope, newest-first order, and the
// default meta exclusion.
func TestLinodeAuditRecentReturnsEvents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, 2),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 3),
	})

	_, _, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

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

	decoded := decodeAuditResult(t, result)
	if decoded.Count != 2 {
		t.Errorf("decoded.Count = %v, want %v", decoded.Count, 2)
	}

	if len(decoded.Events) != 2 {
		t.Fatalf("len(decoded.Events) = %d, want %d", len(decoded.Events), 2)
	}

	if decoded.Events[0].Tool != canRunDestroyTool {
		t.Errorf("decoded.Events[0].Tool = %v, want %v", decoded.Events[0].Tool, canRunDestroyTool)
	}

	for i := range decoded.Events {
		if decoded.Events[i].ToolCapability == audit.CapabilityMeta {
			t.Errorf("decoded.Events[i].ToolCapability = %v, do not want %v", decoded.Events[i].ToolCapability, audit.CapabilityMeta)
		}
	}
}

// TestLinodeAuditRecentInvalidSince verifies a malformed timestamp
// surfaces as an error result rather than being silently ignored.
func TestLinodeAuditRecentInvalidSince(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySince: "not-a-timestamp"}))
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

	if !strings.Contains(textContent.Text, "since") {
		t.Errorf("textContent.Text does not contain %v", "since")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	for i := range events {
		if err := encoder.Encode(&events[i]); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// decodeAuditResult extracts and JSON-decodes the tool's text result.
func decodeAuditResult(t *testing.T, result *mcp.CallToolResult) auditRecentResult {
	t.Helper()

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded auditRecentResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return decoded
}

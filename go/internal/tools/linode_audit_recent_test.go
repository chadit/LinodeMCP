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
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// auditRecentResult mirrors the tool's JSON response so the test can
// decode and assert on it. The canonical serializer emits int64 counters
// as JSON numbers, so the full audit.Event decodes the response directly.
type auditRecentResult struct {
	Events []audit.Event `json:"events"`
	Count  int           `json:"count"`
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

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, 2),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 3),
	})

	_, _, handler := gentools.NewLinodeAuditRecentTool(&config.Config{})

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
		if decoded.Events[i].ToolCapability == string(audit.CapabilityMeta) {
			t.Errorf("decoded.Events[i].ToolCapability = %v, do not want %v", decoded.Events[i].ToolCapability, audit.CapabilityMeta)
		}
	}
}

// TestLinodeAuditRecentInvalidSince verifies a malformed timestamp
// surfaces as an error result rather than being silently ignored.
func TestLinodeAuditRecentInvalidSince(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeAuditRecentTool(&config.Config{})

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
func auditEvent(tool string, capability audit.Capability, status audit.Status, seq int) *audit.Event {
	ts := time.Date(2026, time.May, 20, 0, 0, seq, 0, time.UTC)

	return &audit.Event{
		Ts:             audit.EventTimestamp(ts),
		TsUnixNs:       ts.UnixNano(),
		EventId:        "evt_" + tool,
		Tool:           tool,
		ToolCapability: string(capability),
		Status:         string(status),
	}
}

// writeAuditLog writes events as one record per line, in slice order.
//
// It goes through the audit package's own NDJSON encoder rather than through a
// second spelling, so a fixture log carries the format the sink writes.
func writeAuditLog(t *testing.T, path string, events []*audit.Event) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = file.Close() }()

	if err := audit.EncodeEvents(file, events, audit.ExportFormatNDJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
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

// TestLinodeAuditRecentHonorsAValidSince covers the reader's other answer: a
// bound it can read narrows the window rather than refusing it. The refusal
// beside it is pinned above, so this is the half that proves the parse lands.
func TestLinodeAuditRecentHonorsAValidSince(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent(canRunDestroyTool, audit.CapabilityDestroy, audit.StatusError, 3),
	})

	_, _, handler := gentools.NewLinodeAuditRecentTool(&config.Config{})

	since := auditEvent("", audit.CapabilityRead, audit.StatusSuccess, 3).Ts

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySince: since}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, want false (%s)", behaviorText(t, result))
	}

	decoded := decodeAuditResult(t, result)
	if decoded.Count != 1 {
		t.Fatalf("decoded.Count = %d, want %d", decoded.Count, 1)
	}

	if decoded.Events[0].Tool != canRunDestroyTool {
		t.Errorf("decoded.Events[0].Tool = %v, want %v", decoded.Events[0].Tool, canRunDestroyTool)
	}
}

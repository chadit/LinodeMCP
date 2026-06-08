package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// reportResult mirrors the subset of the linode_audit_report JSON
// response the tests assert on.
type reportResult struct {
	Name        string             `json:"name"`
	Output      string             `json:"output"`
	TotalEvents int                `json:"total_events"`
	Rows        []audit.SummaryRow `json:"rows"`
	Events      []audit.Event      `json:"events"`
}

// TestLinodeAuditReportDefinition pins the tool identity.
func TestLinodeAuditReportDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAuditReportTool(&config.Config{})

	if tool.Name != "linode_audit_report" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_audit_report")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if _, ok := tool.InputSchema.Properties["name"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "name")
	}

	if !slices.Contains(tool.InputSchema.Required, "name") {
		t.Errorf("tool.InputSchema.Required does not contain %v", "name")
	}
}

// TestLinodeAuditReportUnknownName returns an error result rather than
// running an empty report when the name doesn't match a config entry.
func TestLinodeAuditReportUnknownName(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditReportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: "does-not-exist"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// TestLinodeAuditReportSummary runs a list-of-destroys report against
// a temp JSONL log and verifies the per-tool counts. Exercises the
// capability_in post-filter and the default group_by behavior
// (empty group_by → {tool, status}).
func TestLinodeAuditReportSummary(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusSuccess, 1),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusSuccess, 2),
		auditEvent("linode_volume_delete", audit.CapabilityDestroy, audit.StatusSuccess, 3),
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 4),
	})

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Reports: map[string]config.ReportConfig{
				"destroys": {
					Filter: config.ReportFilter{CapabilityIn: []string{"destroy"}},
					Output: config.ReportOutputSummary,
				},
			},
		},
	}

	_, _, handler := tools.NewLinodeAuditReportTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: "destroys"}))
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

	var decoded reportResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.Name != tcDestroys {
		t.Errorf("decoded.Name = %v, want %v", decoded.Name, tcDestroys)
	}

	if decoded.Output != config.ReportOutputSummary {
		t.Errorf("decoded.Output = %v, want %v", decoded.Output, config.ReportOutputSummary)
	}

	if decoded.TotalEvents != 3 {
		t.Errorf("decoded.TotalEvents = %v, want %v", decoded.TotalEvents, 3)
	}

	if len(decoded.Rows) != 2 {
		t.Fatalf("len(decoded.Rows) = %d, want %d", len(decoded.Rows), 2)
	}

	if decoded.Rows[0].Count != 2 {
		t.Errorf("decoded.Rows[0].Count = %v, want %v", decoded.Rows[0].Count, 2)
	}
}

// TestLinodeAuditReportListLimit returns matching events as a list and
// caps the result at the report's limit.
func TestLinodeAuditReportListLimit(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 2),
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 3),
	})

	cfg := &config.Config{
		Audit: config.AuditConfig{
			Reports: map[string]config.ReportConfig{
				"recent-reads": {
					Filter: config.ReportFilter{Capability: string(audit.CapabilityRead)},
					Output: config.ReportOutputList,
					Limit:  2,
				},
			},
		},
	}

	_, _, handler := tools.NewLinodeAuditReportTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: "recent-reads"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded reportResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.Output != config.ReportOutputList {
		t.Errorf("decoded.Output = %v, want %v", decoded.Output, config.ReportOutputList)
	}

	if decoded.TotalEvents != 2 {
		t.Errorf("decoded.TotalEvents = %v, want %v", decoded.TotalEvents, 2)
	}

	if len(decoded.Events) != 2 {
		t.Errorf("len(decoded.Events) = %d, want %d", len(decoded.Events), 2)
	}
}

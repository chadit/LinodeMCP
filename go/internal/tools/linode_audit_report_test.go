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

	checkEqual(t, "linode_audit_report", tool.Name)
	checkEqual(t, profiles.CapMeta, capability, "report is CapMeta so every profile can read it")
	requireNotNil(t, handler)
	checkContains(t, tool.InputSchema.Properties, "name")
	checkContains(t, tool.InputSchema.Required, "name")
}

// TestLinodeAuditReportUnknownName returns an error result rather than
// running an empty report when the name doesn't match a config entry.
func TestLinodeAuditReportUnknownName(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditReportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: "does-not-exist"}))
	requireNoError(t, err)
	requireNotNil(t, result)
	checkTrue(t, result.IsError)
}

// TestLinodeAuditReportSummary runs a list-of-destroys report against
// a temp JSONL log and verifies the per-tool counts. Exercises the
// capability_in post-filter and the default group_by behavior
// (empty group_by → {tool, status}).
func TestLinodeAuditReportSummary(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	requireNoError(t, os.MkdirAll(auditDir, 0o750))

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
	requireNoError(t, err)
	requireNotNil(t, result)
	checkFalse(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok)

	var decoded reportResult

	requireNoError(t, json.Unmarshal([]byte(textContent.Text), &decoded))
	checkEqual(t, "destroys", decoded.Name)
	checkEqual(t, config.ReportOutputSummary, decoded.Output)
	checkEqual(t, 3, decoded.TotalEvents, "three destroy events match, the read event is excluded")
	requireLen(t, decoded.Rows, 2, "instance_delete and volume_delete buckets")
	checkEqual(t, 2, decoded.Rows[0].Count, "instance_delete is the higher bucket")
}

// TestLinodeAuditReportListLimit returns matching events as a list and
// caps the result at the report's limit.
func TestLinodeAuditReportListLimit(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	requireNoError(t, os.MkdirAll(auditDir, 0o750))

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
	requireNoError(t, err)

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok)

	var decoded reportResult

	requireNoError(t, json.Unmarshal([]byte(textContent.Text), &decoded))
	checkEqual(t, config.ReportOutputList, decoded.Output)
	checkEqual(t, 2, decoded.TotalEvents, "capped at the report's limit")
	checkLen(t, decoded.Events, 2)
}

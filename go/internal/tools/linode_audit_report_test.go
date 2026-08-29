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

// reportResult mirrors the subset of the linode_audit_report JSON
// response the tests assert on. The canonical serializer emits int64
// counters as JSON numbers, so the full audit.Event decodes the events;
// the buckets are named here the way the summary tool's own test names them,
// because the generated answer carries no wire tags for a decode to read.
type reportResult struct {
	Name string `json:"name"`
	Rows []struct {
		Groups map[string]string `json:"groups"`
		Count  int               `json:"count"`
	} `json:"rows"`
	Output      string        `json:"output"`
	Events      []audit.Event `json:"events"`
	TotalEvents int           `json:"total_events"`
}

// TestLinodeAuditReportUnknownName returns an error result rather than
// running an empty report when the name doesn't match a config entry.
func TestLinodeAuditReportUnknownName(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeAuditReportTool(&config.Config{})

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

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
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

	_, _, handler := gentools.NewLinodeAuditReportTool(cfg)

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

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
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

	_, _, handler := gentools.NewLinodeAuditReportTool(cfg)

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

// TestLinodeAuditReportDefinitions covers the definition members the two happy
// paths do not reach: the bounds the load query parses, the columns the summary
// validates, and the lists and globs the post-filter reads.
//
// Each row is a whole report definition, because a report is only ever run as
// one, and the two refusals are the same tool result a caller would read.
func TestLinodeAuditReportDefinitions(t *testing.T) {
	for name, testCase := range map[string]struct {
		want   string
		report config.ReportConfig
		total  int
		refuse bool
	}{
		"an unknown group_by column refuses": {
			report: config.ReportConfig{
				Filter:  config.ReportFilter{},
				GroupBy: []string{"not-a-column"},
				Output:  config.ReportOutputSummary,
			},
			refuse: true,
			want:   "failed to run report: validate report group_by:",
		},
		"a since_offset that is not a duration refuses": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{SinceOffset: "two hours"},
				Output: config.ReportOutputList,
			},
			refuse: true,
			want:   "failed to run report: parse since_offset",
		},
		"a since that is not a timestamp refuses": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{Since: "yesterday"},
				Output: config.ReportOutputList,
			},
			refuse: true,
			want:   "failed to run report: parse since",
		},
		"an until that is not a timestamp refuses": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{Until: "tomorrow"},
				Output: config.ReportOutputList,
			},
			refuse: true,
			want:   "failed to run report: parse until",
		},
		"absolute bounds keep the window they name": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{
					Since: "2026-05-20T00:00:00Z",
					Until: "2026-05-20T00:00:01Z",
				},
				Output: config.ReportOutputList,
			},
			total: 1,
		},
		"a status list drops what it leaves out": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{StatusIn: []string{string(audit.StatusError)}},
				Output: config.ReportOutputList,
			},
			total: 1,
		},
		"an environment glob drops what it does not match": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{Environment: "prod-*"},
				Output: config.ReportOutputList,
			},
			total: 1,
		},
		"a profile glob matching neither drops both": {
			report: config.ReportConfig{
				Filter: config.ReportFilter{Profile: "nobody-*"},
				Output: config.ReportOutputList,
			},
			total: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", reportStateHome(t))

			cfg := &config.Config{Audit: config.AuditConfig{
				Reports: map[string]config.ReportConfig{tcDestroys: testCase.report},
			}}

			_, _, handler := gentools.NewLinodeAuditReportTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: tcDestroys}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if testCase.refuse {
				if !result.IsError {
					t.Fatalf("result.IsError = false, want true: %s", textContent.Text)
				}

				if !strings.HasPrefix(textContent.Text, testCase.want) {
					t.Errorf("refusal = %q, want the prefix %q", textContent.Text, testCase.want)
				}

				return
			}

			if result.IsError {
				t.Fatalf("report refused: %s", textContent.Text)
			}

			var decoded reportResult

			if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if decoded.TotalEvents != testCase.total {
				t.Errorf("decoded.TotalEvents = %d, want %d", decoded.TotalEvents, testCase.total)
			}
		})
	}
}

// reportStateHome stages the two events the definition cases read: one write in
// a production environment and one failed destroy in staging, which is what
// lets a row tell each filter member apart from the others.
func reportStateHome(t *testing.T) string {
	t.Helper()

	stateHome := t.TempDir()

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	boot := auditEvent("linode_instance_boot", audit.CapabilityWrite, audit.StatusSuccess, 1)
	boot.Environment = "prod-east"
	boot.Profile = profileOperator

	failed := auditEvent("linode_volume_delete", audit.CapabilityDestroy, audit.StatusError, 2)
	failed.Environment = tcStaging
	failed.Profile = "operator"

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{boot, failed})

	return stateHome
}

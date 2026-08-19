package tools_test

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The audit filters more than one of the five tools advertise.
const (
	keyIncludeMeta = "include_meta"
	keyFormat      = "format"
)

// auditToolFactory is the shape every generated audit factory has.
type auditToolFactory func(*config.Config) (mcp.Tool, profiles.Capability, tools.Handler)

// TestGeneratedAuditToolDefinitions pins each audit tool's identity and the
// parameters its schema advertises. The answers themselves are the hooks', and
// go/internal/toolhooks tests those; what is pinned here is what the contract
// puts in front of a client.
func TestGeneratedAuditToolDefinitions(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		factory auditToolFactory
		params  []string
	}{
		{
			name:    "linode_audit_health",
			factory: gentools.NewLinodeAuditHealthTool,
		},
		{
			name:    "linode_audit_report",
			factory: gentools.NewLinodeAuditReportTool,
			params:  []string{"name"},
		},
		{
			name:    "linode_audit_recent",
			factory: gentools.NewLinodeAuditRecentTool,
			params: []string{
				"limit", keySince, "until", canRunKeyTool, "capability", "status", keyIncludeMeta,
			},
		},
		{
			name:    "linode_audit_export",
			factory: gentools.NewLinodeAuditExportTool,
			params:  []string{keyFormat, keySince, "until", canRunKeyTool, "max_records", keyIncludeMeta},
		},
		{
			name:    "linode_audit_summary",
			factory: gentools.NewLinodeAuditSummaryTool,
			params:  []string{keySince, "group_by", keyIncludeMeta},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tool, capability, handler := testCase.factory(&config.Config{})

			if tool.Name != testCase.name {
				t.Errorf("tool.Name = %v, want %v", tool.Name, testCase.name)
			}

			if capability != profiles.CapMeta {
				t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
			}

			if handler == nil {
				t.Fatal("handler is nil")
			}

			raw := string(tool.RawInputSchema)
			for _, param := range testCase.params {
				if !strings.Contains(raw, param) {
					t.Errorf("tool.RawInputSchema missing key %v", param)
				}
			}

			// No audit tool gates anything: the meta tier advertises no
			// confirm, and none of the five may advertise dry_run either.
			for _, unwanted := range []string{"confirm", keyDryRun} {
				if strings.Contains(raw, unwanted) {
					t.Errorf("tool.RawInputSchema has unexpected key %v", unwanted)
				}
			}
		})
	}
}

// TestGeneratedAuditToolsAnswerTheContractsRules pins the two sentences the
// audit surface's own rules answer with, and that both reach a caller as a
// tool-result error the way every other refusal does. They moved from hand
// checks into the contract with the tools, so nothing else would catch a
// change of wording: the meta surface carries no behavior fixtures.
func TestGeneratedAuditToolsAnswerTheContractsRules(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		factory auditToolFactory
		args    map[string]any
		want    string
	}{
		{
			name:    "report name is required",
			factory: gentools.NewLinodeAuditReportTool,
			args:    map[string]any{},
			want:    "report name is required",
		},
		{
			name:    "export format is required",
			factory: gentools.NewLinodeAuditExportTool,
			args:    map[string]any{},
			want:    "format must be one of: json, csv, ndjson",
		},
		{
			name:    "export format is one the encoder knows",
			factory: gentools.NewLinodeAuditExportTool,
			args:    map[string]any{keyFormat: "xml"},
			want:    "format must be one of: json, csv, ndjson",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := testCase.factory(&config.Config{})

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatal("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if textContent.Text != testCase.want {
				t.Errorf("textContent.Text = %v, want %v", textContent.Text, testCase.want)
			}
		})
	}
}

// TestGeneratedVersionToolAnswersRatherThanErroring pins version's one
// behavior: VersionInput declares no rules and its hook reads no argument, so
// nothing a caller sends can reach the error result the tier now carries.
func TestGeneratedVersionToolAnswersRatherThanErroring(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewVersionTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"unexpected": "argument"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, appinfo.Version) {
		t.Errorf("textContent.Text does not carry %v", appinfo.Version)
	}
}

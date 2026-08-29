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

// storeWarningMark is the phrase every fallback warning ends with, whichever
// tool answered it. Both languages spell it this way.
const storeWarningMark = "answered from the JSONL log instead"

// fallbackResult is the part of an audit answer this file reads: the counts a
// tool reports plus the warnings it carries.
type fallbackResult struct {
	Warnings    []string `json:"warnings"`
	Count       int      `json:"count"`
	TotalEvents int      `json:"total_events"`
	RecordCount int      `json:"record_count"`
}

// unopenableStoreConfig points the SQLite sink at a file that is not a
// database, with two events in the JSONL log beside it. That is the shape
// behind D-7: the store the config names cannot answer, and the durable log
// can. The caller owns the XDG_STATE_HOME override, since a test that sets an
// environment variable is the one that cannot run in parallel.
func unopenableStoreConfig(t *testing.T, stateHome string) *config.Config {
	t.Helper()

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 2),
	})

	dbPath := filepath.Join(auditDir, "audit.db")
	if err := os.WriteFile(dbPath, []byte("not a database"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = dbPath

	return cfg
}

// decodeFallbackResult reads a successful audit answer's counts and warnings.
func decodeFallbackResult(t *testing.T, result *mcp.CallToolResult) fallbackResult {
	t.Helper()

	if result.IsError {
		t.Fatalf("result.IsError = true, want false (%s)", behaviorText(t, result))
	}

	var decoded fallbackResult
	if err := json.Unmarshal([]byte(behaviorText(t, result)), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return decoded
}

// behaviorText reads the one text content off a tool result.
func behaviorText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("len(result.Content) = 0, want 1")
	}

	content, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	return content.Text
}

// assertNamesTheStore checks that the one warning names the configured path
// and says which source answered instead.
func assertNamesTheStore(t *testing.T, warnings []string, dbPath string) {
	t.Helper()

	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1 (%v)", len(warnings), warnings)
	}

	if !strings.Contains(warnings[0], dbPath) {
		t.Errorf("warning %q does not name the store %q", warnings[0], dbPath)
	}

	if !strings.Contains(warnings[0], storeWarningMark) {
		t.Errorf("warning %q does not say which source answered", warnings[0])
	}
}

// TestAuditSummaryFallsBackToJSONL proves a store that will not open leaves
// the summary answering from the durable log, with the swap stated.
func TestAuditSummaryFallsBackToJSONL(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := unopenableStoreConfig(t, stateHome)

	_, _, handler := gentools.NewLinodeAuditSummaryTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded := decodeFallbackResult(t, result)
	if decoded.TotalEvents != 2 {
		t.Errorf("decoded.TotalEvents = %d, want %d", decoded.TotalEvents, 2)
	}

	assertNamesTheStore(t, decoded.Warnings, cfg.Audit.SQLite.Path)
}

// TestAuditExportFallsBackToJSONL proves the same for the export tool.
func TestAuditExportFallsBackToJSONL(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := unopenableStoreConfig(t, stateHome)

	_, _, handler := gentools.NewLinodeAuditExportTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		"format": "json",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded := decodeFallbackResult(t, result)
	if decoded.RecordCount != 2 {
		t.Errorf("decoded.RecordCount = %d, want %d", decoded.RecordCount, 2)
	}

	assertNamesTheStore(t, decoded.Warnings, cfg.Audit.SQLite.Path)
}

// TestAuditHealthFallsBackToJSONL proves the health report keeps its JSONL
// half rather than answering a failure for an unreadable database.
func TestAuditHealthFallsBackToJSONL(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := unopenableStoreConfig(t, stateHome)

	_, _, handler := gentools.NewLinodeAuditHealthTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded := decodeFallbackResult(t, result)

	assertNamesTheStore(t, decoded.Warnings, cfg.Audit.SQLite.Path)
}

// TestAuditQueriesCarryNoWarningWhenTheStoreIsOff proves the warning is not a
// permanent fixture of the answer: with no SQLite configured there is no swap
// to report.
func TestAuditQueriesCarryNoWarningWhenTheStoreIsOff(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	_, _, handler := gentools.NewLinodeAuditSummaryTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded := decodeFallbackResult(t, result)
	if len(decoded.Warnings) != 0 {
		t.Errorf("len(decoded.Warnings) = %d, want 0 (%v)", len(decoded.Warnings), decoded.Warnings)
	}
}

// TestAuditQueryAnswersWhenNeitherStoreReads proves the fallback is not a way
// to swallow a real failure: with the JSONL log unreadable too, the tool
// answers the read failure rather than an empty window.
func TestAuditQueryAnswersWhenNeitherStoreReads(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode-000 directory, so nothing fails")
	}

	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := unopenableStoreConfig(t, stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.Chmod(auditDir, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(auditDir, 0o700) })

	_, _, handler := gentools.NewLinodeAuditSummaryTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true (%s)", behaviorText(t, result))
	}

	if got := behaviorText(t, result); !strings.HasPrefix(got, "failed to read audit log: ") {
		t.Errorf("refusal = %q, want the read-failure sentence", got)
	}
}

// TestAuditReportAnswersWhenNeitherStoreReads proves the report tool answers
// its own declared read-failure sentence rather than the query tools' one.
//
// The sentence is declared on AuditReportInput, so its twin in the other
// language is the same case: one contract edit turns both red.
func TestAuditReportAnswersWhenNeitherStoreReads(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode-000 directory, so nothing fails")
	}

	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := unopenableStoreConfig(t, stateHome)
	cfg.Audit.Reports = map[string]config.ReportConfig{
		tcDestroys: {
			Filter: config.ReportFilter{},
			Output: config.ReportOutputList,
		},
	}

	if err := os.Chmod(filepath.Join(stateHome, "linodemcp"), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(filepath.Join(stateHome, "linodemcp"), 0o700) })

	_, _, handler := gentools.NewLinodeAuditReportTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: tcDestroys}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true (%s)", behaviorText(t, result))
	}

	want := "failed to run report: load report events: "
	if got := behaviorText(t, result); !strings.HasPrefix(got, want) {
		t.Errorf("refusal = %q, want the declared read-failure sentence", got)
	}
}

// TestAuditStorePathDefaultsBesideTheLog proves an enabled sink with no
// configured path reads audit.db beside the JSONL log, which is where the sink
// writes it.
func TestAuditStorePathDefaultsBesideTheLog(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dbPath := filepath.Join(auditDir, "audit.db")
	writeSQLiteAuditEvents(t, dbPath)

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true

	_, _, handler := gentools.NewLinodeAuditHealthTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := behaviorText(t, result); !strings.Contains(got, dbPath) {
		t.Errorf("answer does not name the derived store %q: %s", dbPath, got)
	}
}

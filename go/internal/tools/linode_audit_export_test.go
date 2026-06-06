package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// exportResult mirrors the linode_audit_export JSON response.
type exportResult struct {
	Path        string `json:"path"`
	Format      string `json:"format"`
	RecordCount int    `json:"record_count"`
}

// TestLinodeAuditExportDefinition pins the tool identity and that
// format is a required parameter.
func TestLinodeAuditExportDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAuditExportTool(&config.Config{})

	checkEqual(t, "linode_audit_export", tool.Name)
	checkEqual(t, profiles.CapMeta, capability, "export is CapMeta so every profile can read it")
	requireNotNil(t, handler)
	checkContains(t, tool.InputSchema.Properties, "format")
	checkContains(t, tool.InputSchema.Required, "format", "format must be required")
}

// TestLinodeAuditExportWritesNDJSON drives the handler against a temp
// JSONL log (SQLite disabled), exporting NDJSON, and confirms the
// response points at a file containing one line per exported event.
func TestLinodeAuditExportWritesNDJSON(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	requireNoError(t, os.MkdirAll(auditDir, 0o750))

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_volume_list", audit.CapabilityRead, audit.StatusSuccess, 2),
	})

	_, _, handler := tools.NewLinodeAuditExportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"format": "ndjson"}))
	requireNoError(t, err)
	requireNotNil(t, result)
	checkFalse(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok)

	var decoded exportResult

	requireNoError(t, json.Unmarshal([]byte(textContent.Text), &decoded))
	checkEqual(t, "ndjson", decoded.Format)
	checkEqual(t, 2, decoded.RecordCount)

	t.Cleanup(func() { requireNoError(t, os.Remove(decoded.Path), "remove exported audit file") })

	body, err := os.ReadFile(decoded.Path)
	requireNoError(t, err)

	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	checkLen(t, lines, 2, "one NDJSON line per event")
}

// TestLinodeAuditExportUnknownFormat returns an error result rather
// than writing a file for an unsupported format.
func TestLinodeAuditExportUnknownFormat(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditExportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"format": "xml"}))
	requireNoError(t, err)
	requireNotNil(t, result)
	checkTrue(t, result.IsError, "unknown format is an error result")
}

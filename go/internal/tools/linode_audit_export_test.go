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
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// exportResult mirrors the linode_audit_export JSON response.
type exportResult struct {
	Path        string `json:"path"`
	Format      string `json:"format"`
	RecordCount int    `json:"record_count"`
}

// TestLinodeAuditExportWritesNDJSON drives the handler against a temp
// JSONL log (SQLite disabled), exporting NDJSON, and confirms the
// response points at a file containing one line per exported event.
func TestLinodeAuditExportWritesNDJSON(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_volume_list", audit.CapabilityRead, audit.StatusSuccess, 2),
	})

	_, _, handler := gentools.NewLinodeAuditExportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFormat: "ndjson"}))
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

	var decoded exportResult

	if unmarshalErr := json.Unmarshal([]byte(textContent.Text), &decoded); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if decoded.Format != "ndjson" {
		t.Errorf("decoded.Format = %v, want %v", decoded.Format, "ndjson")
	}

	if decoded.RecordCount != 2 {
		t.Errorf("decoded.RecordCount = %v, want %v", decoded.RecordCount, 2)
	}

	t.Cleanup(func() { _ = os.Remove(decoded.Path) })

	body, err := os.ReadFile(decoded.Path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("len(lines) = %d, want %d", len(lines), 2)
	}
}

// TestAuditExportReportsAFormatTheEncoderRefuses covers the branch the
// contract's own rule normally stands in front of: the operation is reachable
// from a caller that never ran that rule, and a format the encoder does not
// know has to be reported rather than leaving a stray file behind.
func TestAuditExportReportsAFormatTheEncoderRefuses(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	// Through the emitted arm rather than the subsystem alone: what a caller
	// reads is the cause the arm carries into the tool's own sentence.
	outcome := gentools.RunAuditExport(t.Context(), &config.Config{}, tools.AuditExport,
		"xml", time.Time{}, time.Time{}, "", 0, false)

	if outcome.Refusal != tools.LocalRefusalWriteFailed {
		t.Fatalf("outcome.Refusal = %v, want %v", outcome.Refusal, tools.LocalRefusalWriteFailed)
	}

	if !strings.Contains(outcome.Cause, "xml") {
		t.Errorf("outcome.Cause does not name the format:\n%s", outcome.Cause)
	}

	left, err := filepath.Glob(filepath.Join(tempDir, "linode-audit-export-*"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(left) != 0 {
		t.Errorf("export left %v behind, want the half-written file removed", left)
	}
}

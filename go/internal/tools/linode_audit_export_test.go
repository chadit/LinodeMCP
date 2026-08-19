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

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
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

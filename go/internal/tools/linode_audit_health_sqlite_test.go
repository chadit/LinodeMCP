package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// sqliteHealthResult mirrors the SQLite section of the linode_audit_health
// response. The section is a proto message with explicit presence, so it is
// absent from the JSON entirely when the sink is off; a pointer lets the test
// tell "absent" from "present and zeroed".
type sqliteHealthResult struct {
	SQLite *struct {
		Path              string `json:"path"`
		EventCount        int64  `json:"event_count"`
		OldestEventUnixNS int64  `json:"oldest_event_unix_ns"`
		DBBytes           int64  `json:"db_bytes"`
	} `json:"sqlite"`
}

// TestLinodeAuditHealthReportsSQLiteSection drives the health tool with the
// SQLite sink enabled and asserts the response carries the sink's row count,
// oldest event stamp, and file size. Without the sink the section is omitted,
// so this is the only path that fills it.
func TestLinodeAuditHealthReportsSQLiteSection(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dbPath := filepath.Join(auditDir, "audit.db")
	oldest := writeSQLiteAuditEvents(t, dbPath)

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = dbPath

	_, _, handler := tools.NewLinodeAuditHealthTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
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

	var decoded sqliteHealthResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.SQLite == nil {
		t.Fatal("decoded.SQLite is nil, want the SQLite section")
	}

	if decoded.SQLite.Path != dbPath {
		t.Errorf("decoded.SQLite.Path = %v, want %v", decoded.SQLite.Path, dbPath)
	}

	if decoded.SQLite.EventCount != int64(2) {
		t.Errorf("decoded.SQLite.EventCount = %v, want %v", decoded.SQLite.EventCount, int64(2))
	}

	if decoded.SQLite.OldestEventUnixNS != oldest.UnixNano() {
		t.Errorf("decoded.SQLite.OldestEventUnixNS = %v, want %v",
			decoded.SQLite.OldestEventUnixNS, oldest.UnixNano())
	}

	if decoded.SQLite.DBBytes <= 0 {
		t.Errorf("decoded.SQLite.DBBytes = %v, want a positive value", decoded.SQLite.DBBytes)
	}
}

// TestLinodeAuditHealthOmitsSQLiteSectionWhenDisabled is the other half of the
// pair: with the sink off the response carries no SQLite section at all rather
// than a zeroed one, which is what makes the section meaningful.
func TestLinodeAuditHealthOmitsSQLiteSectionWhenDisabled(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	_, _, handler := tools.NewLinodeAuditHealthTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded sqliteHealthResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.SQLite != nil {
		t.Errorf("decoded.SQLite = %v, want nil", decoded.SQLite)
	}
}

// writeSQLiteAuditEvents seeds a SQLite audit database with two events and
// returns the timestamp of the older one.
func writeSQLiteAuditEvents(t *testing.T, dbPath string) time.Time {
	t.Helper()

	sink, err := audit.NewSQLiteSink(t.Context(), dbPath, config.DefaultAuditSQLiteBusyTimeoutMS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oldest := time.Date(2026, time.May, 18, 8, 0, 0, 0, time.UTC)

	for seq, ts := range []time.Time{oldest, oldest.Add(48 * time.Hour)} {
		event := auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, seq)
		event.EventID = "evt_sqlite_" + ts.Format("20060102")
		event.TS = ts
		event.TSUnixNS = ts.UnixNano()
		sink.Write(t.Context(), &event)
	}

	if closeErr := sink.Close(); closeErr != nil {
		t.Fatalf("unexpected error: %v", closeErr)
	}

	return oldest
}

package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// auditReadFailed is the sentence every audit answer reports for a store it
// cannot read.
const (
	auditReadFailed = "failed to read audit log"
	// auditFormatJSON is the export format the store-failure cases ask for,
	// since format has no default and every case has to name one.
	auditFormatJSON = "json"
)

// unreadableStateHome answers a state directory whose linodemcp entry is a
// file, so every read of the log under it fails with ENOTDIR. It is what
// drives the answers' store-failure branches without depending on permissions,
// which vary by platform and by who runs the test.
func unreadableStateHome(t *testing.T) string {
	t.Helper()

	stateHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateHome, "linodemcp"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return stateHome
}

// TestAuditAnswersReportAnUnreadableStore covers the branch every audit answer
// carries for a store it cannot read: the failure reaches the caller as a
// tool-result error naming what went wrong, rather than as a transport error
// the model cannot act on.
func TestAuditAnswersReportAnUnreadableStore(t *testing.T) {
	for _, testCase := range []struct {
		factory auditToolFactory
		args    map[string]any
		name    string
		want    string
	}{
		{
			factory: gentools.NewLinodeAuditRecentTool,
			name:    "recent",
			want:    auditReadFailed,
		},
		{
			factory: gentools.NewLinodeAuditSummaryTool,
			name:    "summary",
			want:    auditReadFailed,
		},
		{
			factory: gentools.NewLinodeAuditExportTool,
			name:    "export",
			want:    auditReadFailed,
			args:    map[string]any{keyFormat: auditFormatJSON},
		},
		{
			factory: gentools.NewLinodeAuditHealthTool,
			name:    "health",
			want:    "failed to collect audit health",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", unreadableStateHome(t))

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

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text = %v, want it to name %v", textContent.Text, testCase.want)
			}
		})
	}
}

// TestAuditReportReportsAnUnreadableStore is the report tool's half of the same
// branch. It is separate because reaching the load needs a report in the
// catalog, which the others do not have.
func TestAuditReportReportsAnUnreadableStore(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", unreadableStateHome(t))

	cfg := &config.Config{}
	cfg.Audit.Reports = map[string]config.ReportConfig{
		tcDestroys: {Output: "summary"},
	}

	_, _, handler := gentools.NewLinodeAuditReportTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyName: tcDestroys}))
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

	if !strings.Contains(textContent.Text, "failed to run report") {
		t.Errorf("textContent.Text = %v, want it to name a failed report", textContent.Text)
	}
}

// TestAuditExportReportsAMalformedBound covers the answer's other refusal: a
// since or until the query builder cannot parse never reaches the store.
func TestAuditExportReportsAMalformedBound(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeAuditExportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyFormat: auditFormatJSON,
		keySince:  "not-a-timestamp",
	}))
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

	if !strings.Contains(textContent.Text, keySince) {
		t.Errorf("textContent.Text = %v, want it to name the bad bound", textContent.Text)
	}
}

// TestAuditRecentStopsOnAClosedContext covers the cancellation check the recent
// answer carries for itself: audit.ReadRecent takes no context, so a call
// nobody is waiting for has to stop before the log walk.
func TestAuditRecentStopsOnAClosedContext(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeAuditRecentTool(&config.Config{})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := handler(ctx, createRequestWithArgs(t, nil)); err == nil {
		t.Error("err = nil, want a cancellation")
	}
}

// TestAuditExportReportsAFileItCannotWrite covers the answer's last branch: a
// temp file the writer cannot create is reported rather than raised, the way
// Python's answer reports it.
func TestAuditExportReportsAFileItCannotWrite(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "tmp")
	if err := os.WriteFile(notADir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Setenv("TMPDIR", notADir)

	_, _, handler := gentools.NewLinodeAuditExportTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFormat: auditFormatJSON}))
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

	if !strings.Contains(textContent.Text, "failed to write export file") {
		t.Errorf("textContent.Text = %v, want it to name the write", textContent.Text)
	}
}

// TestAuditRecentAnswerStopsOnAClosedContext drives the answer directly, since
// the generated shell's own cancellation check stands in front of it. The
// answer keeps its own because audit.ReadRecent takes no context, so any caller
// but the shell would otherwise walk the whole log after the caller left.
func TestAuditRecentAnswerStopsOnAClosedContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := createRequestWithArgs(t, nil)

	if _, err := tools.AuditRecentAnswer(ctx, &request, &config.Config{}); err == nil {
		t.Error("err = nil, want a cancellation")
	}
}

package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The report these tests run: a relative window, so its answer is measured back
// from whatever clock the call context carries.
const tcWindowReport = "window"

// seedClockReport writes four events at a known instant under stateHome and
// answers a config holding one report whose window is relative. The events all
// land within a minute of each other, so a two-minute window either takes all
// four or none. The caller owns the XDG_STATE_HOME redirect, since a test that
// sets an environment variable cannot also run in parallel.
func seedClockReport(t *testing.T, stateHome string) *config.Config {
	t.Helper()

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_boot", audit.CapabilityWrite, audit.StatusSuccess, 2),
		auditEvent("linode_volume_delete", audit.CapabilityDestroy, audit.StatusSuccess, 3),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 4),
	})

	return &config.Config{
		Audit: config.AuditConfig{
			Reports: map[string]config.ReportConfig{
				tcWindowReport: {
					Filter: config.ReportFilter{SinceOffset: "2m"},
					Output: config.ReportOutputSummary,
				},
			},
		},
	}
}

// runClockReport runs the relative-window report under ctx and answers how many
// events landed inside the window.
func runClockReport(ctx context.Context, t *testing.T, cfg *config.Config) int {
	t.Helper()

	_, _, handler := gentools.NewLinodeAuditReportTool(cfg)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{keyName: tcWindowReport}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded reportResult

	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return decoded.TotalEvents
}

// TestWithClockMovesTheRelativeWindow proves an attached clock is the instant a
// relative report window is measured back from: the same store and the same
// report answer differently under two clocks, one standing just after the
// events and one standing long after them.
func TestWithClockMovesTheRelativeWindow(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := seedClockReport(t, stateHome)

	justAfter := time.Date(2026, time.May, 20, 0, 1, 0, 0, time.UTC)
	longAfter := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)

	insideCtx := tools.WithClock(t.Context(), func() time.Time { return justAfter })

	inside := runClockReport(insideCtx, t, cfg)
	if inside != 4 {
		t.Errorf("events inside the window = %d, want 4", inside)
	}

	outsideCtx := tools.WithClock(t.Context(), func() time.Time { return longAfter })

	outside := runClockReport(outsideCtx, t, cfg)
	if outside != 0 {
		t.Errorf("events inside the window = %d, want 0", outside)
	}
}

// TestWithClockNilIsIgnored proves a nil clock leaves the call reading the wall
// clock: the answer under it matches the answer with nothing attached at all.
// Comparing the two answers rather than naming one keeps the assertion off the
// wall clock's actual value.
func TestWithClockNilIsIgnored(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := seedClockReport(t, stateHome)

	bare := runClockReport(t.Context(), t, cfg)

	withNil := runClockReport(tools.WithClock(t.Context(), nil), t, cfg)
	if withNil != bare {
		t.Errorf("events under a nil clock = %d, want %d (the answer with none attached)",
			withNil, bare)
	}
}

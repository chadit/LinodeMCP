package tools_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestDestroyBypassDryRunGate covers the Phase 3 bypass-dry-run enforcement
// shared by every CapDestroy tool (the gate lives in RunDestructiveAction).
// volume_delete is the representative tool; all destroy tools route through
// the same gate. The error paths short-circuit before any API call, so the
// no-call server never sees a request.
func TestDestroyBypassDryRunGate(t *testing.T) {
	t.Parallel()

	errText := func(t *testing.T, result *mcp.CallToolResult) string {
		t.Helper()

		if !result.IsError {
			t.Fatal("result.IsError = false, want true")
		}

		content, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		return content.Text
	}

	t.Run("confirm without a dry-run assertion is rejected", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeDeleteTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(789),
			keyConfirm:  true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		text := errText(t, result)
		if !strings.Contains(text, "is destructive") {
			t.Errorf("text does not contain %v", "is destructive")
		}

		if !strings.Contains(text, "confirmed_dry_run") {
			t.Errorf("text does not contain %v", "confirmed_dry_run")
		}

		if !strings.Contains(text, "confirm_bypass_dry_run") {
			t.Errorf("text does not contain %v", "confirm_bypass_dry_run")
		}
	})

	t.Run("bypass without confirm is rejected", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeDeleteTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID:            float64(789),
			keyConfirmBypassDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(errText(t, result), "only takes effect with confirm: true") {
			t.Errorf("errText(t, result) does not contain %v", "only takes effect with confirm: true")
		}
	})

	t.Run("both bypass and confirmed flags is rejected", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeDeleteTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID:            float64(789),
			keyConfirm:             true,
			keyConfirmedDryRun:     true,
			keyConfirmBypassDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(errText(t, result), "not both") {
			t.Errorf("errText(t, result) does not contain %v", "not both")
		}
	})

	// The happy paths (confirm + confirmed_dry_run, and confirm + bypass both
	// reach execution) are covered by every CapDestroy tool's existing
	// execution test, which now passes confirmed_dry_run: true.
}

// TestDestroyYoloBypass verifies that a permitted yolo execution (the server
// marks the context via WithYoloAllowed) bypasses both the dry-run gate AND
// the confirm requirement: the destroy executes with neither confirm nor a
// dry-run assertion present.
func TestDestroyYoloBypass(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes/789" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/789")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	// No confirm, no confirmed_dry_run; only the yolo-marked context.
	ctx := tools.WithYoloAllowed(t.Context())

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{keyVolumeID: float64(789)}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}
}

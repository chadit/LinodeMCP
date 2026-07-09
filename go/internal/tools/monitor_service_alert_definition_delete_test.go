package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	monitorServiceAlertDefinitionDeletePath     = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionDeleteToolName = "linode_monitor_service_alert_definition_delete"
)

func monitorAlertDefinitionDeleteArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam: monitorServiceToolTypeDatabase,
		monitorAlertIDParam:     20000,
		keyConfirm:              true,
		// The destroy gate requires a prior confirmed dry-run (or an
		// explicit bypass) on top of confirm, like every CapDestroy tool.
		keyConfirmedDryRun: true,
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)
	if tool.Name != monitorServiceAlertDefinitionDeleteToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceAlertDefinitionDeleteToolName)
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{monitorServiceTypeParam, monitorAlertIDParam, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != monitorServiceAlertDefinitionDeletePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionDeletePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionDeleteArgs())

	result, err := handler(t.Context(), req)
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

	for _, want := range []string{
		"Monitor service alert definition 20000 deleted for 'dbaas'",
		"\"service_type\"",
		"\"alert_id\"",
		"20000",
	} {
		if !strings.Contains(textContent.Text, want) {
			t.Errorf("textContent.Text does not contain %v", want)
		}
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != monitorServiceAlertDefinitionDeletePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionDeletePath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionDeleteArgs())

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to delete "+monitorServiceAlertDefinitionDeleteToolName) {
		t.Errorf("textContent.Text does not contain %v", "Failed to delete "+monitorServiceAlertDefinitionDeleteToolName)
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseFalseConfirmRejected, value: false, set: true},
		{name: caseStringConfirmRejected, value: boolStringTrue, set: true},
		{name: caseNumericConfirmRejected, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := monitorAlertDefinitionDeleteArgs()
			if !testCase.set {
				delete(args, keyConfirm)
			}

			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolInvalidArgumentsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceType, args: map[string]any{monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeRequiredError},
		{name: caseSeparatorServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeSlash, monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseQueryServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeQuery, monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseTraversalServiceType, args: map[string]any{monitorServiceTypeParam: pathTraversalValue, monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseMissingAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorAlertIDRequiredError},
		{name: caseZeroAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 0, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorAlertIDPositiveError},
		{name: caseStringAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: "20000", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorAlertIDPositiveError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolTransientErrorIsNotReplayed(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != monitorServiceAlertDefinitionDeletePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionDeletePath)
		}

		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionDeleteArgs())

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

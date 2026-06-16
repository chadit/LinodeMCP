package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	monitorServiceAlertDefinitionGetPath     = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionGetToolName = "linode_monitor_service_alert_definition_get"
	monitorAlertIDParam                      = "alert_id"
	monitorAlertIDRequiredError              = "alert_id is required"
	monitorAlertIDPositiveError              = "alert_id must be a positive integer"
)

func TestLinodeMonitorServiceAlertDefinitionGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)
	if tool.Name != monitorServiceAlertDefinitionGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceAlertDefinitionGetToolName)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{monitorServiceTypeParam, monitorAlertIDParam} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceAlertDefinitionGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          20000,
			keyLabel:       "High CPU Usage",
			keyServiceType: monitorServiceToolTypeDatabase,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 20000})

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

	if !strings.Contains(textContent.Text, "High CPU Usage") {
		t.Errorf("textContent.Text does not contain %v", "High CPU Usage")
	}

	if !strings.Contains(textContent.Text, monitorServiceToolTypeDatabase) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolTypeDatabase)
	}
}

func TestLinodeMonitorServiceAlertDefinitionGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 20000})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve "+monitorServiceAlertDefinitionGetToolName) {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve "+monitorServiceAlertDefinitionGetToolName)
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceAlertDefinitionGetToolInvalidArgumentsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceType, args: map[string]any{monitorAlertIDParam: 20000}, wantMessage: monitorServiceTypeRequiredError},
		{name: "separator service type", args: map[string]any{monitorServiceTypeParam: "dbaas/postgres", monitorAlertIDParam: 20000}, wantMessage: monitorServiceTypeInvalidError},
		{name: "query service type", args: map[string]any{monitorServiceTypeParam: "dbaas?x=1", monitorAlertIDParam: 20000}, wantMessage: monitorServiceTypeInvalidError},
		{name: "traversal service type", args: map[string]any{monitorServiceTypeParam: pathTraversalValue, monitorAlertIDParam: 20000}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseMissingAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase}, wantMessage: monitorAlertIDRequiredError},
		{name: caseZeroAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 0}, wantMessage: monitorAlertIDPositiveError},
		{name: caseStringAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: "20000"}, wantMessage: monitorAlertIDPositiveError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)

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

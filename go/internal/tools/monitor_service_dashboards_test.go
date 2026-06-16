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
	monitorServiceDashboardsToolPath = "/monitor/services/dbaas/dashboards"
	monitorServiceDashboardsToolName = "linode_monitor_service_dashboard_list"
)

func TestLinodeMonitorServiceDashboardsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)
	if tool.Name != monitorServiceDashboardsToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceDashboardsToolName)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !slices.Contains(tool.InputSchema.Required, monitorServiceTypeParam) {
		t.Errorf("tool.InputSchema.Required does not contain %v", monitorServiceTypeParam)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceDashboardsToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceDashboardsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceDashboardsToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:      monitorDashboardToolID,
				keyLabel:   monitorDashboardToolLabel,
				keyWidgets: []map[string]any{{keyLabel: monitorDashboardToolWidget}},
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase})

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

	if !strings.Contains(textContent.Text, monitorDashboardToolLabel) {
		t.Errorf("textContent.Text does not contain %v", monitorDashboardToolLabel)
	}

	if !strings.Contains(textContent.Text, monitorDashboardToolWidget) {
		t.Errorf("textContent.Text does not contain %v", monitorDashboardToolWidget)
	}
}

func TestLinodeMonitorServiceDashboardsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceDashboardsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceDashboardsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve "+monitorServiceDashboardsToolName) {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve "+monitorServiceDashboardsToolName)
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceDashboardsToolInvalidServiceTypeRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceType, args: map[string]any{}, wantMessage: monitorServiceTypeRequiredError},
		{name: caseNumericServiceType, args: map[string]any{monitorServiceTypeParam: 123}, wantMessage: monitorServiceTypeNonStringError},
		{name: caseSeparatorServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeSlash}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseQueryServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeQuery}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseTraversalServiceType, args: map[string]any{monitorServiceTypeParam: pathTraversalValue}, wantMessage: monitorServiceTypeInvalidError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)

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

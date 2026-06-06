package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	monitorServiceDashboardsToolPath = "/monitor/services/dbaas/dashboards"
	monitorServiceDashboardsToolName = "linode_monitor_service_dashboards"
)

func TestLinodeMonitorServiceDashboardsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)
		assertEqual(t, monitorServiceDashboardsToolName, tool.Name, "tool name should match")
		assertEqual(t, profiles.CapRead, capability, "tool should be read-only")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, monitorServiceDashboardsToolPath, r.URL.Path, "request path should match")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{
					keyID:      monitorDashboardToolID,
					keyLabel:   monitorDashboardToolLabel,
					keyWidgets: []map[string]any{{keyLabel: monitorDashboardToolWidget}},
				}},
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase})
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, monitorDashboardToolLabel, "response should contain dashboard label")
		assertContains(t, textContent.Text, monitorDashboardToolWidget, "response should contain dashboard widget")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, monitorServiceDashboardsToolPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceDashboardsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase})
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to retrieve "+monitorServiceDashboardsToolName, "response should identify failed tool")
		assertContains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid service type rejects before client", func(t *testing.T) {
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
				requireNoError(t, err, "handler should return validation as a tool error")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "invalid service type should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

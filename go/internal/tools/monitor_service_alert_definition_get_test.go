package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	monitorServiceAlertDefinitionGetPath     = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionGetToolName = "linode_monitor_service_alert_definition_get"
	monitorAlertIDParam                      = "alert_id"
	monitorAlertIDRequiredError              = "alert_id is required"
	monitorAlertIDPositiveError              = "alert_id must be a positive integer"
)

func TestLinodeMonitorServiceAlertDefinitionGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)
		assert.Equal(t, monitorServiceAlertDefinitionGetToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		assert.Contains(t, tool.InputSchema.Required, monitorAlertIDParam, "alert ID should be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:          20000,
				keyLabel:       "High CPU Usage",
				keyServiceType: monitorServiceToolTypeDatabase,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 20000})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "High CPU Usage", "response should contain alert label")
		assert.Contains(t, textContent.Text, monitorServiceToolTypeDatabase, "response should contain service type")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 20000})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve "+monitorServiceAlertDefinitionGetToolName, "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid arguments reject before client", func(t *testing.T) {
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
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid argument should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

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

const monitorServiceAlertDefinitionUpdateToolName = "linode_monitor_service_alert_definition_update"

func monitorAlertDefinitionUpdateArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam:                 monitorServiceToolTypeDatabase,
		monitorAlertIDParam:                     monitorAlertDefinitionToolID,
		monitorAlertDefinitionLabelParam:        monitorAlertDefinitionToolLabel + " Updated",
		monitorAlertDefinitionSeverityParam:     1,
		keyStatus:                               statusEnabled,
		monitorAlertDefinitionRuleCriteriaParam: map[string]any{"rules": []any{map[string]any{"metric": "cpu_usage", "operator": "gt", "threshold": 80}}},
		monitorAlertDefinitionTriggerParam:      map[string]any{"criteria_condition": monitorCriteriaAll, "evaluation_period_seconds": 300, "polling_interval_seconds": 300, "trigger_occurrences": 3},
		monitorAlertDefinitionChannelIDsParam:   []any{546, 392},
		keyDescription:                          "Updated alert when CPU usage is high",
		keyEntityIDs:                            []any{"13116"},
		keyConfirm:                              true,
	}
}

func TestLinodeMonitorServiceAlertDefinitionUpdateToolPartialUpdate(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		assertEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")

		var body map[string]any
		if !assertNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assertEqual(t, map[string]any{keyLabel: monitorAlertDefinitionToolLabel + " Partial"}, body)

		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionToolID,
			keyLabel:       monitorAlertDefinitionToolLabel + " Partial",
			keyServiceType: monitorServiceToolTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

	args := map[string]any{
		monitorServiceTypeParam:          monitorServiceToolTypeDatabase,
		monitorAlertIDParam:              monitorAlertDefinitionToolID,
		monitorAlertDefinitionLabelParam: monitorAlertDefinitionToolLabel + " Partial",
		keyConfirm:                       true,
	}
	result, err := handler(t.Context(), createRequestWithArgs(t, args))

	requireNoError(t, err, "handler should not return an error")
	requireNotNil(t, result, "result should not be nil")
	assertFalse(t, result.IsError, "should not be an error result")
}

func TestLinodeMonitorServiceAlertDefinitionUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)
		assertEqual(t, monitorServiceAlertDefinitionUpdateToolName, tool.Name, "tool name should match")
		assertEqual(t, profiles.CapWrite, capability, "tool should be write-capable")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		assertContains(t, tool.InputSchema.Required, monitorAlertIDParam, "alert ID should be required")
		assertContains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			assertEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assertNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assertEqual(t, monitorAlertDefinitionToolLabel+" Updated", body[keyLabel])
			expectNumericEqual(t, float64(1), body["severity"])
			assertEqual(t, statusEnabled, body[keyStatus])
			assertEqual(t, []any{float64(546), float64(392)}, body["channel_ids"])
			assertEqual(t, "Updated alert when CPU usage is high", body[keyDescription])
			assertEqual(t, []any{"13116"}, body[keyEntityIDs])

			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:          monitorAlertDefinitionToolID,
				keyLabel:       monitorAlertDefinitionToolLabel + " Updated",
				keyServiceType: monitorServiceToolTypeDatabase,
				keyStatus:      statusEnabled,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionUpdateArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, monitorAlertDefinitionToolLabel+" Updated", "response should contain alert label")
		assertContains(t, textContent.Text, monitorServiceToolTypeDatabase, "response should contain service type")
		assertContains(t, textContent.Text, statusEnabled, "response should contain status")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			assertEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionUpdateArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to update "+monitorServiceAlertDefinitionUpdateToolName, "response should identify failed tool")
		assertContains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

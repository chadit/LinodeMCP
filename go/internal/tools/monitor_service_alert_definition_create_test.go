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
	monitorServiceAlertDefinitionCreateToolName = "linode_monitor_service_alert_definition_create"
	monitorAlertDefinitionLabelParam            = "label"
	monitorAlertDefinitionSeverityParam         = "severity"
	monitorAlertDefinitionRuleCriteriaParam     = "rule_criteria"
	monitorAlertDefinitionTriggerParam          = "trigger_conditions"
	monitorAlertDefinitionChannelIDsParam       = "channel_ids"
)

func monitorAlertDefinitionCreateArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam:                 monitorServiceToolTypeDatabase,
		monitorAlertDefinitionLabelParam:        monitorAlertDefinitionToolLabel,
		monitorAlertDefinitionSeverityParam:     2,
		monitorAlertDefinitionRuleCriteriaParam: map[string]any{"rules": []any{map[string]any{keyMetric: "cpu_usage", "operator": "gt", "threshold": 80}}},
		monitorAlertDefinitionTriggerParam:      map[string]any{"criteria_condition": "ALL", "evaluation_period_seconds": 300, "polling_interval_seconds": 300, "trigger_occurrences": 3},
		monitorAlertDefinitionChannelIDsParam:   []any{546, 392},
		keyDescription:                          "Alert when CPU usage is high",
		"entity_ids":                            []any{"13116"},
		keyConfirm:                              true,
	}
}

func TestLinodeMonitorServiceAlertDefinitionCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)
		assert.Equal(t, monitorServiceAlertDefinitionCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write-capable")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, monitorServiceAlertDefinitionsToolPath, r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assert.Equal(t, monitorAlertDefinitionToolLabel, body[keyLabel])
			assert.InEpsilon(t, float64(2), body["severity"], 0)
			assert.Equal(t, []any{float64(546), float64(392)}, body["channel_ids"])
			assert.Equal(t, "Alert when CPU usage is high", body[keyDescription])
			assert.Equal(t, []any{"13116"}, body["entity_ids"])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:          20000,
				keyLabel:       monitorAlertDefinitionToolLabel,
				keyServiceType: monitorServiceToolTypeDatabase,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionCreateArgs())
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, monitorAlertDefinitionToolLabel, "response should contain alert label")
		assert.Contains(t, textContent.Text, monitorServiceToolTypeDatabase, "response should contain service type")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, monitorServiceAlertDefinitionsToolPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionCreateArgs())
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create "+monitorServiceAlertDefinitionCreateToolName, "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm required before client", func(t *testing.T) {
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

				args := monitorAlertDefinitionCreateArgs()
				if !testCase.set {
					delete(args, keyConfirm)
				}

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return confirmation failures as tool errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "confirm=true", "response should require confirm=true")
			})
		}
	})

	t.Run("invalid input rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			mutate      func(map[string]any)
			wantMessage string
		}{
			{name: caseSeparatorServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeSlash }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseQueryServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeQuery }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseTraversalServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = pathTraversalValue }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseMissingLabel, mutate: func(args map[string]any) { delete(args, monitorAlertDefinitionLabelParam) }, wantMessage: "label, severity, rule_criteria, trigger_conditions, and channel_ids are required"},
			{name: "invalid severity", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 5 }, wantMessage: "severity must be an integer from 0 through 3"},
			{name: "fractional severity", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 1.5 }, wantMessage: "severity must be an integer from 0 through 3"},
			{name: "empty rule criteria", mutate: func(args map[string]any) { args[monitorAlertDefinitionRuleCriteriaParam] = map[string]any{} }, wantMessage: "rule_criteria must be a non-empty object"},
			{name: "string trigger conditions", mutate: func(args map[string]any) { args[monitorAlertDefinitionTriggerParam] = "ALL" }, wantMessage: "trigger_conditions must be a non-empty object"},
			{name: "empty channel ids", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{} }, wantMessage: "channel_ids must be a non-empty array of positive integers"},
			{name: "zero channel id", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{0} }, wantMessage: "channel_ids must be a non-empty array of positive integers"},
			{name: "string entity id", mutate: func(args map[string]any) { args["entity_ids"] = []any{123} }, wantMessage: "entity_ids must be an array of non-empty strings"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := monitorAlertDefinitionCreateArgs()
				testCase.mutate(args)

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

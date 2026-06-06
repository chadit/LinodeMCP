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
		monitorAlertDefinitionTriggerParam:      map[string]any{"criteria_condition": monitorCriteriaAll, "evaluation_period_seconds": 300, "polling_interval_seconds": 300, "trigger_occurrences": 3},
		monitorAlertDefinitionChannelIDsParam:   []any{546, 392},
		keyDescription:                          "Alert when CPU usage is high",
		keyEntityIDs:                            []any{"13116"},
		keyConfirm:                              true,
	}
}

// The package-local assert* helpers report failures and continue; require*
// helpers stop the current test. assertNoError reports and returns false so
// HTTP handler checks can return without calling FailNow from another goroutine.

func TestLinodeMonitorServiceAlertDefinitionCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)
		assertEqual(t, monitorServiceAlertDefinitionCreateToolName, tool.Name, "tool name should match")
		assertEqual(t, profiles.CapWrite, capability, "tool should be write-capable")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		assertContains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPost, r.Method, "request method should be POST")
			assertEqual(t, monitorServiceAlertDefinitionsToolPath, r.URL.Path, "request path should match")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assertNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assertEqual(t, monitorAlertDefinitionToolLabel, body[keyLabel])
			expectNumericEqual(t, float64(2), body["severity"])
			assertEqual(t, []any{float64(546), float64(392)}, body["channel_ids"])
			assertEqual(t, "Alert when CPU usage is high", body[keyDescription])
			assertEqual(t, []any{"13116"}, body[keyEntityIDs])

			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
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
		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, monitorAlertDefinitionToolLabel, "response should contain alert label")
		assertContains(t, textContent.Text, monitorServiceToolTypeDatabase, "response should contain service type")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPost, r.Method, "request method should be POST")
			assertEqual(t, monitorServiceAlertDefinitionsToolPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionCreateArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to create "+monitorServiceAlertDefinitionCreateToolName, "response should identify failed tool")
		assertContains(t, textContent.Text, errForbidden, "response should include API error detail")
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
				requireNoError(t, err, "handler should return confirmation failures as tool errors")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "missing or invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, "confirm=true", "response should require confirm=true")
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
			{name: "invalid severity", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 5 }, wantMessage: errAlertDefinitionSeverity},
			{name: "fractional severity", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 1.5 }, wantMessage: errAlertDefinitionSeverity},
			{name: "empty rule criteria", mutate: func(args map[string]any) { args[monitorAlertDefinitionRuleCriteriaParam] = map[string]any{} }, wantMessage: "rule_criteria must be a non-empty object"},
			{name: "string trigger conditions", mutate: func(args map[string]any) { args[monitorAlertDefinitionTriggerParam] = monitorCriteriaAll }, wantMessage: "trigger_conditions must be a non-empty object"},
			{name: "empty channel ids", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{} }, wantMessage: errAlertDefinitionChannels},
			{name: "zero channel id", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{0} }, wantMessage: errAlertDefinitionChannels},
			{name: caseStringEntityID, mutate: func(args map[string]any) { args[keyEntityIDs] = []any{123} }, wantMessage: errAlertDefinitionEntityIDs},
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
				requireNoError(t, err, "handler should return validation as a tool error")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

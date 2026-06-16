package tools_test

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeMonitorServiceAlertDefinitionUpdateToolInvalidInput(t *testing.T) {
	t.Parallel()

	t.Run("invalid input rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			mutate      func(map[string]any)
			wantMessage string
		}{
			{name: caseMissingServiceType, mutate: func(args map[string]any) { delete(args, monitorServiceTypeParam) }, wantMessage: monitorServiceTypeRequiredError},
			{name: caseSeparatorServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeSlash }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseQueryServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeQuery }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseTraversalServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = pathTraversalValue }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseMissingAlertID, mutate: func(args map[string]any) { delete(args, monitorAlertIDParam) }, wantMessage: monitorAlertIDRequiredError},
			{name: caseZeroAlertID, mutate: func(args map[string]any) { args[monitorAlertIDParam] = 0 }, wantMessage: monitorAlertIDPositiveError},
			{name: caseStringAlertID, mutate: func(args map[string]any) { args[monitorAlertIDParam] = "20000" }, wantMessage: monitorAlertIDPositiveError},
			{name: "separator alert id", mutate: func(args map[string]any) { args[monitorAlertIDParam] = "1/2" }, wantMessage: monitorAlertIDPositiveError},
			{name: "query alert id", mutate: func(args map[string]any) { args[monitorAlertIDParam] = "1?x=2" }, wantMessage: monitorAlertIDPositiveError},
			{name: "traversal alert id", mutate: func(args map[string]any) { args[monitorAlertIDParam] = pathTraversalValue }, wantMessage: monitorAlertIDPositiveError},
			{name: caseNoUpdateFields, mutate: func(args map[string]any) {
				delete(args, monitorAlertDefinitionLabelParam)
				delete(args, monitorAlertDefinitionSeverityParam)
				delete(args, keyStatus)
				delete(args, monitorAlertDefinitionRuleCriteriaParam)
				delete(args, monitorAlertDefinitionTriggerParam)
				delete(args, monitorAlertDefinitionChannelIDsParam)
				delete(args, keyDescription)
				delete(args, keyEntityIDs)
			}, wantMessage: "at least one alert definition field must be provided"},
			{name: caseEmptyLabel, mutate: func(args map[string]any) { args[monitorAlertDefinitionLabelParam] = "" }, wantMessage: errLabelNonEmpty},
			{name: "invalid severity", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 5 }, wantMessage: errAlertDefinitionSeverity},
			{name: "invalid status", mutate: func(args map[string]any) { args[keyStatus] = "paused" }, wantMessage: "status must be enabled or disabled"},
			{name: "empty rule criteria", mutate: func(args map[string]any) { args[monitorAlertDefinitionRuleCriteriaParam] = map[string]any{} }, wantMessage: "rule_criteria must be a non-empty object"},
			{name: "string trigger conditions", mutate: func(args map[string]any) { args[monitorAlertDefinitionTriggerParam] = monitorCriteriaAll }, wantMessage: "trigger_conditions must be a non-empty object"},
			{name: "empty channel ids", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{} }, wantMessage: errAlertDefinitionChannels},
			{name: "zero channel id", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{0} }, wantMessage: errAlertDefinitionChannels},
			{name: "empty entity ids", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{} }, wantMessage: errAlertDefinitionEntityIDs},
			{name: caseStringEntityID, mutate: func(args map[string]any) { args[keyEntityIDs] = []any{123} }, wantMessage: errAlertDefinitionEntityIDs},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := monitorAlertDefinitionUpdateArgs()
				testCase.mutate(args)

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

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

				if !strings.Contains(textContent.Text, testCase.wantMessage) {
					t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
				}
			})
		}
	})
}

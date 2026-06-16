package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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

func TestLinodeMonitorServiceAlertDefinitionCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)
	if tool.Name != monitorServiceAlertDefinitionCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceAlertDefinitionCreateToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{monitorServiceTypeParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceAlertDefinitionCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceAlertDefinitionsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionsToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], monitorAlertDefinitionToolLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], monitorAlertDefinitionToolLabel)
		}

		if body[monitorAlertDefinitionSeverityParam] != float64(2) {
			t.Errorf("value = %v, want %v", body[monitorAlertDefinitionSeverityParam], float64(2))
		}

		for key, want := range map[string]any{
			"channel_ids":  []any{float64(546), float64(392)},
			keyDescription: "Alert when CPU usage is high",
			keyEntityIDs:   []any{"13116"},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          20000,
			keyLabel:       monitorAlertDefinitionToolLabel,
			keyServiceType: monitorServiceToolTypeDatabase,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionCreateArgs())

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

	if !strings.Contains(textContent.Text, monitorAlertDefinitionToolLabel) {
		t.Errorf("textContent.Text does not contain %v", monitorAlertDefinitionToolLabel)
	}

	if !strings.Contains(textContent.Text, monitorServiceToolTypeDatabase) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolTypeDatabase)
	}
}

func TestLinodeMonitorServiceAlertDefinitionCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceAlertDefinitionsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionCreateArgs())

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

	if !strings.Contains(textContent.Text, "Failed to create "+monitorServiceAlertDefinitionCreateToolName) {
		t.Errorf("textContent.Text does not contain %v", "Failed to create "+monitorServiceAlertDefinitionCreateToolName)
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceAlertDefinitionCreateToolConfirmRequiredBeforeClient(t *testing.T) {
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

func TestLinodeMonitorServiceAlertDefinitionCreateToolInvalidInputRejectsBeforeClient(t *testing.T) {
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

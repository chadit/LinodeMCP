package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	monitorServiceAlertDefinitionCloneToolName = "linode_monitor_service_alert_definition_clone"
	monitorAlertDefinitionEntityIDsParam       = "entity_ids"
	monitorAlertDefinitionGroupByParam         = "group_by"
	monitorAlertDefinitionGroupByValue         = "entity_id"
	monitorAlertDefinitionRegionsParam         = "regions"
	errCloneChannelIDs                         = "channel_ids must be an array of positive integers"
	errGroupByNonEmpty                         = "group_by must be an array of non-empty strings"
	errRegionsNonEmpty                         = "regions must be an array of non-empty strings"
)

func monitorAlertDefinitionCloneArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam:                 monitorServiceToolTypeDatabase,
		monitorAlertIDParam:                     float64(monitorAlertDefinitionToolID),
		monitorAlertDefinitionLabelParam:        monitorAlertDefinitionToolLabel + " Clone",
		monitorAlertDefinitionChannelIDsParam:   []any{float64(1)},
		keyDescription:                          "",
		monitorAlertDefinitionEntityIDsParam:    []any{"13116"},
		monitorAlertDefinitionGroupByParam:      []any{monitorAlertDefinitionGroupByValue},
		monitorAlertDefinitionRegionsParam:      []any{regionUSEast},
		monitorAlertDefinitionRuleCriteriaParam: map[string]any{},
		monitorAlertDefinitionSeverityParam:     float64(0),
		monitorAlertDefinitionTriggerParam:      map[string]any{},
		keyConfirm:                              true,
	}
}

func TestLinodeMonitorServiceAlertDefinitionCloneToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(&config.Config{})

	if tool.Name != monitorServiceAlertDefinitionCloneToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceAlertDefinitionCloneToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{
		monitorServiceTypeParam,
		monitorAlertIDParam,
		monitorAlertDefinitionLabelParam,
		monitorAlertDefinitionChannelIDsParam,
		keyDescription,
		monitorAlertDefinitionEntityIDsParam,
		monitorAlertDefinitionGroupByParam,
		monitorAlertDefinitionRegionsParam,
		monitorAlertDefinitionRuleCriteriaParam,
		monitorAlertDefinitionSeverityParam,
		monitorAlertDefinitionTriggerParam,
		keyConfirm,
		keyDryRun,
	} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("schema.Properties missing key %v", key)
		}
	}

	wantRequired := []string{monitorServiceTypeParam, monitorAlertIDParam, monitorAlertDefinitionLabelParam, keyConfirm}
	if !reflect.DeepEqual(schema.Required, wantRequired) {
		t.Errorf("schema.Required = %v, want %v", schema.Required, wantRequired)
	}
}

func TestLinodeMonitorServiceAlertDefinitionCloneToolValidationBeforeClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(map[string]any)
		wantMessage string
	}{
		{name: caseMissingServiceType, mutate: func(args map[string]any) { delete(args, monitorServiceTypeParam) }, wantMessage: monitorServiceTypeRequiredError},
		{name: "invalid service type", mutate: func(args map[string]any) { args[monitorServiceTypeParam] = "dbaas/postgres" }, wantMessage: "service_type must be a single non-empty service type slug"},
		{name: caseMissingAlertID, mutate: func(args map[string]any) { delete(args, monitorAlertIDParam) }, wantMessage: monitorAlertIDRequiredError},
		{name: "zero alert id", mutate: func(args map[string]any) { args[monitorAlertIDParam] = float64(0) }, wantMessage: monitorAlertIDPositiveError},
		{name: caseMissingLabel, mutate: func(args map[string]any) { delete(args, monitorAlertDefinitionLabelParam) }, wantMessage: errLabelNonEmpty},
		{name: caseEmptyLabel, mutate: func(args map[string]any) { args[monitorAlertDefinitionLabelParam] = " " }, wantMessage: errLabelNonEmpty},
		{name: "channel ids wrong type", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{"1"} }, wantMessage: errCloneChannelIDs},
		{name: "channel ids not array", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = "1" }, wantMessage: errCloneChannelIDs},
		{name: "channel ids zero", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{0} }, wantMessage: errCloneChannelIDs},
		{name: "channel ids negative", mutate: func(args map[string]any) { args[monitorAlertDefinitionChannelIDsParam] = []any{-1} }, wantMessage: errCloneChannelIDs},
		{name: "description wrong type", mutate: func(args map[string]any) { args[keyDescription] = 1 }, wantMessage: "description must be a string"},
		{name: "entity ids wrong type", mutate: func(args map[string]any) { args[monitorAlertDefinitionEntityIDsParam] = []any{1} }, wantMessage: errAlertDefinitionEntityIDs},
		{name: "entity ids blank", mutate: func(args map[string]any) { args[monitorAlertDefinitionEntityIDsParam] = []any{" "} }, wantMessage: errAlertDefinitionEntityIDs},
		{name: "group by wrong type", mutate: func(args map[string]any) { args[monitorAlertDefinitionGroupByParam] = []any{1} }, wantMessage: errGroupByNonEmpty},
		{name: "group by blank", mutate: func(args map[string]any) { args[monitorAlertDefinitionGroupByParam] = []any{" "} }, wantMessage: errGroupByNonEmpty},
		{name: "group by not array", mutate: func(args map[string]any) {
			args[monitorAlertDefinitionGroupByParam] = monitorAlertDefinitionGroupByValue
		}, wantMessage: errGroupByNonEmpty},
		{name: "regions wrong type", mutate: func(args map[string]any) { args[monitorAlertDefinitionRegionsParam] = []any{1} }, wantMessage: errRegionsNonEmpty},
		{name: "regions blank", mutate: func(args map[string]any) { args[monitorAlertDefinitionRegionsParam] = []any{""} }, wantMessage: errRegionsNonEmpty},
		{name: "rule criteria wrong type", mutate: func(args map[string]any) { args[monitorAlertDefinitionRuleCriteriaParam] = []any{} }, wantMessage: "rule_criteria must be an object"},
		{name: "fractional severity", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 1.5 }, wantMessage: errAlertDefinitionSeverity},
		{name: "severity out of range", mutate: func(args map[string]any) { args[monitorAlertDefinitionSeverityParam] = 4 }, wantMessage: errAlertDefinitionSeverity},
		{name: "trigger conditions wrong type", mutate: func(args map[string]any) { args[monitorAlertDefinitionTriggerParam] = "ALL" }, wantMessage: "trigger_conditions must be an object"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := monitorAlertDefinitionCloneArgs()
			testCase.mutate(args)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "http://127.0.0.1:1", Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			text, ok := result.Content[0].(mcp.TextContent)
			if !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}
		})
	}
}

func TestLinodeMonitorServiceAlertDefinitionCloneToolRequiresExplicitConfirm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value  any
		name   string
		delete bool
	}{
		{name: caseMissing, delete: true},
		{name: caseFalse, value: false},
		{name: caseString, value: "true"},
		{name: caseNumber, value: float64(1)},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := monitorAlertDefinitionCloneArgs()
			args[keyConfirm] = testCase.value

			if testCase.delete {
				delete(args, keyConfirm)
			}

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "http://127.0.0.1:1", Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			text, ok := result.Content[0].(mcp.TextContent)
			if !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain confirm=true", text.Text)
			}
		})
	}
}

func TestLinodeMonitorServiceAlertDefinitionCloneToolSuccessPreservesOverridesAndReturnedScope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/monitor/services/dbaas/alert-definitions/20000/clone" {
			t.Errorf("r.URL.Path = %v, want clone path", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		for _, key := range []string{monitorAlertDefinitionChannelIDsParam, keyDescription, monitorAlertDefinitionEntityIDsParam, monitorAlertDefinitionGroupByParam, monitorAlertDefinitionRegionsParam, monitorAlertDefinitionRuleCriteriaParam, monitorAlertDefinitionSeverityParam, monitorAlertDefinitionTriggerParam} {
			if _, ok := body[key]; !ok {
				t.Errorf("body missing optional key %v", key)
			}
		}

		if !reflect.DeepEqual(body[monitorAlertDefinitionChannelIDsParam], []any{}) {
			t.Errorf("empty channel_ids override was not preserved: %#v", body)
		}

		if !reflect.DeepEqual(body[monitorAlertDefinitionGroupByParam], []any{monitorAlertDefinitionGroupByValue}) {
			t.Errorf("group_by override was not preserved: %#v", body)
		}

		if !reflect.DeepEqual(body[monitorAlertDefinitionEntityIDsParam], []any{"13116"}) {
			t.Errorf("entity_ids override was not preserved: %#v", body)
		}

		if !reflect.DeepEqual(body[monitorAlertDefinitionRegionsParam], []any{regionUSEast}) {
			t.Errorf("regions override was not preserved: %#v", body)
		}

		if !reflect.DeepEqual(body[monitorAlertDefinitionRuleCriteriaParam], map[string]any{}) || !reflect.DeepEqual(body[monitorAlertDefinitionTriggerParam], map[string]any{}) {
			t.Errorf("object overrides were not preserved: %#v", body)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keySupportTicketID:                 monitorAlertDefinitionToolID + 1,
			managedServiceLabelParam:           monitorAlertDefinitionToolLabel + " Clone",
			"service_type":                     monitorServiceToolTypeDatabase,
			"severity":                         0,
			monitorAlertDefinitionGroupByParam: []string{monitorAlertDefinitionGroupByValue},
			"scope":                            keySupportTicketRegion,
			monitorAlertDefinitionRegionsParam: []string{regionUSEast},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(cfg)
	args := monitorAlertDefinitionCloneArgs()
	args[monitorAlertDefinitionChannelIDsParam] = []any{}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("result.IsError = true, want false: %#v", result.Content)
	}

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("response content is not text")
	}

	assertScopedCloneResponse(t, text.Text)
}

func assertScopedCloneResponse(t *testing.T, raw string) {
	t.Helper()

	var response struct {
		AlertDefinition struct {
			GroupBy []string `json:"group_by"`
			Scope   string   `json:"scope"`
			Regions []string `json:"regions"`
		} `json:"alert_definition"`
	}

	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !reflect.DeepEqual(response.AlertDefinition.GroupBy, []string{monitorAlertDefinitionGroupByValue}) {
		t.Errorf("response group_by = %v, want %v", response.AlertDefinition.GroupBy, []string{monitorAlertDefinitionGroupByValue})
	}

	if response.AlertDefinition.Scope != keySupportTicketRegion {
		t.Errorf("response scope = %q, want region", response.AlertDefinition.Scope)
	}

	if !reflect.DeepEqual(response.AlertDefinition.Regions, []string{regionUSEast}) {
		t.Errorf("response regions = %v, want %v", response.AlertDefinition.Regions, []string{regionUSEast})
	}
}

func TestLinodeMonitorServiceAlertDefinitionCloneToolDryRun(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keySupportTicketID:                  monitorAlertDefinitionToolID,
			managedServiceLabelParam:            monitorAlertDefinitionToolLabel,
			monitorServiceTypeParam:             monitorServiceToolTypeDatabase,
			monitorAlertDefinitionSeverityParam: 0,
			monitorAlertDefinitionGroupByParam:  []string{monitorAlertDefinitionGroupByValue},
			"scope":                             keySupportTicketRegion,
			monitorAlertDefinitionRegionsParam:  []string{regionUSEast},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(cfg)
	args := monitorAlertDefinitionCloneArgs()
	args[keyDryRun] = true

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("result.IsError = true, want false: %#v", result.Content)
	}

	text, textOK := result.Content[0].(mcp.TextContent)
	if !textOK {
		t.Fatalf("response content = %T, want mcp.TextContent", result.Content[0])
	}

	if !strings.Contains(text.Text, "/monitor/services/dbaas/alert-definitions/20000/clone") {
		t.Errorf("dry-run text %q does not contain clone path", text.Text)
	}

	var preview map[string]any
	if err := json.Unmarshal([]byte(text.Text), &preview); err != nil {
		t.Fatalf("decode dry-run response: %v", err)
	}

	currentState, ok := preview["current_state"].(map[string]any)
	if !ok {
		t.Fatalf("current_state = %T, want object", preview["current_state"])
	}

	if !reflect.DeepEqual(currentState[monitorAlertDefinitionGroupByParam], []any{monitorAlertDefinitionGroupByValue}) {
		t.Errorf(
			"current_state.group_by = %#v, want %#v",
			currentState[monitorAlertDefinitionGroupByParam],
			[]any{monitorAlertDefinitionGroupByValue},
		)
	}

	if currentState["scope"] != keySupportTicketRegion {
		t.Errorf("current_state.scope = %#v, want region", currentState["scope"])
	}

	if !reflect.DeepEqual(currentState[monitorAlertDefinitionRegionsParam], []any{regionUSEast}) {
		t.Errorf(
			"current_state.regions = %#v, want %#v",
			currentState[monitorAlertDefinitionRegionsParam],
			[]any{regionUSEast},
		)
	}
}

func TestLinodeMonitorServiceAlertDefinitionCloneToolReturnsClientAndAPIErrors(t *testing.T) {
	t.Parallel()

	t.Run("client configuration", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, monitorAlertDefinitionCloneArgs()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})

	t.Run("API response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "failure", http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCloneTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, monitorAlertDefinitionCloneArgs()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		text, ok := result.Content[0].(mcp.TextContent)
		if !ok || !strings.Contains(text.Text, "Failed to clone") {
			t.Errorf("error text %q does not contain clone failure", text.Text)
		}
	})
}

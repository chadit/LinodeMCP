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
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body, map[string]any{keyLabel: monitorAlertDefinitionToolLabel + " Partial"}) {
			t.Errorf("body = %v, want %v", body, map[string]any{keyLabel: monitorAlertDefinitionToolLabel + " Partial"})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionToolID,
			keyLabel:       monitorAlertDefinitionToolLabel + " Partial",
			keyServiceType: monitorServiceToolTypeDatabase,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeMonitorServiceAlertDefinitionUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)
	if tool.Name != monitorServiceAlertDefinitionUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceAlertDefinitionUpdateToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{monitorServiceTypeParam, monitorAlertIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceAlertDefinitionUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], monitorAlertDefinitionToolLabel+" Updated") {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], monitorAlertDefinitionToolLabel+" Updated")
		}

		if body[monitorAlertDefinitionSeverityParam] != float64(1) {
			t.Errorf("value = %v, want %v", body[monitorAlertDefinitionSeverityParam], float64(1))
		}

		for key, want := range map[string]any{
			keyStatus:      statusEnabled,
			"channel_ids":  []any{float64(546), float64(392)},
			keyDescription: "Updated alert when CPU usage is high",
			keyEntityIDs:   []any{"13116"},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionToolID,
			keyLabel:       monitorAlertDefinitionToolLabel + " Updated",
			keyServiceType: monitorServiceToolTypeDatabase,
			keyStatus:      statusEnabled,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionUpdateArgs())

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

	if !strings.Contains(textContent.Text, monitorAlertDefinitionToolLabel+" Updated") {
		t.Errorf("textContent.Text does not contain %v", monitorAlertDefinitionToolLabel+" Updated")
	}

	if !strings.Contains(textContent.Text, monitorServiceToolTypeDatabase) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolTypeDatabase)
	}

	if !strings.Contains(textContent.Text, statusEnabled) {
		t.Errorf("textContent.Text does not contain %v", statusEnabled)
	}
}

func TestLinodeMonitorServiceAlertDefinitionUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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
	_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

	req := createRequestWithArgs(t, monitorAlertDefinitionUpdateArgs())

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

	if !strings.Contains(textContent.Text, "Failed to update "+monitorServiceAlertDefinitionUpdateToolName) {
		t.Errorf("textContent.Text does not contain %v", "Failed to update "+monitorServiceAlertDefinitionUpdateToolName)
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

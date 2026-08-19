package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	monitorServicesToolPath                 = "/monitor/services"
	monitorServiceGetToolPath               = "/monitor/services/dbaas"
	monitorServiceMetricDefinitionsToolPath = "/monitor/services/dbaas/metric-definitions"
	monitorServiceAlertDefinitionsToolPath  = "/monitor/services/dbaas/alert-definitions"
	monitorServicesToolName                 = "linode_monitor_service_list"
	monitorServiceGetToolName               = "linode_monitor_service_get"
	monitorServiceMetricDefinitionsToolName = "linode_monitor_service_metric_definition_list"
	monitorServiceAlertDefinitionsToolName  = "linode_monitor_service_alert_definition_list"
	monitorServiceToolLabel                 = "Databases"
	monitorMetricDefinitionToolLabel        = "CPU Usage"
	monitorMetricDefinitionToolMetric       = "cpu_usage"
	monitorServiceToolTypeDatabase          = "dbaas"
	monitorServiceTypeParam                 = "service_type"
	monitorAlertDefinitionLabelParam        = "label"
	monitorAlertDefinitionSeverityParam     = "severity"
	monitorAlertDefinitionRuleCriteriaParam = "rule_criteria"
	monitorAlertDefinitionTriggerParam      = "trigger_conditions"
	monitorAlertDefinitionChannelIDsParam   = "channel_ids"
	monitorAlertDefinitionGroupByParam      = "group_by"
	monitorAlertDefinitionRegionsParam      = "regions"
	monitorServiceTypeInvalidError          = "service_type must be a single non-empty service type slug"
	monitorServiceTypeNonStringError        = "service_type must be a string"
	monitorServiceTypeRequiredError         = "service_type is required"
)

func TestLinodeMonitorServiceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := gentools.NewLinodeMonitorServiceGetTool(cfg)
	if tool.Name != monitorServiceGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceGetToolName)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), monitorServiceTypeParam) {
		t.Errorf("tool.RawInputSchema does not contain %v", monitorServiceTypeParam)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceGetToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceGetToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceToolLabel,
			keyServiceType: monitorServiceToolTypeDatabase,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeMonitorServiceGetTool(cfg)

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

	if !strings.Contains(textContent.Text, monitorServiceToolLabel) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolLabel)
	}

	if !strings.Contains(textContent.Text, monitorServiceToolTypeDatabase) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolTypeDatabase)
	}
}

func TestLinodeMonitorServiceGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceGetToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceGetToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeMonitorServiceGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve monitor service") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve monitor service")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceGetToolInvalidServiceTypeRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceType, args: map[string]any{}, wantMessage: monitorServiceTypeRequiredError},
		{name: "empty service type", args: map[string]any{monitorServiceTypeParam: ""}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseNumericServiceType, args: map[string]any{monitorServiceTypeParam: 123}, wantMessage: monitorServiceTypeNonStringError},
		{name: caseSeparatorServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeSlash}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseQueryServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeQuery}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseTraversalServiceType, args: map[string]any{monitorServiceTypeParam: pathTraversalValue}, wantMessage: monitorServiceTypeInvalidError},
		{name: "dot service type", args: map[string]any{monitorServiceTypeParam: "."}, wantMessage: monitorServiceTypeInvalidError},
		{name: "leading whitespace service type", args: map[string]any{monitorServiceTypeParam: " dbaas"}, wantMessage: monitorServiceTypeInvalidError},
		{name: "embedded whitespace service type", args: map[string]any{monitorServiceTypeParam: "db aas"}, wantMessage: monitorServiceTypeInvalidError},
		{name: "encoded separator service type", args: map[string]any{monitorServiceTypeParam: "dbaas%2Fpostgres"}, wantMessage: monitorServiceTypeInvalidError},
		{name: "backslash service type", args: map[string]any{monitorServiceTypeParam: `dbaas\postgres`}, wantMessage: monitorServiceTypeInvalidError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := gentools.NewLinodeMonitorServiceGetTool(cfg)

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

func TestLinodeMonitorServiceMetricDefinitionsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := gentools.NewLinodeMonitorServiceMetricDefinitionListTool(cfg)
	if tool.Name != monitorServiceMetricDefinitionsToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceMetricDefinitionsToolName)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), monitorServiceTypeParam) {
		t.Errorf("tool.RawInputSchema missing key %v", monitorServiceTypeParam)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceMetricDefinitionsToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceMetricDefinitionsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceMetricDefinitionsToolPath)
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
				keyLabel:      monitorMetricDefinitionToolLabel,
				keyMetric:     monitorMetricDefinitionToolMetric,
				keyMetricType: "gauge",
			}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeMonitorServiceMetricDefinitionListTool(cfg)

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

	if !strings.Contains(textContent.Text, monitorMetricDefinitionToolLabel) {
		t.Errorf("textContent.Text does not contain %v", monitorMetricDefinitionToolLabel)
	}

	if !strings.Contains(textContent.Text, monitorMetricDefinitionToolMetric) {
		t.Errorf("textContent.Text does not contain %v", monitorMetricDefinitionToolMetric)
	}
}

func TestLinodeMonitorServiceMetricDefinitionsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceMetricDefinitionsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceMetricDefinitionsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeMonitorServiceMetricDefinitionListTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceMetricDefinitionsToolInvalidServiceTypeRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceType, args: map[string]any{}, wantMessage: monitorServiceTypeRequiredError},
		{name: "empty service type", args: map[string]any{monitorServiceTypeParam: ""}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseNumericServiceType, args: map[string]any{monitorServiceTypeParam: 123}, wantMessage: monitorServiceTypeNonStringError},
		{name: caseSeparatorServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeSlash}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseQueryServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeQuery}, wantMessage: monitorServiceTypeInvalidError},
		{name: caseTraversalServiceType, args: map[string]any{monitorServiceTypeParam: pathTraversalValue}, wantMessage: monitorServiceTypeInvalidError},
		{name: "dot service type", args: map[string]any{monitorServiceTypeParam: "."}, wantMessage: monitorServiceTypeInvalidError},
		{name: "leading whitespace service type", args: map[string]any{monitorServiceTypeParam: " dbaas"}, wantMessage: monitorServiceTypeInvalidError},
		{name: "embedded whitespace service type", args: map[string]any{monitorServiceTypeParam: "db aas"}, wantMessage: monitorServiceTypeInvalidError},
		{name: "encoded separator service type", args: map[string]any{monitorServiceTypeParam: "dbaas%2Fpostgres"}, wantMessage: monitorServiceTypeInvalidError},
		{name: "backslash service type", args: map[string]any{monitorServiceTypeParam: `dbaas\postgres`}, wantMessage: monitorServiceTypeInvalidError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := gentools.NewLinodeMonitorServiceMetricDefinitionListTool(cfg)

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

func TestLinodeMonitorServiceAlertDefinitionsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := gentools.NewLinodeMonitorServiceAlertDefinitionListTool(cfg)
	if tool.Name != monitorServiceAlertDefinitionsToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceAlertDefinitionsToolName)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), monitorServiceTypeParam) {
		t.Errorf("tool.RawInputSchema missing key %v", monitorServiceTypeParam)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceAlertDefinitionsToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
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

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:          20000,
				keyLabel:       monitorAlertDefinitionToolLabel,
				keyServiceType: monitorServiceToolTypeDatabase,
			}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeMonitorServiceAlertDefinitionListTool(cfg)

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

	if !strings.Contains(textContent.Text, monitorAlertDefinitionToolLabel) {
		t.Errorf("textContent.Text does not contain %v", monitorAlertDefinitionToolLabel)
	}

	if !strings.Contains(textContent.Text, monitorServiceToolTypeDatabase) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolTypeDatabase)
	}
}

func TestLinodeMonitorServiceAlertDefinitionsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
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
	_, _, handler := gentools.NewLinodeMonitorServiceAlertDefinitionListTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceAlertDefinitionsToolInvalidServiceTypeRejectsBeforeClient(t *testing.T) {
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
			_, _, handler := gentools.NewLinodeMonitorServiceAlertDefinitionListTool(cfg)

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

func TestLinodeMonitorServicesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := gentools.NewLinodeMonitorServiceListTool(cfg)
	if tool.Name != monitorServicesToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServicesToolName)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("tool.InputSchema.Required = %v, want empty", tool.InputSchema.Required)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServicesToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServicesToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServicesToolPath)
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
				keyLabel:       monitorServiceToolLabel,
				keyServiceType: monitorServiceToolTypeDatabase,
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
	_, _, handler := gentools.NewLinodeMonitorServiceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	if !strings.Contains(textContent.Text, monitorServiceToolLabel) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolLabel)
	}

	if !strings.Contains(textContent.Text, monitorServiceToolTypeDatabase) {
		t.Errorf("textContent.Text does not contain %v", monitorServiceToolTypeDatabase)
	}
}

func TestLinodeMonitorServicesToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServicesToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServicesToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeMonitorServiceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

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

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyLongviewSubscription         = "longview_subscription"
	longviewSubscription            = "longview-40"
	errLongviewSubscriptionRequired = "longview_subscription is required"
)

func TestLinodeLongviewPlanToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeLongviewPlanTool(cfg)
	if tool.Name != "linode_longview_plan_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_longview_plan_get")
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

func TestLinodeLongviewPlanToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewPlanBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanBasePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 10,
			keyID:              "longview-10",
			keyLabel:           longviewSubscriptionLabel,
			keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewPlanTool(cfg)

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

	if !strings.Contains(textContent.Text, "longview-10") {
		t.Errorf("textContent.Text does not contain %v", "longview-10")
	}

	if !strings.Contains(textContent.Text, longviewSubscriptionLabel) {
		t.Errorf("textContent.Text does not contain %v", longviewSubscriptionLabel)
	}
}

func TestLinodeLongviewPlanToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewPlanBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanBasePath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewPlanTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_longview_plan_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_longview_plan_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeLongviewPlanUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeLongviewPlanUpdateTool(cfg)
	if tool.Name != "linode_longview_plan_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_longview_plan_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyLongviewSubscription, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyLongviewSubscription, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLongviewPlanUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != longviewPlanBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanBasePath)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body, map[string]any{keyLongviewSubscription: longviewSubscription}) {
			t.Errorf("body = %v, want %v", body, map[string]any{keyLongviewSubscription: longviewSubscription})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 40,
			keyID:              longviewSubscription,
			keyLabel:           "Longview Pro 40 pack",
			keyPrice:           map[string]float64{keyHourly: 0.12, keyMonthly: 80},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewPlanUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLongviewSubscription: longviewSubscription, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, longviewSubscription) {
		t.Errorf("textContent.Text does not contain %v", longviewSubscription)
	}

	if !strings.Contains(textContent.Text, "Longview Pro 40 pack") {
		t.Errorf("textContent.Text does not contain %v", "Longview Pro 40 pack")
	}
}

func TestLinodeLongviewPlanUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != longviewPlanBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanBasePath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewPlanUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLongviewSubscription: longviewSubscription, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Failed to update linode_longview_plan_update") {
		t.Errorf("textContent.Text does not contain %v", "Failed to update linode_longview_plan_update")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeLongviewPlanUpdateToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewPlanUpdateTool(cfg)

			args := map[string]any{keyLongviewSubscription: longviewSubscription}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
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

			if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
				t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeLongviewPlanUpdateToolValidationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing subscription", args: map[string]any{keyConfirm: true}, want: errLongviewSubscriptionRequired},
		{name: "empty subscription", args: map[string]any{keyLongviewSubscription: "", keyConfirm: true}, want: errLongviewSubscriptionRequired},
		{name: "whitespace subscription", args: map[string]any{keyLongviewSubscription: blankString, keyConfirm: true}, want: errLongviewSubscriptionRequired},
		{name: "numeric subscription", args: map[string]any{keyLongviewSubscription: 123, keyConfirm: true}, want: errLongviewSubscriptionRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewPlanUpdateTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
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

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text does not contain %v", testCase.want)
			}
		})
	}
}

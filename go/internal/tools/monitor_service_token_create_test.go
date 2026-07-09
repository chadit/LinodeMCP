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
	monitorServiceTokenCreateToolName = "linode_monitor_service_token_create"
	monitorServiceTokenToolPath       = "/monitor/services/dbaas/token"
	errMonitorServiceTokenEntityIDs   = "entity_ids must be a non-empty array of positive integers"
)

func monitorServiceTokenCreateArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam: monitorServiceToolTypeDatabase,
		keyEntityIDs:            []any{10, 20},
		keyConfirm:              true,
	}
}

// The package-local assert* helpers report failures and continue; require*
// helpers stop the current test. assertNoError reports and returns false so
// HTTP handler checks can return without calling FailNow from another goroutine.

func TestLinodeMonitorServiceTokenCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)
	if tool.Name != monitorServiceTokenCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, monitorServiceTokenCreateToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	var parsed struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(tool.RawInputSchema, &parsed); err != nil {
		t.Fatalf("unmarshal RawInputSchema: %v", err)
	}

	for _, key := range []string{monitorServiceTypeParam, keyConfirm} {
		if !slices.Contains(parsed.Required, key) {
			t.Errorf("RawInputSchema required does not contain %v", key)
		}
	}

	// entity_ids converts to a repeated proto field, which the generator never
	// marks required; the handler enforces its presence at runtime instead.
	if slices.Contains(parsed.Required, keyEntityIDs) {
		t.Errorf("RawInputSchema required unexpectedly contains %v", keyEntityIDs)
	}

	if _, ok := parsed.Properties[keyEntityIDs]; !ok {
		t.Errorf("RawInputSchema properties missing %v", keyEntityIDs)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMonitorServiceTokenCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceTokenToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceTokenToolPath)
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

		if !reflect.DeepEqual(body[keyEntityIDs], []any{float64(10), float64(20)}) {
			t.Errorf("body[keyEntityIDs] = %v, want %v", body[keyEntityIDs], []any{float64(10), float64(20)})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyToken:  "monitor-token",
			keyExpiry: tfaConfirmExpiry,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

	req := createRequestWithArgs(t, monitorServiceTokenCreateArgs())

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

	if !strings.Contains(textContent.Text, "monitor-token") {
		t.Errorf("textContent.Text does not contain %v", "monitor-token")
	}

	if !strings.Contains(textContent.Text, tfaConfirmExpiry) {
		t.Errorf("textContent.Text does not contain expiry %v", tfaConfirmExpiry)
	}
}

func TestLinodeMonitorServiceTokenCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceTokenToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceTokenToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

	req := createRequestWithArgs(t, monitorServiceTokenCreateArgs())

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

	if !strings.Contains(textContent.Text, "Failed to create "+monitorServiceTokenCreateToolName) {
		t.Errorf("textContent.Text does not contain %v", "Failed to create "+monitorServiceTokenCreateToolName)
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeMonitorServiceTokenCreateToolConfirmRequiredBeforeClient(t *testing.T) {
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

			args := monitorServiceTokenCreateArgs()
			if !testCase.set {
				delete(args, keyConfirm)
			}

			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

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

func TestLinodeMonitorServiceTokenCreateToolInvalidInputRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		mutate      func(map[string]any)
		wantMessage string
	}{
		{name: caseSeparatorServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeSlash }, wantMessage: monitorServiceTypeInvalidError},
		{name: caseQueryServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeQuery }, wantMessage: monitorServiceTypeInvalidError},
		{name: caseTraversalServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = pathTraversalValue }, wantMessage: monitorServiceTypeInvalidError},
		{name: caseMissingEntityIDs, mutate: func(args map[string]any) { delete(args, keyEntityIDs) }, wantMessage: errMonitorServiceTokenEntityIDs},
		{name: "empty entity ids", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{} }, wantMessage: errMonitorServiceTokenEntityIDs},
		{name: "zero entity id", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{0} }, wantMessage: errMonitorServiceTokenEntityIDs},
		{name: caseStringEntityID, mutate: func(args map[string]any) { args[keyEntityIDs] = []any{"10"} }, wantMessage: errMonitorServiceTokenEntityIDs},
		{name: "fractional entity id", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{10.5} }, wantMessage: errMonitorServiceTokenEntityIDs},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := monitorServiceTokenCreateArgs()
			testCase.mutate(args)

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

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

package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeProfilePreferencesUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)

	if tool.Name != "linode_profile_preferences_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_preferences_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keyPreferences, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}
}

func TestLinodeProfilePreferencesUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("request body should be readable: %v", readErr)

			return
		}

		{
			expectedJSON := `{"theme":"dark"}`
			actualJSON := string(body)

			var (
				expectedBody any
				actualBody   any
			)

			if err := json.Unmarshal([]byte(expectedJSON), &expectedBody); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := json.Unmarshal([]byte(actualJSON), &actualBody); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(actualBody, expectedBody) {
				t.Errorf("actualBody = %v, want %v", actualBody, expectedBody)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "dark") {
		t.Errorf("textContent.Text does not contain %v", "dark")
	}
}

func TestLinodeProfilePreferencesUpdateToolDryRunPreviewsWithoutPut(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}, keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Error("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "PUT") {
		t.Errorf("got %v, want %v", would["method"], "PUT")
	}

	if !reflect.DeepEqual(would["path"], tcProfilePreferences) {
		t.Errorf("got %v, want %v", would["path"], tcProfilePreferences)
	}

	previewBody, previewBodyOK := would["body"].(map[string]any)
	if !previewBodyOK {
		t.Error("previewBodyOK = false, want true")
	}

	if !reflect.DeepEqual(previewBody[profilePreferenceKeyTheme], profilePreferenceValueDark) {
		t.Errorf("previewBody[profilePreferenceKeyTheme] = %v, want %v", previewBody[profilePreferenceKeyTheme], profilePreferenceValueDark)
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Errorf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}
}

func TestLinodeProfilePreferencesUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_profile_preferences_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_profile_preferences_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfilePreferencesUpdateToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)

			args := map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfilePreferencesUpdateToolPreferencesBodyRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		preferences any
	}{
		{name: caseMissing},
		{name: "empty object", preferences: map[string]any{}},
		{name: caseString, preferences: "theme=dark"},
		{name: caseNumericConfirm, preferences: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)

			args := map[string]any{keyConfirm: true}
			if testCase.name != caseMissing {
				args[keyPreferences] = testCase.preferences
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "preferences must be a non-empty object") {
				t.Errorf("error text %q does not contain %q", text.Text, "preferences must be a non-empty object")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

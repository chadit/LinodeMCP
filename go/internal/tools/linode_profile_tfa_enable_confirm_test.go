package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	keyTFACode             = "tfa_code"
	keyTFAConfirmExpiry    = "expiry"
	tfaConfirmExpiry       = "2026-01-01T00:00:00"
	tfaConfirmScratchToken = "setup-token"
)

func TestLinodeProfileTFAEnableConfirmToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)

	if tool.Name != "linode_profile_tfa_enable_confirm" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_tfa_enable_confirm")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyTFACode, keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfileTFAEnableConfirmToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/tfa-enable-confirm" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/tfa-enable-confirm")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body, map[string]string{keyTFACode: "123456"}) {
			t.Errorf("body = %v, want %v", body, map[string]string{keyTFACode: "123456"})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{"scratch": tfaConfirmScratchToken, keyTFAConfirmExpiry: tfaConfirmExpiry}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyTFACode: "123456", keyConfirm: true})

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

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("result.Content[0] is not mcp.TextContent")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("unexpected error unmarshaling result: %v", err)
	}

	want := map[string]any{
		"message":           "Profile two-factor authentication enabled successfully",
		"scratch":           tfaConfirmScratchToken,
		keyTFAConfirmExpiry: tfaConfirmExpiry,
	}
	if !reflect.DeepEqual(payload, want) {
		t.Errorf("payload = %v, want %v", payload, want)
	}
}

func TestLinodeProfileTFAEnableConfirmToolDryRunPreviewsBodyWithoutPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyTFACode: "123456", keyDryRun: true})

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

	dryRun, dryRunOK := body[keyDryRun].(bool)
	if !dryRunOK {
		t.Error("dryRunOK = false, want true")
	}

	if !dryRun {
		t.Error("dryRun = false, want true")
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Error("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/profile/tfa-enable-confirm") {
		t.Errorf("got %v, want %v", would["path"], "/profile/tfa-enable-confirm")
	}

	wouldBody, bodyOK := would["body"].(map[string]any)
	if !bodyOK {
		t.Error("bodyOK = false, want true")
	}

	if !reflect.DeepEqual(wouldBody[keyTFACode], "123456") {
		t.Errorf("wouldBody[keyTFACode] = %v, want %v", wouldBody[keyTFACode], "123456")
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Errorf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Error("gotString = false, want true")
	}

	if !strings.Contains(effect, "enabled") {
		t.Errorf("effect does not contain %v", "enabled")
	}
}

func TestLinodeProfileTFAEnableConfirmToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/tfa-enable-confirm" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/tfa-enable-confirm")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyTFACode: "123456", keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to confirm linode_profile_tfa_enable_confirm") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to confirm linode_profile_tfa_enable_confirm")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileTFAEnableConfirmToolTfaCodeRequiredBeforeConfirmAndClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code any
	}{
		{name: caseMissing},
		{name: caseEmpty, code: ""},
		{name: caseString, code: 123456},
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
			_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)

			args := map[string]any{keyConfirm: true}
			if testCase.name != caseMissing {
				args[keyTFACode] = testCase.code
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "tfa_code must be a non-empty string") {
				t.Errorf("error text %q does not contain %q", text.Text, "tfa_code must be a non-empty string")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileTFAEnableConfirmToolConfirmRequiredBeforeClient(t *testing.T) {
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
			_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)

			args := map[string]any{keyTFACode: "123456"}
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

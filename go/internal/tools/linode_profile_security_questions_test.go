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

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	toolProfileSecurityQuestionsAnswer = "linode_profile_security_question_answer"
	keySecurityQuestions               = "security_questions"
	profileSecurityQuestionsPayload    = "answer payload"
)

func TestLinodeProfileSecurityQuestionsAnswerToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

	if tool.Name != toolProfileSecurityQuestionsAnswer {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolProfileSecurityQuestionsAnswer)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keySecurityQuestions, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}
}

func TestLinodeProfileSecurityQuestionsAnswerToolSecurityQuestionsValidationBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    map[string]any
		message string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}, message: "security_questions is required"},
		{name: "empty", args: map[string]any{keySecurityQuestions: "", keyConfirm: true}, message: "security_questions must not be empty"},
		{name: caseString, args: map[string]any{keySecurityQuestions: []any{"unexpected"}, keyConfirm: true}, message: "security_questions must be a string"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

			req := createRequestWithArgs(t, tt.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.message) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.message)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileSecurityQuestionsAnswerToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

			args := map[string]any{keySecurityQuestions: profileSecurityQuestionsPayload}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileSecurityQuestionsAnswerToolDryRunPreviewsWithoutClientCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: profileSecurityQuestionsPayload, keyDryRun: true})

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

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], toolProfileSecurityQuestionsAnswer) {
		t.Errorf("got %v, want %v", body["tool"], toolProfileSecurityQuestionsAnswer)
	}

	would, isObject := body["would_execute"].(map[string]any)
	if !isObject {
		t.Error("isObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], http.MethodPost) {
		t.Errorf("got %v, want %v", would["method"], http.MethodPost)
	}

	if !reflect.DeepEqual(would["path"], tcProfileSecurityQuestions) {
		t.Errorf("got %v, want %v", would["path"], tcProfileSecurityQuestions)
	}

	bodyValue, hasBody := would["body"].(map[string]any)
	if !hasBody {
		t.Error("hasBody = false, want true")
	}

	if !reflect.DeepEqual(bodyValue[keySecurityQuestions], "[redacted]") {
		t.Errorf("bodyValue[keySecurityQuestions] = %v, want %v", bodyValue[keySecurityQuestions], "[redacted]")
	}

	if strings.Contains(textContent.Text, profileSecurityQuestionsPayload) {
		t.Errorf("dry-run output must not expose answers: %q unexpectedly contains %q", textContent.Text, profileSecurityQuestionsPayload)
	}

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "answers are saved") {
		t.Errorf("textContent.Text does not contain %v", "answers are saved")
	}
}

func TestLinodeProfileSecurityQuestionsAnswerToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		{
			expectedJSON := `{"security_questions":"answer payload"}`
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

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: profileSecurityQuestionsPayload, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Profile security questions answered successfully") {
		t.Errorf("textContent.Text does not contain %v", "Profile security questions answered successfully")
	}
}

func TestLinodeProfileSecurityQuestionsAnswerToolApiErrorProducesToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "security questions rejected"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: profileSecurityQuestionsPayload, keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to answer profile security questions") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to answer profile security questions")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "security questions rejected") {
		t.Errorf("error text %q does not contain %q", text.Text, "security questions rejected")
	}
}

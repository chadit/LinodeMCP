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
	keyQuestionID                      = "question_id"
	keyResponse                        = "response"
	securityAnswerResponseFirst        = "first answer"
	securityAnswerResponseSecond       = "second answer"
	securityAnswerResponseThird        = "third answer"
)

// validSecurityQuestionsPayload returns the array form the tool now accepts:
// exactly three answers, each an object with a positive question_id and a
// response of valid length.
func validSecurityQuestionsPayload() []any {
	return []any{
		map[string]any{keyQuestionID: 1, keyResponse: securityAnswerResponseFirst},
		map[string]any{keyQuestionID: 2, keyResponse: securityAnswerResponseSecond},
		map[string]any{keyQuestionID: 3, keyResponse: securityAnswerResponseThird},
	}
}

// assertRedactedSecurityAnswers verifies the dry-run preview body carries the
// answers as an array whose responses are scrubbed while question IDs survive.
func assertRedactedSecurityAnswers(t *testing.T, answers []any) {
	t.Helper()

	if len(answers) != len(validSecurityQuestionsPayload()) {
		t.Errorf("len(answers) = %v, want %v", len(answers), len(validSecurityQuestionsPayload()))
	}

	for idx, raw := range answers {
		answer, isObject := raw.(map[string]any)
		if !isObject {
			t.Fatalf("answers[%d] = %T, want map[string]any", idx, raw)
		}

		if !reflect.DeepEqual(answer[keyResponse], "[redacted]") {
			t.Errorf("answers[%d][response] = %v, want %v", idx, answer[keyResponse], "[redacted]")
		}

		if _, hasQuestionID := answer[keyQuestionID]; !hasQuestionID {
			t.Errorf("answers[%d] missing question_id", idx)
		}
	}
}

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

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keySecurityQuestions, keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
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
		{
			name: "wrong count",
			args: map[string]any{keySecurityQuestions: []any{
				map[string]any{keyQuestionID: 1, keyResponse: securityAnswerResponseFirst},
			}, keyConfirm: true},
			message: "security_questions must contain exactly 3 answers",
		},
		{
			name: "non-positive question_id",
			args: map[string]any{keySecurityQuestions: []any{
				map[string]any{keyQuestionID: 0, keyResponse: securityAnswerResponseFirst},
				map[string]any{keyQuestionID: 2, keyResponse: securityAnswerResponseSecond},
				map[string]any{keyQuestionID: 3, keyResponse: securityAnswerResponseThird},
			}, keyConfirm: true},
			message: "question_id must be a positive integer",
		},
		{
			name: "duplicate question_id",
			args: map[string]any{keySecurityQuestions: []any{
				map[string]any{keyQuestionID: 1, keyResponse: securityAnswerResponseFirst},
				map[string]any{keyQuestionID: 1, keyResponse: securityAnswerResponseSecond},
				map[string]any{keyQuestionID: 3, keyResponse: securityAnswerResponseThird},
			}, keyConfirm: true},
			message: "question_id values must be unique",
		},
		{
			name: "response too short",
			args: map[string]any{keySecurityQuestions: []any{
				map[string]any{keyQuestionID: 1, keyResponse: "no"},
				map[string]any{keyQuestionID: 2, keyResponse: securityAnswerResponseSecond},
				map[string]any{keyQuestionID: 3, keyResponse: securityAnswerResponseThird},
			}, keyConfirm: true},
			message: "response length must be between 3 and 17 characters",
		},
		{
			name: caseString,
			args: map[string]any{keySecurityQuestions: []any{
				map[string]any{keyQuestionID: "not-an-int", keyResponse: securityAnswerResponseFirst},
				map[string]any{keyQuestionID: 2, keyResponse: securityAnswerResponseSecond},
				map[string]any{keyQuestionID: 3, keyResponse: securityAnswerResponseThird},
			}, keyConfirm: true},
			message: "security_questions must be an array of objects",
		},
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

			args := map[string]any{keySecurityQuestions: validSecurityQuestionsPayload()}
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

	req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: validSecurityQuestionsPayload(), keyDryRun: true})

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

	answers, isArray := bodyValue[keySecurityQuestions].([]any)
	if !isArray {
		t.Fatalf("bodyValue[keySecurityQuestions] = %T, want []any", bodyValue[keySecurityQuestions])
	}

	assertRedactedSecurityAnswers(t, answers)

	for _, plaintext := range []string{securityAnswerResponseFirst, securityAnswerResponseSecond, securityAnswerResponseThird} {
		if strings.Contains(textContent.Text, plaintext) {
			t.Errorf("dry-run output must not expose answers: %q unexpectedly contains %q", textContent.Text, plaintext)
		}
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
			expectedJSON := `{"security_questions":[{"question_id":1,"response":"first answer"},{"question_id":2,"response":"second answer"},{"question_id":3,"response":"third answer"}]}`
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

	req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: validSecurityQuestionsPayload(), keyConfirm: true})

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

	req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: validSecurityQuestionsPayload(), keyConfirm: true})

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

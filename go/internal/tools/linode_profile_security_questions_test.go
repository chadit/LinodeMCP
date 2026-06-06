package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	toolProfileSecurityQuestionsAnswer = "linode_profile_security_questions_answer"
	keySecurityQuestions               = "security_questions"
	profileSecurityQuestionsPayload    = "answer payload"
)

func TestLinodeProfileSecurityQuestionsAnswerTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

		checkEqual(t, toolProfileSecurityQuestionsAnswer, tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, profiles.CapAdmin, capability, "security question answers should be CapAdmin")
		expectNotNil(t, handler, "handler should not be nil")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keySecurityQuestions, "schema should include security_questions")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
	})

	t.Run("security questions validation before client call", func(t *testing.T) {
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

				expectNoError(t, err, "handler should not return transport error")
				expectNotNil(t, result, "result should not be nil")
				checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, tt.message)
				checkEqual(t, int32(0), calls.Load(), "validation failure must happen before client call")
			})
		}
	})

	t.Run("confirm required before client call", func(t *testing.T) {
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

				expectNoError(t, err, "handler should not return transport error")
				expectNotNil(t, result, "result should not be nil")
				checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				checkEqual(t, int32(0), calls.Load(), "confirm failure must happen before client call")
			})
		}
	})

	t.Run("dry run previews without client call", func(t *testing.T) {
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

		expectNoError(t, err, "handler should not return transport error")
		expectNotNil(t, result, "result should not be nil")
		checkFalseWithMode(t, false, result.IsError, "dry-run should not be an error")
		checkEqual(t, int32(0), calls.Load(), "dry-run must not call the API")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, toolProfileSecurityQuestionsAnswer, body["tool"])
		would, isObject := body["would_execute"].(map[string]any)
		expectTrue(t, isObject, "would_execute should be an object")
		checkEqual(t, http.MethodPost, would["method"])
		checkEqual(t, "/profile/security-questions", would["path"])
		bodyValue, hasBody := would["body"].(map[string]any)
		expectTrue(t, hasBody, "would_execute body should be an object")
		checkEqual(t, "[redacted]", bodyValue[keySecurityQuestions])

		if contains(textContent.Text, profileSecurityQuestionsPayload) {
			t.Errorf("expected %v not to contain %v%s", textContent.Text, profileSecurityQuestionsPayload, expectationMessage([]string{"dry-run output must not expose answers"}))
		}

		expectContainsWithMode(t, false, textContent.Text, "side_effects", "dry-run should surface a side effect")
		expectContainsWithMode(t, false, textContent.Text, "answers are saved", "side effect should describe the action")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")

			body, err := io.ReadAll(r.Body)
			checkNoError(t, err)
			{
				expectedJSON := `{"security_questions":"answer payload"}`
				actualJSON := string(body)

				var (
					expectedBody any
					actualBody   any
				)

				expectNoError(t, json.Unmarshal([]byte(expectedJSON), &expectedBody))
				expectNoError(t, json.Unmarshal([]byte(actualJSON), &actualBody))
				checkEqual(t, expectedBody, actualBody)
			}

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: profileSecurityQuestionsPayload, keyConfirm: true})
		result, err := handler(t.Context(), req)

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		checkFalseWithMode(t, false, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "Profile security questions answered successfully", "response should contain success message")
	})

	t.Run("api error produces tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "security questions rejected"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileSecurityQuestionsAnswerTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keySecurityQuestions: profileSecurityQuestionsPayload, keyConfirm: true})
		result, err := handler(t.Context(), req)

		expectNoError(t, err, "handler should not return transport error")
		expectNotNil(t, result, "result should not be nil")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to answer profile security questions")
		assertErrorContains(t, result, "security questions rejected")
	})
}

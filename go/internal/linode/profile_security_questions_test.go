package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAnswerProfileSecurityQuestionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")
		checkEqual(t, "application/json", r.Header.Get("Content-Type"), "values differ")

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err, "expected no error")

		var (
			expectedJSON any
			actualJSON   any
		)

		requireNoError(t, json.Unmarshal([]byte(`{"security_questions":"answer payload"}`), &expectedJSON), "test JSON should decode")
		requireNoError(t, json.Unmarshal([]byte(string(body)), &actualJSON), "request JSON should decode")
		checkEqual(t, expectedJSON, actualJSON, "JSON mismatch")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AnswerProfileSecurityQuestions(t.Context(), &linode.AnswerProfileSecurityQuestionsRequest{SecurityQuestions: "answer payload"})

	requireNoError(t, err, "AnswerProfileSecurityQuestions should succeed on 200 response")
}

func TestClientAnswerProfileSecurityQuestionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"security questions rejected"}]}`))
		checkNoError(t, err, "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AnswerProfileSecurityQuestions(t.Context(), &linode.AnswerProfileSecurityQuestionsRequest{})

	requireError(t, err, "AnswerProfileSecurityQuestions should fail on 400 response")

	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode, "values differ")
}

func TestClientAnswerProfileSecurityQuestionsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err, "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AnswerProfileSecurityQuestions(t.Context(), &linode.AnswerProfileSecurityQuestionsRequest{})

	requireError(t, err, "AnswerProfileSecurityQuestions should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "AnswerProfileSecurityQuestions must not retry and replay a mutating request")
}

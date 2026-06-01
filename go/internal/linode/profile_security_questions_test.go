package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAnswerProfileSecurityQuestionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"security_questions":"answer payload"}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AnswerProfileSecurityQuestions(t.Context(), &linode.AnswerProfileSecurityQuestionsRequest{SecurityQuestions: "answer payload"})

	require.NoError(t, err, "AnswerProfileSecurityQuestions should succeed on 200 response")
}

func TestClientAnswerProfileSecurityQuestionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"security questions rejected"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AnswerProfileSecurityQuestions(t.Context(), &linode.AnswerProfileSecurityQuestionsRequest{})

	require.Error(t, err, "AnswerProfileSecurityQuestions should fail on 400 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientAnswerProfileSecurityQuestionsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AnswerProfileSecurityQuestions(t.Context(), &linode.AnswerProfileSecurityQuestionsRequest{})

	require.Error(t, err, "AnswerProfileSecurityQuestions should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "AnswerProfileSecurityQuestions must not retry and replay a mutating request")
}

package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")
		checkEqual(t, http.NoBody, r.Body, "request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteProfileToken(t.Context(), 12345)

	requireNoError(t, err, "DeleteProfileToken should succeed on 200 response")
}

func TestClientDeleteProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteProfileToken(t.Context(), 12345)

	requireError(t, err, "DeleteProfileToken should fail on 403 response")

	apiErr := requireAPIError(t, err, "error should wrap APIError")
	requireNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode, "values differ")
	checkEqual(t, errForbidden, apiErr.Message, "values differ")
}

func TestClientDeleteProfileTokenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	err := client.DeleteProfileToken(t.Context(), 12345)

	requireError(t, err, "DeleteProfileToken should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "personal access token revocation must not be retried")
}

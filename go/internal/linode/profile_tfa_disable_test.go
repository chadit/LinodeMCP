package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDisableProfileTFASuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-disable", r.URL.Path, "request path should be /profile/tfa-disable")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")
		checkEqual(t, int64(0), r.ContentLength, "POST request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DisableProfileTFA(t.Context())

	requireNoError(t, err, "DisableProfileTFA should succeed on 200 response")
}

func TestClientDisableProfileTFAAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-disable", r.URL.Path, "request path should be /profile/tfa-disable")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DisableProfileTFA(t.Context())

	requireError(t, err, "DisableProfileTFA should fail on 403 response")

	_ = requireAPIError(t, err, "DisableProfileTFA should return APIError")
}

func TestClientDisableProfileTFANoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-disable", r.URL.Path, "request path should be /profile/tfa-disable")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.DisableProfileTFA(t.Context())

	requireError(t, err, "DisableProfileTFA should return the transient error")
	checkEqual(t, int32(1), calls.Load(), "security-state-changing POST must not be retried")
}

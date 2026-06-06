package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const managedCredentialRevokePath = "/managed/credentials/9991/revoke"

func TestClientRevokeManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedCredentialRevokePath, r.URL.Path, "request path should revoke managed credential")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkNoError(t, r.Body.Close())

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.RevokeManagedCredential(t.Context(), 9991)

	requireNoError(t, err, "RevokeManagedCredential should succeed on 200 response")
}

func TestClientRevokeManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedCredentialRevokePath, r.URL.Path, "request path should revoke managed credential")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot revoke managed credentials"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.RevokeManagedCredential(t.Context(), 9991)

	requireError(t, err)
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientRevokeManagedCredentialDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedCredentialRevokePath, r.URL.Path, "request path should revoke managed credential")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.RevokeManagedCredential(t.Context(), 9991)

	requireError(t, err, "RevokeManagedCredential should fail on 500 response")
	checkEqual(t, int32(1), attempts.Load(), "RevokeManagedCredential must not retry and replay a mutating request")
}

package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetStackScriptSuccess(t *testing.T) {
	t.Parallel()

	script := linode.StackScript{ID: 123, Label: "deploy-base", Description: "Base deploy script"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(script))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetStackScript(t.Context(), 123)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, 123, result.ID)
	checkEqual(t, "deploy-base", result.Label)
}

func TestClientGetStackScriptError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetStackScript(t.Context(), 123)

	apiErr := requireAPIError(t, err)
	checkNil(t, result)
	checkEqual(t, errTemporaryFailure, apiErr.Message)
}

func TestClientGetStackScriptRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	script := linode.StackScript{ID: 123, Label: "deploy-base"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(script))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetStackScript(t.Context(), 123)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load())
	checkEqual(t, 123, result.ID)
}

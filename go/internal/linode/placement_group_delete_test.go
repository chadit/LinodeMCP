package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeletePlacementGroupRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/placement/groups/528", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")
		checkEqual(t, http.NoBody, r.Body, "delete request should not include a body")
		w.WriteHeader(http.StatusOK)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeletePlacementGroup(t.Context(), 528)

	requireNoError(t, err, "DeletePlacementGroup should succeed")
	checkEqual(t, int32(1), requestCount.Load(), "DeletePlacementGroup should make one request")
}

func TestClientDeletePlacementGroupError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeletePlacementGroup(t.Context(), 528)

	requireError(t, err, "DeletePlacementGroup should propagate HTTP errors")
}

func TestClientDeletePlacementGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	err := client.DeletePlacementGroup(t.Context(), 528)

	requireError(t, err, "DeletePlacementGroup should return the transient failure")
	checkEqual(t, int32(1), requestCount.Load(), "destructive DELETE route must not retry transient failures")
}

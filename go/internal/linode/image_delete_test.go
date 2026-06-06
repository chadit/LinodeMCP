package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteImageSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/images/private%2F12345", r.URL.EscapedPath(), "image ID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "delete request should not include a body")
		w.WriteHeader(http.StatusOK)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImage(t.Context(), privateImage12345Fixture)

	requireNoError(t, err)
	checkEqual(t, int32(1), requestCount.Load(), "delete should make one request")
}

func TestClientDeleteImageEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/images/private%2F%2E%2E%3Fquery%23frag", r.URL.EscapedPath(), "image ID should be one encoded path segment")
		w.WriteHeader(http.StatusOK)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImage(t.Context(), "private/..?query#frag")

	requireNoError(t, err)
}

func TestClientDeleteImageEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/images/%2E%2E", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.WriteHeader(http.StatusOK)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImage(t.Context(), "..")

	requireNoError(t, err)
}

func TestClientDeleteImageError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImage(t.Context(), privateImage12345Fixture)

	requireError(t, err)
}

func TestClientDeleteImageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteImage(t.Context(), privateImage12345Fixture)

	requireError(t, err)
	checkEqual(t, int32(1), calls.Load(), "destructive DELETE route must not retry transient failures")
}

package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteImageShareGroupMemberTokenSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/images/sharegroups/123/members/"+imageShareGroupTokenUUID, r.URL.Path, "request path should include share group ID and token UUID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "delete request should not include a body")
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImageShareGroupMemberToken(t.Context(), 123, imageShareGroupTokenUUID)

	require.NoError(t, err)
	assert.Equal(t, int32(1), requestCount.Load(), "delete should make one request")
}

func TestClientDeleteImageShareGroupMemberTokenEscapesTokenPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/images/sharegroups/123/members/token%2F..%3Fquery%23frag", r.URL.EscapedPath(), "token UUID should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImageShareGroupMemberToken(t.Context(), 123, "token/..?query#frag")

	require.NoError(t, err)
}

func TestClientDeleteImageShareGroupMemberTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/images/sharegroups/123/members/%2E%2E", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImageShareGroupMemberToken(t.Context(), 123, "..")

	require.NoError(t, err)
}

func TestClientDeleteImageShareGroupMemberTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteImageShareGroupMemberToken(t.Context(), 123, imageShareGroupTokenUUID)

	require.Error(t, err)
}

func TestClientDeleteImageShareGroupMemberTokenDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteImageShareGroupMemberToken(t.Context(), 123, imageShareGroupTokenUUID)

	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "destructive DELETE route must not retry transient failures")
}

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

func TestClientGetLKEVersionSuccess(t *testing.T) {
	t.Parallel()

	version := linode.LKEVersion{ID: lkeVersion129}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/lke/versions/1.29", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(version))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetLKEVersion(t.Context(), lkeVersion129)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, lkeVersion129, result.ID)
}

func TestClientGetLKEVersionEscapesPathSegment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/versions/1.29/edge", r.URL.Path, "request path should include decoded version ID")
		assert.Equal(t, "/lke/versions/1.29%2Fedge", r.URL.EscapedPath(), "version ID should be one encoded path segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.LKEVersion{ID: lkeVersionWithSlash}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetLKEVersion(t.Context(), lkeVersionWithSlash)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, lkeVersionWithSlash, result.ID)
}

func TestClientGetLKEVersionError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetLKEVersion(t.Context(), lkeVersion129)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientGetLKEVersionRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.LKEVersion{ID: lkeVersion129}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetLKEVersion(t.Context(), lkeVersion129)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

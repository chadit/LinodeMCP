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

func TestClientGetLKETierVersionSuccess(t *testing.T) {
	t.Parallel()

	tierVersion := linode.LKETierVersion{ID: "1.31", Tier: lkeTierStandard}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/lke/tiers/standard/versions/1.31", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(tierVersion))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetLKETierVersion(t.Context(), lkeTierStandard, "1.31")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "1.31", result.ID)
	assert.Equal(t, lkeTierStandard, result.Tier)
}

func TestClientGetLKETierVersionEscapesPathParameters(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/tiers/standard%2Ftier/versions/1.31%3Fbad=1", r.URL.EscapedPath(), "path parameters should be encoded as single segments")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.LKETierVersion{ID: "1.31?bad=1", Tier: "standard/tier"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetLKETierVersion(t.Context(), "standard/tier", "1.31?bad=1")

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestClientGetLKETierVersionRetriesReadOnlyRoute(t *testing.T) {
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.LKETierVersion{ID: "1.31", Tier: lkeTierStandard}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetLKETierVersion(t.Context(), lkeTierStandard, "1.31")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

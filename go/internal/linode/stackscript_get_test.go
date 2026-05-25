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

func TestClientGetStackScriptSuccess(t *testing.T) {
	t.Parallel()

	script := linode.StackScript{ID: 123, Label: "deploy-base", Description: "Base deploy script"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(script))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetStackScript(t.Context(), 123)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, "deploy-base", result.Label)
}

func TestClientGetStackScriptError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetStackScript(t.Context(), 123)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, errTemporaryFailure)
}

func TestClientGetStackScriptRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	script := linode.StackScript{ID: 123, Label: "deploy-base"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(script))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetStackScript(t.Context(), 123)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, 123, result.ID)
}

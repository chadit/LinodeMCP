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

func TestClientGetInstanceConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:    float64(456),
			keyLabel: "boot-config",
			"kernel": configKernelLatest,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetInstanceConfig(t.Context(), 123, 456)

	require.NoError(t, err, "GetInstanceConfig should succeed on 200 response")
	assert.Equal(t, "boot-config", got.Label)
	assert.Equal(t, configKernelLatest, got.Kernel)
}

func TestClientGetInstanceConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetInstanceConfig(t.Context(), 123, 456)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetInstanceConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid IDs should be rejected before issuing HTTP request")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	t.Run("invalid linode id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetInstanceConfig(t.Context(), 0, 456)

		require.ErrorIs(t, err, linode.ErrLinodeIDPositive)
	})

	t.Run("invalid config id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetInstanceConfig(t.Context(), 123, 0)

		require.ErrorIs(t, err, linode.ErrConfigIDPositive)
	})
}

func TestClientGetInstanceConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "rate limited"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"label": "boot-config"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetInstanceConfig(t.Context(), 123, 456)

	require.NoError(t, err, "read-only GetInstanceConfig should retry transient failures")
	assert.Equal(t, "boot-config", got.Label)
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

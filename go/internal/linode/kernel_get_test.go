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

func TestClientGetKernelSuccess(t *testing.T) {
	t.Parallel()

	kernel := linode.Kernel{ID: configKernelLatest, Label: "Latest 64 bit", Version: "6.8.9"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/kernels/linode/latest-64bit", r.URL.Path, "request path should include decoded kernel ID")
		assert.Equal(t, "/linode/kernels/linode%2Flatest-64bit", r.URL.EscapedPath(), "kernel ID should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(kernel))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetKernel(t.Context(), configKernelLatest)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, configKernelLatest, result.ID)
	assert.Equal(t, "Latest 64 bit", result.Label)
}

func TestClientGetKernelError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetKernel(t.Context(), configKernelLatest)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientGetKernelRetriesReadOnlyRoute(t *testing.T) {
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Kernel{ID: configKernelLatest}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetKernel(t.Context(), configKernelLatest)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

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

func TestClientGetObjectStorageQuotaUsageSuccess(t *testing.T) {
	t.Parallel()

	used := 10
	usage := linode.ObjectStorageQuotaUsage{QuotaLimit: 100, Usage: &used}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/object-storage/quotas/obj-bucket-us-ord-1/usage", r.URL.Path, "request path should include quota ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(usage))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj-bucket-us-ord-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 100, result.QuotaLimit)
	require.NotNil(t, result.Usage)
	assert.Equal(t, 10, *result.Usage)
}

func TestClientGetObjectStorageQuotaUsageEscapesQuotaID(t *testing.T) {
	t.Parallel()

	used := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/quotas/obj%2Fbucket%3Funsafe/usage", r.URL.EscapedPath(), "quota ID should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "encoded path value should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuotaUsage{QuotaLimit: 2, Usage: &used}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj/bucket?unsafe")

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestClientGetObjectStorageQuotaUsageError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj-bucket-us-ord-1")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientGetObjectStorageQuotaUsageRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	used := 10

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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuotaUsage{QuotaLimit: 100, Usage: &used}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj-bucket-us-ord-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

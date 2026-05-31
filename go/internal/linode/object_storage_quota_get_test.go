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

func TestClientGetObjectStorageQuotaSuccess(t *testing.T) {
	t.Parallel()

	quota := linode.ObjectStorageQuota{keyID: "obj-buckets-us-sea-1.linodeobjects.com", "quota": 250}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/object-storage/quotas/obj-buckets-us-sea-1.linodeobjects.com", r.URL.Path, "request path should match quota endpoint")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(quota))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuota(t.Context(), "obj-buckets-us-sea-1.linodeobjects.com")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "obj-buckets-us-sea-1.linodeobjects.com", (*result)[keyID])
}

func TestClientGetObjectStorageQuotaEscapesQuotaID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/quotas/quota%2F..%2F%3Fx=1", r.URL.EscapedPath(), "quota ID should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "encoded quota ID should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuota{keyID: "quota/../?x=1"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuota(t.Context(), "quota/../?x=1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "quota/../?x=1", (*result)[keyID])
}

func TestClientGetObjectStorageQuotaError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuota(t.Context(), "missing-quota")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientGetObjectStorageQuotaRetriesReadOnlyRoute(t *testing.T) {
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuota{keyID: "retry-quota"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetObjectStorageQuota(t.Context(), "retry-quota")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

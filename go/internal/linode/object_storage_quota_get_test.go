package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetObjectStorageQuotaSuccess(t *testing.T) {
	t.Parallel()

	quota := linode.ObjectStorageQuota{keyID: "obj-buckets-us-sea-1.linodeobjects.com", "quota": 250}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/object-storage/quotas/obj-buckets-us-sea-1.linodeobjects.com", r.URL.Path, "request path should match quota endpoint")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"), "values differ")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(quota), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuota(t.Context(), "obj-buckets-us-sea-1.linodeobjects.com")

	requireNoError(t, err, "expected no error")
	requireNotNil(t, result, "expected non-nil value")
	checkEqual(t, "obj-buckets-us-sea-1.linodeobjects.com", (*result)[keyID], "values differ")
}

func TestClientGetObjectStorageQuotaEscapesQuotaID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/object-storage/quotas/quota%2F..%2F%3Fx=1", r.URL.EscapedPath(), "quota ID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded quota ID should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuota{keyID: "quota/../?x=1"}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuota(t.Context(), "quota/../?x=1")

	requireNoError(t, err, "expected no error")
	requireNotNil(t, result, "expected non-nil value")
	checkEqual(t, "quota/../?x=1", (*result)[keyID], "values differ")
}

func TestClientGetObjectStorageQuotaError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuota(t.Context(), "missing-quota")

	requireError(t, err, "expected error")
	checkNil(t, result, "expected nil")
}

func TestClientGetObjectStorageQuotaRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}), "expected no error")

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuota{keyID: "retry-quota"}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetObjectStorageQuota(t.Context(), "retry-quota")

	requireNoError(t, err, "expected no error")
	requireNotNil(t, result, "expected non-nil value")
	checkEqual(t, int32(2), calls, "read-only GET route may retry transient failures")
}

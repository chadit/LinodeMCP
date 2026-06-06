package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetObjectStorageQuotaUsageSuccess(t *testing.T) {
	t.Parallel()

	used := 10
	usage := linode.ObjectStorageQuotaUsage{QuotaLimit: 100, Usage: &used}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/object-storage/quotas/obj-bucket-us-ord-1/usage", r.URL.Path, "request path should include quota ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"), "values differ")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(usage), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj-bucket-us-ord-1")

	requireNoError(t, err, "expected no error")
	requireNotNil(t, result, "expected non-nil value")
	checkEqual(t, 100, result.QuotaLimit, "values differ")
	requireNotNil(t, result.Usage, "expected non-nil value")
	checkEqual(t, 10, *result.Usage, "values differ")
}

func TestClientGetObjectStorageQuotaUsageEscapesQuotaID(t *testing.T) {
	t.Parallel()

	used := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/object-storage/quotas/obj%2Fbucket%3Funsafe/usage", r.URL.EscapedPath(), "quota ID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded path value should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuotaUsage{QuotaLimit: 2, Usage: &used}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj/bucket?unsafe")

	requireNoError(t, err, "expected no error")
	requireNotNil(t, result, "expected non-nil value")
}

func TestClientGetObjectStorageQuotaUsageError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj-bucket-us-ord-1")

	requireError(t, err, "expected error")
	checkNil(t, result, "expected nil")
}

func TestClientGetObjectStorageQuotaUsageRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	used := 10

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
		checkNoError(t, json.NewEncoder(w).Encode(linode.ObjectStorageQuotaUsage{QuotaLimit: 100, Usage: &used}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetObjectStorageQuotaUsage(t.Context(), "obj-bucket-us-ord-1")

	requireNoError(t, err, "expected no error")
	requireNotNil(t, result, "expected non-nil value")
	checkEqual(t, int32(2), calls, "read-only GET route may retry transient failures")
}

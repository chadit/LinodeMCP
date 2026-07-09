package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func objectStorageQuotaBody(quotaID string) map[string]any {
	return map[string]any{
		"quota_id":        quotaID,
		"quota_name":      "Number of Objects",
		"endpoint_type":   "E1",
		"s3_endpoint":     "us-sea-1.linodeobjects.com",
		"description":     "Maximum number of objects this endpoint can store",
		"quota_limit":     1000000000,
		"resource_metric": "object",
	}
}

func TestClientGetObjectStorageQuotaProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas/obj-buckets-us-sea-1.linodeobjects.com" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas/obj-buckets-us-sea-1.linodeobjects.com")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(objectStorageQuotaBody("obj-buckets-us-sea-1.linodeobjects.com")); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetObjectStorageQuotaProto(t.Context(), "obj-buckets-us-sea-1.linodeobjects.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetQuotaId() != "obj-buckets-us-sea-1.linodeobjects.com" {
		t.Errorf("result.GetQuotaId() = %v, want %v", result.GetQuotaId(), "obj-buckets-us-sea-1.linodeobjects.com")
	}

	if result.GetQuotaLimit() != int64(1000000000) {
		t.Errorf("result.GetQuotaLimit() = %v, want %v", result.GetQuotaLimit(), int64(1000000000))
	}
}

func TestClientGetObjectStorageQuotaProtoEscapesQuotaID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/object-storage/quotas/quota%2F..%2F%3Fx=1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/object-storage/quotas/quota%2F..%2F%3Fx=1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(objectStorageQuotaBody("quota/../?x=1")); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetObjectStorageQuotaProto(t.Context(), "quota/../?x=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetQuotaId() != "quota/../?x=1" {
		t.Errorf("result.GetQuotaId() = %v, want %v", result.GetQuotaId(), "quota/../?x=1")
	}
}

func TestClientGetObjectStorageQuotaProtoError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetObjectStorageQuotaProto(t.Context(), "missing-quota")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientGetObjectStorageQuotaProtoRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(objectStorageQuotaBody("retry-quota")); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetObjectStorageQuotaProto(t.Context(), "retry-quota")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls != int32(2) {
		t.Errorf("calls = %v, want %v", calls, int32(2))
	}
}

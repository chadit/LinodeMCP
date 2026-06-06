package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetLKEVersionSuccess(t *testing.T) {
	t.Parallel()

	version := linode.LKEVersion{ID: lkeVersion129}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/lke/versions/1.29" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/versions/1.29")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("request query = %q, want empty", r.URL.RawQuery)
		}

		if got := r.Header.Get("Authorization"); got != lkeAuthHeader {
			t.Errorf("Authorization header = %q, want %q", got, lkeAuthHeader)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(version); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(0))

	result, err := client.GetLKEVersion(t.Context(), lkeVersion129)
	if err != nil {
		t.Fatalf("GetLKEVersion returned error: %v", err)
	}

	if result == nil {
		t.Fatal("GetLKEVersion result is nil")
	}

	if result.ID != lkeVersion129 {
		t.Fatalf("result ID = %q, want %q", result.ID, lkeVersion129)
	}
}

func TestClientGetLKEVersionEscapesPathSegment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/versions/1.29/edge" {
			t.Errorf("decoded path = %q, want %q", r.URL.Path, "/lke/versions/1.29/edge")
		}

		if got := r.URL.EscapedPath(); got != "/lke/versions/1.29%2Fedge" {
			t.Errorf("escaped path = %q, want %q", got, "/lke/versions/1.29%2Fedge")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.LKEVersion{ID: lkeVersionWithSlash}); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(0))

	result, err := client.GetLKEVersion(t.Context(), lkeVersionWithSlash)
	if err != nil {
		t.Fatalf("GetLKEVersion returned error: %v", err)
	}

	if result == nil {
		t.Fatal("GetLKEVersion result is nil")
	}

	if result.ID != lkeVersionWithSlash {
		t.Fatalf("result ID = %q, want %q", result.ID, lkeVersionWithSlash)
	}
}

func TestClientGetLKEVersionError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("encoding error response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(0))

	result, err := client.GetLKEVersion(t.Context(), lkeVersion129)
	if err == nil {
		t.Fatal("expected GetLKEVersion to return error")
	}

	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func TestClientGetLKEVersionRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("encoding error response failed: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.LKEVersion{ID: lkeVersion129}); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(1))

	result, err := client.GetLKEVersion(t.Context(), lkeVersion129)
	if err != nil {
		t.Fatalf("GetLKEVersion returned error: %v", err)
	}

	if result == nil {
		t.Fatal("GetLKEVersion result is nil")
	}

	if calls != 2 {
		t.Fatalf("read-only GET calls = %d, want 2", calls)
	}
}

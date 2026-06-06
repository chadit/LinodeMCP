package linode_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteLKEServiceTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/servicetoken" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/clusters/123/servicetoken")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodDelete)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body failed: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("DELETE service token request body = %q, want empty", string(body))
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(2))

	err := client.DeleteLKEServiceToken(t.Context(), 123)
	if err != nil {
		t.Fatalf("DeleteLKEServiceToken returned error: %v", err)
	}
}

func TestClientDeleteLKEServiceTokenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/lke/clusters/123/servicetoken" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/clusters/123/servicetoken")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodDelete)
		}

		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(2))

	err := client.DeleteLKEServiceToken(t.Context(), 123)
	if err == nil {
		t.Fatal("expected DeleteLKEServiceToken to return transient error")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("DELETE service token calls = %d, want 1", got)
	}
}

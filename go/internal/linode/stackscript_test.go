package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientDeleteStackScript(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/stackscripts/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/stackscripts/456")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil)

	err := client.DeleteStackScript(t.Context(), 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteStackScriptDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/linode/stackscripts/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/stackscripts/456")
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteStackScript(t.Context(), 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	lkeTestToken      = "test-token"
	lkeAuthHeader     = "Bearer " + lkeTestToken
	lkeTierVersion131 = "1.31"
)

func TestDeleteLKEControlPlaneACLSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/lke/clusters/123/control_plane_acl" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/clusters/123/control_plane_acl")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("request query = %q, want empty", r.URL.RawQuery)
		}

		if got := r.Header.Get("Authorization"); got != lkeAuthHeader {
			t.Errorf("Authorization header = %q, want %q", got, lkeAuthHeader)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil)

	err := client.DeleteLKEControlPlaneACL(t.Context(), 123)
	if err != nil {
		t.Fatalf("delete LKE control plane ACL returned error: %v", err)
	}
}

func TestDeleteLKEControlPlaneACLDoesNotRetryTransientServerError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/lke/clusters/123/control_plane_acl" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/clusters/123/control_plane_acl")
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(2))

	err := client.DeleteLKEControlPlaneACL(t.Context(), 123)
	if err == nil {
		t.Fatal("expected transient server error")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("destructive ACL delete calls = %d, want 1", got)
	}
}

func TestDeleteLKEControlPlaneACLOpenCircuitShortCircuitsWithoutUpstreamCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"down"}]}`)); err != nil {
			t.Errorf("writing error response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			MaxRetries:              2,
			BaseRetryDelay:          time.Millisecond,
			MaxRetryDelay:           time.Millisecond,
			CircuitBreakerThreshold: 1,
			CircuitBreakerTimeout:   time.Hour,
		},
	}
	client := linode.NewClient(srv.URL, lkeTestToken, cfg)

	err := client.DeleteLKEControlPlaneACL(t.Context(), 123)
	if err == nil {
		t.Fatal("expected first server error")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("destructive ACL delete calls = %d, want 1", got)
	}

	err = client.DeleteLKEControlPlaneACL(t.Context(), 123)
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("open breaker error = %v, want %v", err, linode.ErrCircuitOpen)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("open breaker upstream calls = %d, want 1", got)
	}
}

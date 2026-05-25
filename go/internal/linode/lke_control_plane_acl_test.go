package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestDeleteLKEControlPlaneACL(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/lke/clusters/123/control_plane_acl", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil)
		err := client.DeleteLKEControlPlaneACL(t.Context(), 123)

		require.NoError(t, err, "delete LKE control plane ACL should succeed")
	})

	t.Run("does not retry transient server error", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			assert.Equal(t, "/lke/clusters/123/control_plane_acl", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
		err := client.DeleteLKEControlPlaneACL(t.Context(), 123)

		require.Error(t, err, "transient server error should be returned")
		assert.Equal(t, int32(1), calls.Load(), "destructive ACL delete must not be replayed")
	})

	t.Run("open circuit short-circuits without upstream call", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"down"}]}`))
			assert.NoError(t, writeErr, "writing error response should not fail")
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
		client := linode.NewClient(srv.URL, "test-token", cfg)

		err := client.DeleteLKEControlPlaneACL(t.Context(), 123)
		require.Error(t, err, "first server error should be returned")
		assert.Equal(t, int32(1), calls.Load(), "destructive ACL delete must not retry")

		err = client.DeleteLKEControlPlaneACL(t.Context(), 123)
		require.ErrorIs(t, err, linode.ErrCircuitOpen, "open breaker rejects without upstream call")
		assert.Equal(t, int32(1), calls.Load(), "open breaker must not invoke upstream")
	})
}

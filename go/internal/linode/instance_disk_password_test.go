package linode_test

import (
	"encoding/json"
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

func TestResetInstanceDiskPassword(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/linode/instances/123/disks/456/password", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			var body map[string]string
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Equal(t, "Str0ngP@ssw0rd!", body["password"], "password body should match")
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil)
		err := client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")

		require.NoError(t, err, "reset disk password should succeed")
	})

	t.Run("does not retry transient server error", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			assert.Equal(t, "/linode/instances/123/disks/456/password", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
		err := client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")

		require.Error(t, err, "transient server error should be returned")
		assert.Equal(t, int32(1), calls.Load(), "credential mutation must not be replayed")
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

		err := client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")
		require.Error(t, err, "first server error should be returned")
		assert.Equal(t, int32(1), calls.Load(), "credential mutation must not retry")

		err = client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")
		require.ErrorIs(t, err, linode.ErrCircuitOpen, "open breaker rejects without upstream call")
		assert.Equal(t, int32(1), calls.Load(), "open breaker must not invoke upstream")
	})

	t.Run("validates path ids", func(t *testing.T) {
		t.Parallel()

		client := linode.NewClient("https://api.linode.com/v4", "test-token", nil)
		require.ErrorIs(t, client.ResetInstanceDiskPassword(t.Context(), 0, 456, "Str0ngP@ssw0rd!"), linode.ErrLinodeIDPositive)
		require.ErrorIs(t, client.ResetInstanceDiskPassword(t.Context(), 123, 0, "Str0ngP@ssw0rd!"), linode.ErrDiskIDPositive)
	})
}

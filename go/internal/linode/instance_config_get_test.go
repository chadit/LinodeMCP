package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

// The check* helpers used here are package-local, stdlib-backed test helpers
// from image_assertions_test.go. check* reports and continues for HTTP
// handlers; require* stops the current test goroutine after client calls.

func TestClientGetInstanceConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:    float64(456),
			keyLabel: "boot-config",
			"kernel": configKernelLatest,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetInstanceConfig(t.Context(), 123, 456)

	requireNoError(t, err, "GetInstanceConfig should succeed on 200 response")
	checkEqual(t, "boot-config", got.Label)
	checkEqual(t, configKernelLatest, got.Kernel)
}

func TestClientGetInstanceConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetInstanceConfig(t.Context(), 123, 456)

	requireError(t, err)

	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetInstanceConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		checkTrue(t, false, "invalid IDs should be rejected before issuing HTTP request")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	t.Run("invalid linode id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetInstanceConfig(t.Context(), 0, 456)

		requireErrorIs(t, err, linode.ErrLinodeIDPositive)
	})

	t.Run("invalid config id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetInstanceConfig(t.Context(), 123, 0)

		requireErrorIs(t, err, linode.ErrConfigIDPositive)
	})
}

func TestClientGetInstanceConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "rate limited"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{"label": "boot-config"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetInstanceConfig(t.Context(), 123, 456)

	requireNoError(t, err, "read-only GetInstanceConfig should retry transient failures")
	checkEqual(t, "boot-config", got.Label)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

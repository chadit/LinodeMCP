package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedStatsPath      = "/managed/stats"
	managedStatsForbidden = "Forbidden"
	managedStatsCPUKey    = "cpu"
)

func TestClientGetManagedStatsSuccess(t *testing.T) {
	t.Parallel()

	stats := map[string]any{
		"monitoring": map[string]any{
			managedStatsCPUKey: float64(1),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedStatsPath, r.URL.Path, "request path should be /managed/stats")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(stats))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedStats(t.Context())

	requireNoError(t, err, "GetManagedStats should succeed on 200 response")
	requireNotNil(t, result)
	monitoring, ok := result["monitoring"].(map[string]any)
	requireTrue(t, ok, "monitoring stats should be decoded")
	cpu, ok := monitoring[managedStatsCPUKey].(float64)
	requireTrue(t, ok, "monitoring CPU stat should be decoded")
	checkInEpsilon(t, float64(1), cpu, 0.001)
}

func TestClientGetManagedStatsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedStatsPath, r.URL.Path, "request path should be /managed/stats")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{"monitoring": map[string]any{managedStatsCPUKey: 1}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.GetManagedStats(t.Context())

	requireNoError(t, err, "read-only Managed stats get should retry transient failures")
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientGetManagedStatsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedStatsPath, r.URL.Path, "request path should be /managed/stats")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedStatsForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetManagedStats(t.Context())

	requireError(t, err, "GetManagedStats should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

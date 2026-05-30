package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func writeNodeBalancerStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(`{
		"title":"nodebalancer.example.com (nodebalancer123) - day (5 min avg)",
		"connections":[[1521483600000,12.5]],
		"traffic":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]]}
	}`))
	assert.NoError(t, err, "writing stats fixture should not fail")
}

func TestClientGetNodeBalancerStatsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/nodebalancers/444/stats", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "stats request should not send a body")
		writeNodeBalancerStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerStats(t.Context(), 444)

	require.NoError(t, err, "GetNodeBalancerStats should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, "nodebalancer.example.com (nodebalancer123) - day (5 min avg)", got.Title)
	assert.Equal(t, [][]float64{{1521483600000, 12.5}}, got.Connections)
	assert.Equal(t, [][]float64{{1521484800000, 2004.36}}, got.Traffic.In)
	assert.Equal(t, [][]float64{{1521484800000, 3928.91}}, got.Traffic.Out)
}

func TestClientGetNodeBalancerStatsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/nodebalancers/444/stats", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNodeBalancerStats(t.Context(), 444)

	require.Error(t, err, "GetNodeBalancerStats should fail on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetNodeBalancerStatsRejectsInvalidPathParam(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNodeBalancerStats(t.Context(), 0)
	require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

	_, err = client.GetNodeBalancerStats(t.Context(), -1)
	require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

	assert.False(t, called.Load(), "invalid ID should not issue HTTP requests")
}

func TestClientGetNodeBalancerStatsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/nodebalancers/444/stats", r.URL.Path, "request path should match")

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		writeNodeBalancerStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetNodeBalancerStats(t.Context(), 444)

	require.NoError(t, err, "GetNodeBalancerStats should retry transient failures")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, int32(2), requestCount.Load(), "read-only stats route should retry once before success")
	assert.Equal(t, "nodebalancer.example.com (nodebalancer123) - day (5 min avg)", got.Title)
}

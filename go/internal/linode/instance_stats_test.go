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

func writeInstanceStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(`{
		"title":"linode.com - my-linode (linode123456) - day (5 min avg)",
		"cpu":[[1521483600000,0.42]],
		"io":{"io":[[1521484800000,0.19]],"swap":[[1521484800000,0]]},
		"netv4":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]],"private_in":[[1521484800000,0]],"private_out":[[1521484800000,5.6]]},
		"netv6":{"in":[[1521484800000,0]],"out":[[1521484800000,0]],"private_in":[[1521484800000,195.18]],"private_out":[[1521484800000,5.6]]}
	}`))
	assert.NoError(t, err, "writing stats fixture should not fail")
}

func TestClientGetInstanceStatsSuccess(t *testing.T) {
	t.Parallel()

	want := linode.InstanceStats{
		Title: "linode.com - my-linode (linode123456) - day (5 min avg)",
		CPU:   [][]float64{{1521483600000, 0.42}},
		IO: linode.InstanceIOStats{
			IO:   [][]float64{{1521484800000, 0.19}},
			Swap: [][]float64{{1521484800000, 0}},
		},
		NetV4: linode.InstanceNetV4Stats{
			In:         [][]float64{{1521484800000, 2004.36}},
			Out:        [][]float64{{1521484800000, 3928.91}},
			PrivateIn:  [][]float64{{1521484800000, 0}},
			PrivateOut: [][]float64{{1521484800000, 5.6}},
		},
		NetV6: linode.InstanceNetV6Stats{
			In:         [][]float64{{1521484800000, 0}},
			Out:        [][]float64{{1521484800000, 0}},
			PrivateIn:  [][]float64{{1521484800000, 195.18}},
			PrivateOut: [][]float64{{1521484800000, 5.6}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/stats", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "stats request should not send a body")
		writeInstanceStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetInstanceStats(t.Context(), 123)

	require.NoError(t, err, "GetInstanceStats should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want.Title, got.Title)
	assert.Equal(t, want.CPU, got.CPU)
	assert.Equal(t, want.IO, got.IO)
	assert.Equal(t, want.NetV4, got.NetV4)
	assert.Equal(t, want.NetV6, got.NetV6)
}

func TestClientGetInstanceStatsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/stats", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceStats(t.Context(), 123)

	require.Error(t, err, "GetInstanceStats should fail on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetInstanceStatsRetriesTransientError(t *testing.T) {
	t.Parallel()

	want := linode.InstanceStats{Title: "linode.com - my-linode (linode123456) - day (5 min avg)"}

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/stats", r.URL.Path, "request path should match")

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		writeInstanceStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetInstanceStats(t.Context(), 123)

	require.NoError(t, err, "GetInstanceStats should retry transient failures")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want.Title, got.Title)
	assert.Equal(t, int32(2), requestCount.Load(), "stats read should retry once before success")
}

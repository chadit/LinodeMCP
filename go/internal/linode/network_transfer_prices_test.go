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

const endpointNetworkTransferPrices = "/network-transfer/prices"

func TestClientListNetworkTransferPricesSuccess(t *testing.T) {
	t.Parallel()

	prices := linode.PaginatedResponse[linode.NetworkTransferPrice]{
		Data: []linode.NetworkTransferPrice{{
			ID:       "network_transfer",
			Label:    "Network Transfer",
			Price:    linode.Price{Hourly: 0.005},
			Transfer: 0,
			RegionPrices: []linode.NetworkTransferRegionPrice{{
				ID:      managedServiceRegion,
				Hourly:  0.015,
				Monthly: 0,
			}},
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkTransferPrices, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(prices))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListNetworkTransferPrices(t.Context())

	require.NoError(t, err, "ListNetworkTransferPrices should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, "network_transfer", result.Data[0].ID)
	assert.Equal(t, "Network Transfer", result.Data[0].Label)
	assert.InEpsilon(t, 0.005, result.Data[0].Price.Hourly, 0.0001)
	require.Len(t, result.Data[0].RegionPrices, 1)
	assert.Equal(t, managedServiceRegion, result.Data[0].RegionPrices[0].ID)
}

func TestClientListNetworkTransferPricesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkTransferPrices, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListNetworkTransferPrices(t.Context())

	require.Error(t, err, "ListNetworkTransferPrices should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListNetworkTransferPricesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hj, ok := w.(http.Hijacker)
			if !assert.True(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !assert.NoError(t, err) {
				return
			}

			assert.NoError(t, conn.Close())

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkTransferPrices, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.NetworkTransferPrice]{
			Data: []linode.NetworkTransferPrice{{ID: "network_transfer"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListNetworkTransferPrices(t.Context())

	require.NoError(t, err, "ListNetworkTransferPrices should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, "network_transfer", result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

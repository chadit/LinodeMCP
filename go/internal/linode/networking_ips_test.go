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

const (
	endpointNetworkingIPs      = "/networking/ips"
	networkingIPv4Type         = "ipv4"
	networkingIPAddressFixture = "198.51.100.5"
)

func TestClientListNetworkingIPsSuccess(t *testing.T) {
	t.Parallel()

	ips := linode.PaginatedResponse[linode.IPAddress]{
		Data: []linode.IPAddress{{
			Address:  networkingIPAddressFixture,
			Gateway:  "198.51.100.1",
			Type:     networkingIPv4Type,
			Public:   true,
			RDNS:     "example.test",
			LinodeID: 123,
			Region:   regionUSEast,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPs, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(ips))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListNetworkingIPs(t.Context(), false)

	require.NoError(t, err, "ListNetworkingIPs should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, networkingIPAddressFixture, result.Data[0].Address)
	assert.Equal(t, regionUSEast, result.Data[0].Region)
}

func TestClientListNetworkingIPsWithSkipIPv6RDNSQuery(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPs, r.URL.Path, "request path should match")
		assert.Equal(t, "true", r.URL.Query().Get("skip_ipv6_rdns"), "skip_ipv6_rdns query should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.IPAddress]{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListNetworkingIPs(t.Context(), true)

	require.NoError(t, err, "ListNetworkingIPs should succeed with skip_ipv6_rdns")
}

func TestClientListNetworkingIPsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPs, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListNetworkingIPs(t.Context(), false)

	require.Error(t, err, "ListNetworkingIPs should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListNetworkingIPsRetriesTransientError(t *testing.T) {
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
		assert.Equal(t, endpointNetworkingIPs, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.IPAddress]{
			Data: []linode.IPAddress{{Address: networkingIPAddressFixture}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListNetworkingIPs(t.Context(), false)

	require.NoError(t, err, "ListNetworkingIPs should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, networkingIPAddressFixture, result.Data[0].Address)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

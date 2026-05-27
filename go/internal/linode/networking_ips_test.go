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
	endpointNetworkingIPs         = "/networking/ips"
	endpointNetworkingIPsAssign   = endpointNetworkingIPs + "/assign"
	endpointNetworkingIPsShare    = "/networking/ipv4/share"
	endpointNetworkingIPv4Assign  = "/networking/ipv4/assign"
	networkingIPv4Type            = "ipv4"
	networkingIPAddressFixture    = "198.51.100.5"
	networkingRDNSFixture         = "host.example.test"
	networkingIPv6AddressFixture  = "2001:db8::1"
	networkingScopedIPv6Fixture   = "fe80::1%eth0"
	networkingZoneTraversalValue  = "fe80::1%../../x?y=1"
	endpointNetworkingIPAddress   = endpointNetworkingIPs + "/" + networkingIPAddressFixture
	endpointNetworkingIPv6Escaped = endpointNetworkingIPs + "/2001:db8::1"
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

func TestClientGetNetworkingIPSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPAddress, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{
			Address:  networkingIPAddressFixture,
			Gateway:  "198.51.100.1",
			Type:     networkingIPv4Type,
			Public:   true,
			Region:   regionUSEast,
			LinodeID: 123,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetNetworkingIP(t.Context(), networkingIPAddressFixture)

	require.NoError(t, err, "GetNetworkingIP should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, networkingIPAddressFixture, result.Address)
	assert.Equal(t, regionUSEast, result.Region)
}

func TestClientGetNetworkingIPEncodesIPv6Address(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPv6Escaped, r.URL.EscapedPath(), "request path should preserve valid IPv6 segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPv6AddressFixture}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetNetworkingIP(t.Context(), networkingIPv6AddressFixture)

	require.NoError(t, err, "GetNetworkingIP should succeed for IPv6 address")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, networkingIPv6AddressFixture, result.Address)
}

func TestClientGetNetworkingIPAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPAddress, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNetworkingIP(t.Context(), networkingIPAddressFixture)

	require.Error(t, err, "GetNetworkingIP should fail on 404 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestClientGetNetworkingIPRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	for _, address := range []string{"", "198.51.100.5/24", "198.51.100.5?bad=1", "..", networkingScopedIPv6Fixture, networkingZoneTraversalValue} {
		_, err := client.GetNetworkingIP(t.Context(), address)
		require.Error(t, err, "invalid address should be rejected")
	}
}

func TestClientGetNetworkingIPRetriesTransientError(t *testing.T) {
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
		assert.Equal(t, endpointNetworkingIPAddress, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPAddressFixture}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetNetworkingIP(t.Context(), networkingIPAddressFixture)

	require.NoError(t, err, "GetNetworkingIP should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, networkingIPAddressFixture, result.Address)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientUpdateNetworkingIPSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, endpointNetworkingIPAddress, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body linode.UpdateNetworkingIPRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, networkingRDNSFixture, body.RDNS)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{
			Address: networkingIPAddressFixture,
			RDNS:    networkingRDNSFixture,
			Type:    networkingIPv4Type,
			Public:  true,
			Region:  regionUSEast,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})

	require.NoError(t, err, "UpdateNetworkingIP should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, networkingIPAddressFixture, result.Address)
	assert.Equal(t, networkingRDNSFixture, result.RDNS)
}

func TestClientUpdateNetworkingIPAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, endpointNetworkingIPAddress, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"invalid rdns"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})

	require.Error(t, err, "UpdateNetworkingIP should fail on 400 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientUpdateNetworkingIPEncodesIPv6Address(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, endpointNetworkingIPv6Escaped, r.URL.EscapedPath(), "request path should preserve valid IPv6 segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPv6AddressFixture}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateNetworkingIP(t.Context(), networkingIPv6AddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})

	require.NoError(t, err, "UpdateNetworkingIP should succeed for IPv6 address")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, networkingIPv6AddressFixture, result.Address)
}

func TestClientUpdateNetworkingIPDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})

	require.Error(t, err, "UpdateNetworkingIP should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating PUT must not be replayed")
}

func TestClientUpdateNetworkingIPRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNetworkingIP(t.Context(), "", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	require.Error(t, err, "blank address should be rejected")

	_, err = client.UpdateNetworkingIP(t.Context(), "198.51.100.5/24", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	require.Error(t, err, "slash address should be rejected")

	_, err = client.UpdateNetworkingIP(t.Context(), "198.51.100.5?bad=1", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	require.Error(t, err, "query separator address should be rejected")

	_, err = client.UpdateNetworkingIP(t.Context(), "..", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	require.Error(t, err, "dot traversal address should be rejected")

	_, err = client.UpdateNetworkingIP(t.Context(), networkingScopedIPv6Fixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	require.Error(t, err, "scoped IPv6 address should be rejected")

	_, err = client.UpdateNetworkingIP(t.Context(), networkingZoneTraversalValue, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	require.Error(t, err, "zone traversal address should be rejected")

	_, err = client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{})
	require.ErrorIs(t, err, linode.ErrRDNSRequired)
}

func TestClientAllocateNetworkingIPSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPs, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body linode.AllocateNetworkingIPRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, 123, body.LinodeID)
		assert.True(t, body.Public)
		assert.Equal(t, networkingIPv4Type, body.Type)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
			Public:   true,
			Type:     networkingIPv4Type,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		LinodeID: 123,
		Public:   true,
		Type:     networkingIPv4Type,
	})

	require.NoError(t, err, "AllocateNetworkingIP should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, networkingIPAddressFixture, result.Address)
	assert.Equal(t, 123, result.LinodeID)
}

func TestClientAllocateNetworkingIPAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPs, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		LinodeID: 123,
		Public:   true,
		Type:     networkingIPv4Type,
	})

	require.Error(t, err, "AllocateNetworkingIP should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientAllocateNetworkingIPDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		LinodeID: 123,
		Public:   true,
		Type:     networkingIPv4Type,
	})

	require.Error(t, err, "AllocateNetworkingIP should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent POST must not be replayed")
}

func TestClientAssignNetworkingIPsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPsAssign, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body linode.AssignNetworkingIPsRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, regionUSEast, body.Region)
		assert.Len(t, body.Assignments, 1)
		assert.Equal(t, networkingIPAddressFixture, body.Assignments[0].Address)
		assert.Equal(t, 123, body.Assignments[0].LinodeID)

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
		}},
	})

	require.NoError(t, err, "AssignNetworkingIPs should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
}

func TestClientAssignNetworkingIPsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPsAssign, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
		}},
	})

	require.Error(t, err, "AssignNetworkingIPs should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientAssignNetworkingIPsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
		}},
	})

	require.Error(t, err, "AssignNetworkingIPs should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent POST must not be replayed")
}

func TestClientAssignNetworkingIPv4sSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPv4Assign, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body linode.AssignNetworkingIPsRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, regionUSEast, body.Region)
		assert.Len(t, body.Assignments, 1)
		assert.Equal(t, networkingIPAddressFixture, body.Assignments[0].Address)
		assert.Equal(t, 123, body.Assignments[0].LinodeID)

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
		}},
	})

	require.NoError(t, err, "AssignNetworkingIPv4s should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
}

func TestClientAssignNetworkingIPv4sAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPv4Assign, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
		}},
	})

	require.Error(t, err, "AssignNetworkingIPv4s should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientAssignNetworkingIPv4sDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
		}},
	})

	require.Error(t, err, "AssignNetworkingIPv4s should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent POST must not be replayed")
}

func TestClientAssignNetworkingIPv4sRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{})
	require.ErrorIs(t, err, linode.ErrRegionRequired)

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{Region: regionUSEast})
	require.ErrorIs(t, err, linode.ErrIPAssignmentsRequired)

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{LinodeID: 123}},
	})
	require.ErrorIs(t, err, linode.ErrIPAddressRequired)

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{Address: networkingIPAddressFixture}},
	})
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPv6AddressFixture,
			LinodeID: 123,
		}},
	})
	require.ErrorIs(t, err, linode.ErrIPv4AddressInvalid)

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  "not-an-ip",
			LinodeID: 123,
		}},
	})
	require.ErrorIs(t, err, linode.ErrIPv4AddressInvalid)
}

func TestClientShareNetworkingIPsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPsShare, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body linode.ShareNetworkingIPsRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, 123, body.LinodeID)

		if !assert.Len(t, body.IPs, 1) {
			return
		}

		assert.Equal(t, networkingIPAddressFixture, body.IPs[0])

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{networkingIPAddressFixture},
	})

	require.NoError(t, err, "ShareNetworkingIPs should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
}

func TestClientShareNetworkingIPsAcceptsEmptyList(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPsShare, r.URL.Path, "request path should match")

		var body linode.ShareNetworkingIPsRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, 123, body.LinodeID)
		assert.Empty(t, body.IPs, "empty ips array removes all shared addresses and should pass through")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{},
	})

	require.NoError(t, err, "ShareNetworkingIPs should accept an empty ips array")
	require.NotNil(t, result, "result should not be nil")
}

func TestClientShareNetworkingIPsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPsShare, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{networkingIPAddressFixture},
	})

	require.Error(t, err, "ShareNetworkingIPs should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientShareNetworkingIPsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{networkingIPAddressFixture},
	})

	require.Error(t, err, "ShareNetworkingIPs should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent POST must not be replayed")
}

func TestClientShareNetworkingIPsRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{})
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)

	_, err = client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{LinodeID: 123})
	require.ErrorIs(t, err, linode.ErrIPAddressRequired)

	_, err = client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{""},
	})
	require.ErrorIs(t, err, linode.ErrIPAddressRequired)
}

func TestClientAssignNetworkingIPsRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{})
	require.ErrorIs(t, err, linode.ErrRegionRequired)

	_, err = client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{Region: regionUSEast})
	require.ErrorIs(t, err, linode.ErrIPAssignmentsRequired)

	_, err = client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{LinodeID: 123}},
	})
	require.ErrorIs(t, err, linode.ErrIPAddressRequired)

	_, err = client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{Address: networkingIPAddressFixture}},
	})
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)
}

func TestClientAllocateNetworkingIPRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		Type: networkingIPv4Type,
	})

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)
}

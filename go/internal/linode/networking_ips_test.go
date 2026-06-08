package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPs)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(ips); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListNetworkingIPs(t.Context(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Address != networkingIPAddressFixture {
		t.Errorf("result.Data[0].Address = %v, want %v", result.Data[0].Address, networkingIPAddressFixture)
	}

	if result.Data[0].Region != regionUSEast {
		t.Errorf("result.Data[0].Region = %v, want %v", result.Data[0].Region, regionUSEast)
	}
}

func TestClientListNetworkingIPsWithSkipIPv6RDNSQuery(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPs)
		}

		if r.URL.Query().Get("skip_ipv6_rdns") != "true" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("skip_ipv6_rdns"), "true")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.IPAddress]{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListNetworkingIPs(t.Context(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientListNetworkingIPsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPs)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListNetworkingIPs(t.Context(), false)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientListNetworkingIPsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPs)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.IPAddress]{
			Data: []linode.IPAddress{{Address: networkingIPAddressFixture}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListNetworkingIPs(t.Context(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Address != networkingIPAddressFixture {
		t.Errorf("result.Data[0].Address = %v, want %v", result.Data[0].Address, networkingIPAddressFixture)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetNetworkingIPSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPAddress {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPAddress)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{
			Address:  networkingIPAddressFixture,
			Gateway:  "198.51.100.1",
			Type:     networkingIPv4Type,
			Public:   true,
			Region:   regionUSEast,
			LinodeID: 123,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetNetworkingIP(t.Context(), networkingIPAddressFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != networkingIPAddressFixture {
		t.Errorf("result.Address = %v, want %v", result.Address, networkingIPAddressFixture)
	}

	if result.Region != regionUSEast {
		t.Errorf("result.Region = %v, want %v", result.Region, regionUSEast)
	}
}

func TestClientGetNetworkingIPEncodesIPv6Address(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != endpointNetworkingIPv6Escaped {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointNetworkingIPv6Escaped)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPv6AddressFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetNetworkingIP(t.Context(), networkingIPv6AddressFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != networkingIPv6AddressFixture {
		t.Errorf("result.Address = %v, want %v", result.Address, networkingIPv6AddressFixture)
	}
}

func TestClientGetNetworkingIPAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPAddress {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPAddress)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusNotFound)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNetworkingIP(t.Context(), networkingIPAddressFixture)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestClientGetNetworkingIPRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	for _, address := range []string{"", "198.51.100.5/24", "198.51.100.5?bad=1", "..", networkingScopedIPv6Fixture, networkingZoneTraversalValue} {
		_, err := client.GetNetworkingIP(t.Context(), address)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	}
}

func TestClientGetNetworkingIPRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPAddress {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPAddress)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPAddressFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetNetworkingIP(t.Context(), networkingIPAddressFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != networkingIPAddressFixture {
		t.Errorf("result.Address = %v, want %v", result.Address, networkingIPAddressFixture)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientUpdateNetworkingIPSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != endpointNetworkingIPAddress {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPAddress)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.UpdateNetworkingIPRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.RDNS != networkingRDNSFixture {
			t.Errorf("body.RDNS = %v, want %v", body.RDNS, networkingRDNSFixture)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{
			Address: networkingIPAddressFixture,
			RDNS:    networkingRDNSFixture,
			Type:    networkingIPv4Type,
			Public:  true,
			Region:  regionUSEast,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != networkingIPAddressFixture {
		t.Errorf("result.Address = %v, want %v", result.Address, networkingIPAddressFixture)
	}

	if result.RDNS != networkingRDNSFixture {
		t.Errorf("result.RDNS = %v, want %v", result.RDNS, networkingRDNSFixture)
	}
}

func TestClientUpdateNetworkingIPAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != endpointNetworkingIPAddress {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPAddress)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusBadRequest)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"invalid rdns"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestClientUpdateNetworkingIPEncodesIPv6Address(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != endpointNetworkingIPv6Escaped {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointNetworkingIPv6Escaped)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPv6AddressFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateNetworkingIP(t.Context(), networkingIPv6AddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != networkingIPv6AddressFixture {
		t.Errorf("result.Address = %v, want %v", result.Address, networkingIPv6AddressFixture)
	}
}

func TestClientUpdateNetworkingIPDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientUpdateNetworkingIPRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNetworkingIP(t.Context(), "", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, err = client.UpdateNetworkingIP(t.Context(), "198.51.100.5/24", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, err = client.UpdateNetworkingIP(t.Context(), "198.51.100.5?bad=1", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, err = client.UpdateNetworkingIP(t.Context(), "..", linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, err = client.UpdateNetworkingIP(t.Context(), networkingScopedIPv6Fixture, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, err = client.UpdateNetworkingIP(t.Context(), networkingZoneTraversalValue, linode.UpdateNetworkingIPRequest{RDNS: networkingRDNSFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, err = client.UpdateNetworkingIP(t.Context(), networkingIPAddressFixture, linode.UpdateNetworkingIPRequest{})
	if !errors.Is(err, linode.ErrRDNSRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrRDNSRequired)
	}
}

func TestClientAllocateNetworkingIPSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPs)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.AllocateNetworkingIPRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID != 123 {
			t.Errorf("body.LinodeID = %v, want %v", body.LinodeID, 123)
		}

		if !body.Public {
			t.Error("body.Public = false, want true")
		}

		if body.Type != networkingIPv4Type {
			t.Errorf("body.Type = %v, want %v", body.Type, networkingIPv4Type)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{
			Address:  networkingIPAddressFixture,
			LinodeID: 123,
			Public:   true,
			Type:     networkingIPv4Type,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		LinodeID: 123,
		Public:   true,
		Type:     networkingIPv4Type,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != networkingIPAddressFixture {
		t.Errorf("result.Address = %v, want %v", result.Address, networkingIPAddressFixture)
	}

	if result.LinodeID != 123 {
		t.Errorf("result.LinodeID = %v, want %v", result.LinodeID, 123)
	}
}

func TestClientAllocateNetworkingIPAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPs)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		LinodeID: 123,
		Public:   true,
		Type:     networkingIPv4Type,
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientAllocateNetworkingIPDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		LinodeID: 123,
		Public:   true,
		Type:     networkingIPv4Type,
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAssignNetworkingIPsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPsAssign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPsAssign)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.AssignNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.Region != regionUSEast {
			t.Errorf("body.Region = %v, want %v", body.Region, regionUSEast)
		}

		if len(body.Assignments) != 1 {
			t.Errorf("len(body.Assignments) = %d, want %d", len(body.Assignments), 1)
		}

		if body.Assignments[0].Address != networkingIPAddressFixture {
			t.Errorf("body.Assignments[0].Address = %v, want %v", body.Assignments[0].Address, networkingIPAddressFixture)
		}

		if body.Assignments[0].LinodeID != 123 {
			t.Errorf("body.Assignments[0].LinodeID = %v, want %v", body.Assignments[0].LinodeID, 123)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientAssignNetworkingIPsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPsAssign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPsAssign)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
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
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientAssignNetworkingIPsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAssignNetworkingIPv4sSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPv4Assign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv4Assign)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.AssignNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.Region != regionUSEast {
			t.Errorf("body.Region = %v, want %v", body.Region, regionUSEast)
		}

		if len(body.Assignments) != 1 {
			t.Errorf("len(body.Assignments) = %d, want %d", len(body.Assignments), 1)
		}

		if body.Assignments[0].Address != networkingIPAddressFixture {
			t.Errorf("body.Assignments[0].Address = %v, want %v", body.Assignments[0].Address, networkingIPAddressFixture)
		}

		if body.Assignments[0].LinodeID != 123 {
			t.Errorf("body.Assignments[0].LinodeID = %v, want %v", body.Assignments[0].LinodeID, 123)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientAssignNetworkingIPv4sAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPv4Assign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv4Assign)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
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
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientAssignNetworkingIPv4sDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAssignNetworkingIPv4sRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{})
	if !errors.Is(err, linode.ErrRegionRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrRegionRequired)
	}

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{Region: regionUSEast})
	if !errors.Is(err, linode.ErrIPAssignmentsRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPAssignmentsRequired)
	}

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{LinodeID: 123}},
	})
	if !errors.Is(err, linode.ErrIPAddressRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPAddressRequired)
	}

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{Address: networkingIPAddressFixture}},
	})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  networkingIPv6AddressFixture,
			LinodeID: 123,
		}},
	})
	if !errors.Is(err, linode.ErrIPv4AddressInvalid) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPv4AddressInvalid)
	}

	_, err = client.AssignNetworkingIPv4s(t.Context(), linode.AssignNetworkingIPsRequest{
		Region: regionUSEast,
		Assignments: []linode.IPAssignment{{
			Address:  "not-an-ip",
			LinodeID: 123,
		}},
	})
	if !errors.Is(err, linode.ErrIPv4AddressInvalid) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPv4AddressInvalid)
	}
}

func TestClientShareNetworkingIPsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPsShare {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPsShare)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.ShareNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID != 123 {
			t.Errorf("body.LinodeID = %v, want %v", body.LinodeID, 123)
		}

		if len(body.IPs) != 1 {
			t.Errorf("len(body.IPs) = %d, want %d", len(body.IPs), 1)

			return
		}

		if body.IPs[0] != networkingIPAddressFixture {
			t.Errorf("body.IPs[0] = %v, want %v", body.IPs[0], networkingIPAddressFixture)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{networkingIPAddressFixture},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientShareNetworkingIPsAcceptsEmptyList(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPsShare {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPsShare)
		}

		var body linode.ShareNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID != 123 {
			t.Errorf("body.LinodeID = %v, want %v", body.LinodeID, 123)
		}

		if len(body.IPs) != 0 {
			t.Errorf("body.IPs = %v, want empty", body.IPs)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientShareNetworkingIPsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPsShare {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPsShare)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{networkingIPAddressFixture},
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientShareNetworkingIPsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{networkingIPAddressFixture},
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientShareNetworkingIPsRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{LinodeID: 123})
	if !errors.Is(err, linode.ErrIPAddressRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPAddressRequired)
	}

	_, err = client.ShareNetworkingIPs(t.Context(), linode.ShareNetworkingIPsRequest{
		LinodeID: 123,
		IPs:      []string{""},
	})
	if !errors.Is(err, linode.ErrIPAddressRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPAddressRequired)
	}
}

func TestClientAssignNetworkingIPsRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{})
	if !errors.Is(err, linode.ErrRegionRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrRegionRequired)
	}

	_, err = client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{Region: regionUSEast})
	if !errors.Is(err, linode.ErrIPAssignmentsRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPAssignmentsRequired)
	}

	_, err = client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{LinodeID: 123}},
	})
	if !errors.Is(err, linode.ErrIPAddressRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPAddressRequired)
	}

	_, err = client.AssignNetworkingIPs(t.Context(), linode.AssignNetworkingIPsRequest{
		Region:      regionUSEast,
		Assignments: []linode.IPAssignment{{Address: networkingIPAddressFixture}},
	})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}
}

func TestClientAllocateNetworkingIPRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.AllocateNetworkingIP(t.Context(), linode.AllocateNetworkingIPRequest{
		Type: networkingIPv4Type,
	})

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}
}

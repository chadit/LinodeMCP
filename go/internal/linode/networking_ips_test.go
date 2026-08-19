package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	endpointNetworkingIPs         = "/networking/ips"
	networkingIPv4Type            = "ipv4"
	networkingIPAddressFixture    = "198.51.100.5"
	networkingIPv6AddressFixture  = "2001:db8::1"
	networkingScopedIPv6Fixture   = "fe80::1%eth0"
	networkingZoneTraversalValue  = "fe80::1%../../x?y=1"
	endpointNetworkingIPAddress   = endpointNetworkingIPs + "/" + networkingIPAddressFixture
	endpointNetworkingIPv6Escaped = endpointNetworkingIPs + "/2001:db8::1"
)

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

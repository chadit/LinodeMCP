package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListReservedIPsProtoRoute(t *testing.T) {
	t.Parallel()

	const (
		reservedIPAddress = "192.0.2.10"
		responseBody      = `{"data":[{"address":"192.0.2.10","assigned_entity":null,"gateway":null,"interface_id":null,"linode_id":null,"prefix":24,"public":true,"rdns":null,"region":"us-east","reserved":true,"subnet_mask":"255.255.255.0","tags":["prod"],"type":"ipv4","vpc_nat_1_1":null}],"page":2,"pages":3,"results":1}`
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/reserved/ips" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/reserved/ips")
		}

		if r.URL.RawQuery != tcPage2PageSize50 {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, tcPage2PageSize50)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	page, err := client.ListReservedIPsProto(t.Context(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if page == nil {
		t.Fatal("page is nil")
	}

	if len(page.ReservedIPs) != 1 {
		t.Fatalf("len(page.ReservedIPs) = %d, want %d", len(page.ReservedIPs), 1)
	}

	if page.ReservedIPs[0].GetAddress() != reservedIPAddress {
		t.Errorf("page.ReservedIPs[0].GetAddress() = %v, want %v", page.ReservedIPs[0].GetAddress(), reservedIPAddress)
	}

	if len(page.RawReservedIPs) != 1 {
		t.Fatalf("len(page.RawReservedIPs) = %d, want %d", len(page.RawReservedIPs), 1)
	}
}

func TestClientListReservedIPsProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	page, err := client.ListReservedIPsProto(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if page != nil {
		t.Errorf("page = %v, want nil", page)
	}
}

func TestClientListReservedIPsProtoRequestError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient(":", "test-token", nil, linode.WithMaxRetries(0))

	page, err := client.ListReservedIPsProto(t.Context(), 0, 0)
	if _, ok := errors.AsType[*linode.NetworkError](err); !ok {
		t.Fatalf("error type = %T, want *linode.NetworkError", err)
	}

	if page != nil {
		t.Errorf("page = %v, want nil", page)
	}
}

func TestClientListReservedIPsProtoDecodeError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(`{"data":[{"prefix":"not-an-integer"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	page, err := client.ListReservedIPsProto(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if page != nil {
		t.Errorf("page = %v, want nil", page)
	}
}

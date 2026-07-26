package linode_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientGetReservedIPRawPreservesJSONAndRetries(t *testing.T) {
	t.Parallel()

	const (
		address      = "192.0.2.10"
		responseBody = `{"address":"192.0.2.10","assigned_entity":null,"gateway":null,"interface_id":null,"linode_id":null,"prefix":24,"public":true,"rdns":null,"region":"us-east","reserved":true,"subnet_mask":"255.255.255.0","tags":[],"type":"ipv4","vpc_nat_1_1":null}`
	)

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/networking/reserved/ips/"+address {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/networking/reserved/ips/"+address)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithJitter(false),
	)

	reservedIP, err := client.GetReservedIPRaw(t.Context(), address)
	if err != nil {
		t.Fatalf("GetReservedIPRaw() error = %v", err)
	}

	if string(reservedIP) != responseBody {
		t.Errorf("GetReservedIPRaw() = %s, want exact raw JSON %s", reservedIP, responseBody)
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("GET calls = %d, want 2", got)
	}
}

func TestClientGetReservedIPRawErrors(t *testing.T) {
	t.Parallel()

	client := linode.NewClient(":", "test-token", nil, linode.WithMaxRetries(0))

	if _, err := client.GetReservedIPRaw(t.Context(), "2001:db8::1"); !errors.Is(err, linode.ErrIPv4AddressInvalid) {
		t.Errorf("GetReservedIPRaw() error = %v, want ErrIPv4AddressInvalid", err)
	}

	_, err := client.GetReservedIPRaw(t.Context(), "192.0.2.10")

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("GetReservedIPRaw() error type = %T, want *linode.NetworkError", err)
	}

	if networkErr.Operation != "GetReservedIP" {
		t.Errorf("NetworkError.Operation = %q, want %q", networkErr.Operation, "GetReservedIP")
	}
}

func TestClientDeleteReservedIPRoute(t *testing.T) {
	t.Parallel()

	const address = "192.0.2.10"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != "/networking/reserved/ips/192.0.2.10" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/networking/reserved/ips/192.0.2.10")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("request body = %q, want empty", body)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	if err := client.DeleteReservedIP(t.Context(), address); err != nil {
		t.Fatalf("DeleteReservedIP() error = %v", err)
	}
}

func TestClientDeleteReservedIPDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	if err := client.DeleteReservedIP(t.Context(), "192.0.2.10"); err == nil {
		t.Fatal("DeleteReservedIP() error = nil, want transient error")
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("DELETE calls = %d, want 1", got)
	}
}

func TestClientDeleteReservedIPErrors(t *testing.T) {
	t.Parallel()

	client := linode.NewClient(":", "test-token", nil, linode.WithMaxRetries(0))

	if err := client.DeleteReservedIP(t.Context(), "2001:db8::1"); !errors.Is(err, linode.ErrIPv4AddressInvalid) {
		t.Errorf("DeleteReservedIP() error = %v, want ErrIPv4AddressInvalid", err)
	}

	err := client.DeleteReservedIP(t.Context(), "192.0.2.10")

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("DeleteReservedIP() error type = %T, want *linode.NetworkError", err)
	}

	if networkErr.Operation != "DeleteReservedIP" {
		t.Errorf("NetworkError.Operation = %q, want %q", networkErr.Operation, "DeleteReservedIP")
	}
}

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

// TestClientCreateReservedIPRawDoesNotRetry pins the reason create is the one
// reserved-IP route that never retries: the POST allocates a new address, so a
// retried request can reserve a second one and bill for it.
// reservedIPTagFixture is the tag every reserved-IP write test sends.
const reservedIPTagFixture = "prod"

func TestClientCreateReservedIPRawDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.EscapedPath() != "/networking/reserved/ips" {
			t.Errorf("r.URL.EscapedPath() = %v, want the collection route", r.URL.EscapedPath())
		}

		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	if _, err := client.CreateReservedIPRaw(t.Context(), "us-east", nil); err == nil {
		t.Fatal("CreateReservedIPRaw() error = nil, want the API failure")
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("request count = %d, want exactly 1 attempt", got)
	}
}

// TestClientCreateReservedIPRawSendsOnlySuppliedFields proves an absent tag list
// stays off the request rather than going out as an empty array.
func TestClientCreateReservedIPRawSendsOnlySuppliedFields(t *testing.T) {
	t.Parallel()

	const responseBody = `{"address":"192.0.2.10","region":"us-east"}`

	var seen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		seen = string(body)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	if _, err := client.CreateReservedIPRaw(t.Context(), "us-east", nil); err != nil {
		t.Fatalf("CreateReservedIPRaw() error = %v", err)
	}

	if strings.Contains(seen, "tags") {
		t.Errorf("request body = %v, want no tags field when none was supplied", seen)
	}
}

func TestClientCreateReservedIPRawRejectsAnEmptyRegion(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil)

	if _, err := client.CreateReservedIPRaw(t.Context(), "", nil); !errors.Is(err, linode.ErrRegionRequired) {
		t.Errorf("error = %v, want %v", err, linode.ErrRegionRequired)
	}
}

// TestClientUpdateReservedIPRawSendsTheWholeSet proves the update replaces every
// tag: an empty list has to reach the API as an empty array, because omitting it
// would leave the existing tags in place.
func TestClientUpdateReservedIPRawSendsTheWholeSet(t *testing.T) {
	t.Parallel()

	const responseBody = `{"address":"192.0.2.10","tags":[]}`

	var seen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/networking/reserved/ips/192.0.2.10" {
			t.Errorf("r.URL.EscapedPath() = %v, want the address route", r.URL.EscapedPath())
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		seen = string(body)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	if _, err := client.UpdateReservedIPRaw(t.Context(), "192.0.2.10", nil); err != nil {
		t.Fatalf("UpdateReservedIPRaw() error = %v", err)
	}

	if !strings.Contains(seen, `"tags":[]`) {
		t.Errorf("request body = %v, want an explicit empty tag list", seen)
	}
}

func TestClientUpdateReservedIPRawRejectsANonIPv4Address(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil)

	_, err := client.UpdateReservedIPRaw(t.Context(), "2001:db8::10", []string{reservedIPTagFixture})
	if !errors.Is(err, linode.ErrIPv4AddressInvalid) {
		t.Errorf("error = %v, want %v", err, linode.ErrIPv4AddressInvalid)
	}
}

// TestClientListReservedIPTypesProtoRetries proves the pricing read is safe to
// retry, unlike the create that allocates.
func TestClientListReservedIPTypesProtoRetries(t *testing.T) {
	t.Parallel()

	const responseBody = `{"data":[{"id":"ipv4_reserved","label":"IPv4 Reserved","transfer":0}],"page":1,"pages":1,"results":1}`

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/networking/reserved/ips/types" {
			t.Errorf("r.URL.EscapedPath() = %v, want the types route", r.URL.EscapedPath())
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil,
		linode.WithMaxRetries(3), linode.WithBaseDelay(time.Millisecond))

	types, err := client.ListReservedIPTypesProto(t.Context())
	if err != nil {
		t.Fatalf("ListReservedIPTypesProto() error = %v", err)
	}

	if len(types) != 1 || types[0].GetId() != "ipv4_reserved" {
		t.Errorf("types = %v, want the single pricing type", types)
	}
}

func TestClientCreateReservedIPRawSendsSuppliedTags(t *testing.T) {
	t.Parallel()

	var seen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		seen = string(body)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(`{"address":"192.0.2.10"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	if _, err := client.CreateReservedIPRaw(t.Context(), "us-east", []string{reservedIPTagFixture}); err != nil {
		t.Fatalf("CreateReservedIPRaw() error = %v", err)
	}

	if !strings.Contains(seen, `"tags":["`+reservedIPTagFixture+`"]`) {
		t.Errorf("request body = %v, want the supplied tag list", seen)
	}
}

// TestClientReservedIPWritesWrapTransportErrors proves a request that never
// reaches the API is reported as a NetworkError naming the operation, rather
// than as a nil body the caller would decode as an empty address.
func TestClientReservedIPWritesWrapTransportErrors(t *testing.T) {
	t.Parallel()

	// Closed immediately so every connection is refused.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachable := srv.URL

	srv.Close()

	client := linode.NewClient(unreachable, "test-token", nil,
		linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond))

	if _, err := client.CreateReservedIPRaw(t.Context(), "us-east", nil); err == nil {
		t.Error("CreateReservedIPRaw() error = nil, want a transport failure")
	}

	if _, err := client.UpdateReservedIPRaw(t.Context(), "192.0.2.10", []string{reservedIPTagFixture}); err == nil {
		t.Error("UpdateReservedIPRaw() error = nil, want a transport failure")
	}
}

func TestClientUpdateReservedIPRawSurfacesAPIErrors(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	if _, err := client.UpdateReservedIPRaw(t.Context(), "192.0.2.10", []string{reservedIPTagFixture}); err == nil {
		t.Fatal("UpdateReservedIPRaw() error = nil, want the API error")
	}
}

package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
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

// TestIsObjectBody covers the inputs a live route cannot produce: an empty body
// never reaches the predicate because the JSON decode fails first, and a body
// carrying leading whitespace still has to read as an object.
func TestIsObjectBody(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		raw  string
		want bool
	}{
		"address object":         {raw: `{"address":"192.0.2.10"}`, want: true},
		"object behind newlines": {raw: "\r\n\t {}", want: true},
		"empty":                  {raw: "", want: false},
		"whitespace only":        {raw: "   ", want: false},
		"array behind spaces":    {raw: "  []", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := linode.IsObjectBody([]byte(testCase.raw)); got != testCase.want {
				t.Errorf("IsObjectBody(%q) = %v, want %v", testCase.raw, got, testCase.want)
			}
		})
	}
}

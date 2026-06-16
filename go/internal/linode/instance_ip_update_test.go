package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	ipAddressFixture = "203.0.113.1"
	rdnsFixture      = "test.example.org"
)

func TestClientUpdateInstanceIPUsesPutPathAndBody(t *testing.T) {
	t.Parallel()

	rdns := rdnsFixture
	ipAddr := linode.IPAddress{Address: ipAddressFixture, RDNS: rdnsFixture, LinodeID: 123, Region: "us-east"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/ips/203.0.113.1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/ips/203.0.113.1")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		var body map[string]*string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		rdnsValue := body["rdns"]
		if rdnsValue == nil {
			t.Fatal("rdns should be present")
		}

		if *rdnsValue != rdnsFixture {
			t.Errorf("*rdnsValue = %v, want %v", *rdnsValue, rdnsFixture)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(ipAddr); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil)

	got, err := client.UpdateInstanceIP(t.Context(), 123, ipAddressFixture, linode.UpdateIPRDNSRequest{RDNS: &rdns})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.RDNS != rdnsFixture {
		t.Errorf("got.RDNS = %v, want %v", got.RDNS, rdnsFixture)
	}
}

func TestClientUpdateInstanceIPEncodesAddressPathSegment(t *testing.T) {
	t.Parallel()

	rdns := rdnsFixture

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/linode/instances/123/ips/203.0.113.1%2F..%3Fbad=1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/linode/instances/123/ips/203.0.113.1%2F..%3Fbad=1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.IPAddress{Address: ipAddressFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil)

	_, err := client.UpdateInstanceIP(t.Context(), 123, "203.0.113.1/..?bad=1", linode.UpdateIPRDNSRequest{RDNS: &rdns})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

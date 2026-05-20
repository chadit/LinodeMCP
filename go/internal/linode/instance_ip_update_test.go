package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
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
		assert.Equal(t, "/linode/instances/123/ips/203.0.113.1", r.URL.Path, "request path should match")
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

		var body map[string]*string
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")

		rdnsValue := body["rdns"]
		if assert.NotNil(t, rdnsValue, "rdns should be present") {
			assert.Equal(t, rdnsFixture, *rdnsValue, "rdns should match request")
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(ipAddr), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil)

	got, err := client.UpdateInstanceIP(t.Context(), 123, ipAddressFixture, linode.UpdateIPRDNSRequest{RDNS: &rdns})
	require.NoError(t, err, "UpdateInstanceIP should succeed")
	require.NotNil(t, got, "updated IP should not be nil")
	assert.Equal(t, rdnsFixture, got.RDNS, "response RDNS should match")
}

func TestClientUpdateInstanceIPEncodesAddressPathSegment(t *testing.T) {
	t.Parallel()

	rdns := rdnsFixture

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/ips/203.0.113.1%2F..%3Fbad=1", r.URL.EscapedPath(), "address should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not start a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{Address: ipAddressFixture}), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil)

	_, err := client.UpdateInstanceIP(t.Context(), 123, "203.0.113.1/..?bad=1", linode.UpdateIPRDNSRequest{RDNS: &rdns})
	require.NoError(t, err, "UpdateInstanceIP should encode unsafe path characters")
}

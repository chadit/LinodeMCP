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
	endpointFirewallSettings  = "/networking/firewalls/settings"
	firewallSettingsKeyLinode = "linode"
)

func TestClientListFirewallSettingsSuccess(t *testing.T) {
	t.Parallel()

	settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
		Linode: 100, NodeBalancer: 101, PublicInterface: 200, VPCInterface: 201,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Empty(t, r.URL.Query()["unexpected"], "request should not include extra query parameters")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallSettings(t.Context(), 2, 50)

	require.NoError(t, err, "ListFirewallSettings should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 100, result.DefaultFirewallIDs.Linode)
	assert.Equal(t, 101, result.DefaultFirewallIDs.NodeBalancer)
	assert.Equal(t, 200, result.DefaultFirewallIDs.PublicInterface)
	assert.Equal(t, 201, result.DefaultFirewallIDs.VPCInterface)
}

func TestClientUpdateFirewallSettingsSuccess(t *testing.T) {
	t.Parallel()

	linodeDefaultID := 100
	nodeBalancerDefaultID := 101
	publicInterfaceDefaultID := 102
	vpcInterfaceDefaultID := 103
	settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
		Linode: linodeDefaultID, NodeBalancer: nodeBalancerDefaultID, PublicInterface: publicInterfaceDefaultID, VPCInterface: vpcInterfaceDefaultID,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body map[string]map[string]int
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		assert.Equal(t, map[string]int{firewallSettingsKeyLinode: linodeDefaultID, "nodebalancer": nodeBalancerDefaultID, "public_interface": publicInterfaceDefaultID, "vpc_interface": vpcInterfaceDefaultID}, body["default_firewall_ids"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateFirewallSettings(t.Context(), &linode.UpdateFirewallSettingsRequest{
		DefaultFirewallIDs: linode.UpdateFirewallDefaultIDs{
			Linode:          &linodeDefaultID,
			NodeBalancer:    &nodeBalancerDefaultID,
			PublicInterface: &publicInterfaceDefaultID,
			VPCInterface:    &vpcInterfaceDefaultID,
		},
	})

	require.NoError(t, err, "UpdateFirewallSettings should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, linodeDefaultID, result.DefaultFirewallIDs.Linode)
	assert.Equal(t, nodeBalancerDefaultID, result.DefaultFirewallIDs.NodeBalancer)
	assert.Equal(t, publicInterfaceDefaultID, result.DefaultFirewallIDs.PublicInterface)
	assert.Equal(t, vpcInterfaceDefaultID, result.DefaultFirewallIDs.VPCInterface)
}

func TestClientUpdateFirewallSettingsHTTPError(t *testing.T) {
	t.Parallel()

	linodeDefaultID := 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateFirewallSettings(t.Context(), &linode.UpdateFirewallSettingsRequest{
		DefaultFirewallIDs: linode.UpdateFirewallDefaultIDs{Linode: &linodeDefaultID},
	})

	require.Error(t, err, "UpdateFirewallSettings should fail on HTTP error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateFirewallSettingsDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	linodeDefaultID := 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, endpointFirewallSettings, r.URL.Path, "request path should match")

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
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateFirewallSettings(t.Context(), &linode.UpdateFirewallSettingsRequest{
		DefaultFirewallIDs: linode.UpdateFirewallDefaultIDs{Linode: &linodeDefaultID},
	})

	require.Error(t, err, "UpdateFirewallSettings should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating PUT should not be replayed")
}

func TestClientListFirewallSettingsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallSettings(t.Context(), 0, 0)

	require.Error(t, err, "ListFirewallSettings should fail on HTTP error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListFirewallSettingsRetriesTransientFailure(t *testing.T) {
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
		assert.Equal(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{Linode: 100}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallSettings(t.Context(), 0, 0)

	require.NoError(t, err, "ListFirewallSettings should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 100, result.DefaultFirewallIDs.Linode)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

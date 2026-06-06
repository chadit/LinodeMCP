package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		stdCheckEqual(t, "2", r.URL.Query().Get("page"), "page query should match")
		stdCheckEqual(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		stdCheckEmpty(t, r.URL.Query()["unexpected"], "request should not include extra query parameters")

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(settings))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallSettings(t.Context(), 2, 50)

	stdMustNoError(t, err, "ListFirewallSettings should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, 100, result.DefaultFirewallIDs.Linode)
	stdCheckEqual(t, 101, result.DefaultFirewallIDs.NodeBalancer)
	stdCheckEqual(t, 200, result.DefaultFirewallIDs.PublicInterface)
	stdCheckEqual(t, 201, result.DefaultFirewallIDs.VPCInterface)
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
		stdCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		stdCheckEqual(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")

		var body map[string]map[string]int
		if !stdCheckNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		stdCheckEqual(t, map[string]int{firewallSettingsKeyLinode: linodeDefaultID, "nodebalancer": nodeBalancerDefaultID, "public_interface": publicInterfaceDefaultID, "vpc_interface": vpcInterfaceDefaultID}, body["default_firewall_ids"])

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(settings))
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

	stdMustNoError(t, err, "UpdateFirewallSettings should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, linodeDefaultID, result.DefaultFirewallIDs.Linode)
	stdCheckEqual(t, nodeBalancerDefaultID, result.DefaultFirewallIDs.NodeBalancer)
	stdCheckEqual(t, publicInterfaceDefaultID, result.DefaultFirewallIDs.PublicInterface)
	stdCheckEqual(t, vpcInterfaceDefaultID, result.DefaultFirewallIDs.VPCInterface)
}

func TestClientUpdateFirewallSettingsHTTPError(t *testing.T) {
	t.Parallel()

	linodeDefaultID := 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		stdCheckEqual(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		stdCheckNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateFirewallSettings(t.Context(), &linode.UpdateFirewallSettingsRequest{
		DefaultFirewallIDs: linode.UpdateFirewallDefaultIDs{Linode: &linodeDefaultID},
	})

	stdMustError(t, err, "UpdateFirewallSettings should fail on HTTP error")

	apiErr := stdAPIError(t, err, "error should wrap APIError")

	stdCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateFirewallSettingsDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	linodeDefaultID := 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		stdCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		stdCheckEqual(t, endpointFirewallSettings, r.URL.Path, "request path should match")

		hj, ok := w.(http.Hijacker)
		if !stdCheckTrue(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !stdCheckNoError(t, err) {
			return
		}

		stdCheckNoError(t, conn.Close())
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateFirewallSettings(t.Context(), &linode.UpdateFirewallSettingsRequest{
		DefaultFirewallIDs: linode.UpdateFirewallDefaultIDs{Linode: &linodeDefaultID},
	})

	stdMustError(t, err, "UpdateFirewallSettings should return the transient error")
	stdCheckEqual(t, int32(1), requestCount.Load(), "mutating PUT should not be replayed")
}

func TestClientListFirewallSettingsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		stdCheckNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallSettings(t.Context(), 0, 0)

	stdMustError(t, err, "ListFirewallSettings should fail on HTTP error")

	apiErr := stdAPIError(t, err, "error should wrap APIError")

	stdCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListFirewallSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hj, ok := w.(http.Hijacker)
			if !stdCheckTrue(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !stdCheckNoError(t, err) {
				return
			}

			stdCheckNoError(t, conn.Close())

			return
		}

		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallSettings, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{Linode: 100}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallSettings(t.Context(), 0, 0)

	stdMustNoError(t, err, "ListFirewallSettings should succeed after retry")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, 100, result.DefaultFirewallIDs.Linode)
	stdCheckEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

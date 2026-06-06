package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const endpointFirewallRules = "/networking/firewalls/123/rules"

func TestClientListFirewallRulesSuccess(t *testing.T) {
	t.Parallel()

	rules := linode.FirewallRules{
		InboundPolicy:  policyDrop,
		OutboundPolicy: policyAccept,
		Inbound: []linode.FirewallRule{{
			Action:   policyAccept,
			Protocol: protocolTCP,
			Ports:    "443",
			Label:    firewallRuleLabelAllowHTTPS,
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallRules, r.URL.Path, "request path should match")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(rules))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 123)

	stdMustNoError(t, err, "ListFirewallRules should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, policyDrop, result.InboundPolicy)
	stdCheckEqual(t, policyAccept, result.OutboundPolicy)
	stdMustLen(t, result.Inbound, 1)
	stdCheckEqual(t, firewallRuleLabelAllowHTTPS, result.Inbound[0].Label)
}

func TestClientListFirewallRulesRejectsInvalidFirewallID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 0)

	stdMustErrorIs(t, err, linode.ErrFirewallIDPositive, "invalid input should be rejected")
	stdCheckNil(t, result, "no rules should be returned")
	stdCheckFalse(t, called.Load(), "client should not call API for invalid input")
}

func TestClientListFirewallRulesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallRules, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		stdCheckNoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 123)

	stdMustError(t, err, "ListFirewallRules should fail on HTTP error")
	stdCheckNil(t, result, "no rules should be returned")
}

func TestClientListFirewallRulesRetriesTransientFailure(t *testing.T) {
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
		stdCheckEqual(t, endpointFirewallRules, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.FirewallRules{InboundPolicy: policyDrop}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallRules(t.Context(), 123)

	stdMustNoError(t, err, "ListFirewallRules should succeed after retry")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, policyDrop, result.InboundPolicy)
	stdCheckEqual(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

func TestClientUpdateFirewallRulesSuccess(t *testing.T) {
	t.Parallel()

	request := linode.FirewallRules{
		Inbound: []linode.FirewallRule{{
			Action:   policyAccept,
			Protocol: protocolTCP,
			Ports:    "443",
			Label:    firewallRuleLabelAllowHTTPS,
		}},
		Outbound: []linode.FirewallRule{},
	}
	response := linode.FirewallRules{InboundPolicy: policyDrop, OutboundPolicy: policyAccept, Inbound: request.Inbound}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		stdCheckEqual(t, endpointFirewallRules, r.URL.Path, "request path should match")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.FirewallRules
		stdCheckNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be valid JSON")
		stdCheckEqual(t, request.Inbound, got.Inbound)
		stdCheckEmpty(t, got.Outbound)

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(response))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateFirewallRules(t.Context(), 123, &request)

	stdMustNoError(t, err, "UpdateFirewallRules should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, policyDrop, result.InboundPolicy)
	stdMustLen(t, result.Inbound, 1)
	stdCheckEqual(t, firewallRuleLabelAllowHTTPS, result.Inbound[0].Label)
}

func TestClientUpdateFirewallRulesRejectsInvalidFirewallID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateFirewallRules(t.Context(), 0, &linode.FirewallRules{})

	stdMustErrorIs(t, err, linode.ErrFirewallIDPositive, "invalid input should be rejected")
	stdCheckNil(t, result, "no rules should be returned")
	stdCheckFalse(t, called.Load(), "client should not call API for invalid input")
}

func TestClientUpdateFirewallRulesRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateFirewallRules(t.Context(), 123, nil)

	stdMustErrorIs(t, err, linode.ErrFirewallRulesRequired, "nil rules request should be rejected")
	stdCheckNil(t, result, "no rules should be returned")
	stdCheckFalse(t, called.Load(), "client should not call API for nil rules request")
}

func TestClientUpdateFirewallRulesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		stdCheckEqual(t, endpointFirewallRules, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		stdCheckNoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateFirewallRules(t.Context(), 123, &linode.FirewallRules{})

	stdMustError(t, err, "UpdateFirewallRules should fail on HTTP error")
	stdCheckNil(t, result, "no rules should be returned")
}

func TestClientUpdateFirewallRulesDoesNotRetryTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.UpdateFirewallRules(t.Context(), 123, &linode.FirewallRules{})

	stdMustError(t, err, "UpdateFirewallRules should fail on 500 response")
	stdCheckNil(t, result, "no rules should be returned")
	stdCheckEqual(t, int32(1), requestCount.Load(), "mutating PUT must not retry and replay firewall rule replacement")
}

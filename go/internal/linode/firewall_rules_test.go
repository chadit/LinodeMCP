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

const endpointFirewallRules = "/networking/firewalls/123/rules"

func TestClientListFirewallRulesSuccess(t *testing.T) {
	t.Parallel()

	rules := linode.FirewallRules{
		InboundPolicy:  policyDrop,
		OutboundPolicy: policyAccept,
		Inbound: []linode.FirewallRule{{
			Action:   policyAccept,
			Protocol: "TCP",
			Ports:    "443",
			Label:    "allow-https",
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallRules, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(rules))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 123)

	require.NoError(t, err, "ListFirewallRules should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, policyDrop, result.InboundPolicy)
	assert.Equal(t, policyAccept, result.OutboundPolicy)
	require.Len(t, result.Inbound, 1)
	assert.Equal(t, "allow-https", result.Inbound[0].Label)
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

	require.ErrorIs(t, err, linode.ErrFirewallIDPositive, "invalid input should be rejected")
	assert.Nil(t, result, "no rules should be returned")
	assert.False(t, called.Load(), "client should not call API for invalid input")
}

func TestClientListFirewallRulesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallRules, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 123)

	require.Error(t, err, "ListFirewallRules should fail on HTTP error")
	assert.Nil(t, result, "no rules should be returned")
}

func TestClientListFirewallRulesRetriesTransientFailure(t *testing.T) {
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
		assert.Equal(t, endpointFirewallRules, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.FirewallRules{InboundPolicy: policyDrop}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallRules(t.Context(), 123)

	require.NoError(t, err, "ListFirewallRules should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, policyDrop, result.InboundPolicy)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

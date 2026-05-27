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

const endpointFirewallRuleVersions = "/networking/firewalls/123/history"

func TestClientListFirewallRuleVersionsSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     123,
		Label:  "web-firewall",
		Status: "enabled",
		Rules: linode.FirewallRules{
			Version:        2,
			Fingerprint:    "997dd135",
			InboundPolicy:  "DROP",
			OutboundPolicy: policyAccept,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallRuleVersions, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(firewall))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersions(t.Context(), 123)

	require.NoError(t, err, "ListFirewallRuleVersions should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, 2, result.Rules.Version)
	assert.Equal(t, "997dd135", result.Rules.Fingerprint)
}

func TestClientListFirewallRuleVersionsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallRuleVersions, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersions(t.Context(), 123)

	require.Error(t, err, "ListFirewallRuleVersions should fail on HTTP error")
	assert.Nil(t, result, "no firewall should be returned")
}

func TestClientListFirewallRuleVersionsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersions(t.Context(), 0)

	require.ErrorIs(t, err, linode.ErrFirewallIDPositive, "invalid input should be rejected")
	assert.Nil(t, result, "no firewall should be returned")
	assert.False(t, called.Load(), "client should not call API for invalid input")
}

func TestClientListFirewallRuleVersionsRetriesTransientFailure(t *testing.T) {
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
		assert.Equal(t, endpointFirewallRuleVersions, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Firewall{ID: 123}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallRuleVersions(t.Context(), 123)

	require.NoError(t, err, "ListFirewallRuleVersions should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

func TestClientGetFirewallRuleVersionSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{ID: 123, Label: "web-firewall", Rules: linode.FirewallRules{Version: 2, Fingerprint: "997dd135", Inbound: []linode.FirewallRule{{Action: policyAccept, Protocol: "TCP", Ports: "443", Label: "allow-https"}}}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/networking/firewalls/123/history/rules/2", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(firewall))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallRuleVersion(t.Context(), 123, 2)

	require.NoError(t, err, "GetFirewallRuleVersion should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "web-firewall", result.Label)
	assert.Equal(t, 2, result.Rules.Version)
	require.Len(t, result.Rules.Inbound, 1)
	assert.Equal(t, "allow-https", result.Rules.Inbound[0].Label)
}

func TestClientGetFirewallRuleVersionRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		firewallID int
		version    int
		wantErr    error
	}{
		{name: "zero firewall id", firewallID: 0, version: 2, wantErr: linode.ErrFirewallIDPositive},
		{name: "zero version", firewallID: 123, version: 0, wantErr: linode.ErrFirewallRuleVersionPositive},
		{name: "negative version", firewallID: 123, version: -1, wantErr: linode.ErrFirewallRuleVersionPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

			result, err := client.GetFirewallRuleVersion(t.Context(), testCase.firewallID, testCase.version)

			require.ErrorIs(t, err, testCase.wantErr, "invalid input should be rejected")
			assert.Nil(t, result, "no rule should be returned")
			assert.False(t, called.Load(), "client should not call API for invalid input")
		})
	}
}

func TestClientGetFirewallRuleVersionHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/networking/firewalls/123/history/rules/2", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallRuleVersion(t.Context(), 123, 2)

	require.Error(t, err, "GetFirewallRuleVersion should fail on HTTP error")
	assert.Nil(t, result, "no rule should be returned")
}

func TestClientGetFirewallRuleVersionRetriesTransientFailure(t *testing.T) {
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
		assert.Equal(t, "/networking/firewalls/123/history/rules/2", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Firewall{ID: 123, Label: "retried"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallRuleVersion(t.Context(), 123, 2)

	require.NoError(t, err, "GetFirewallRuleVersion should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "retried", result.Label)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

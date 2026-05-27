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

const endpointFirewallDevices = "/networking/firewalls/123/devices"

func TestClientListFirewallDevicesSuccess(t *testing.T) {
	t.Parallel()

	devices := linode.PaginatedResponse[linode.FirewallDevice]{
		Data: []linode.FirewallDevice{{
			ID: 456,
			Entity: linode.FirewallDeviceEntity{
				ID:    123,
				Label: "web-01",
				Type:  managedLinodeSettingsSSHUser,
				URL:   "/v4/linode/instances/123",
			},
			Created: "2025-01-01T00:01:01",
			Updated: "2025-01-02T00:01:01",
		}},
		Page:    2,
		Pages:   3,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		assert.Empty(t, r.URL.Query()["unexpected"], "request should not include extra query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(devices))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallDevices(t.Context(), 123, 2, 50)

	require.NoError(t, err, "ListFirewallDevices should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1, "result should include one device")
	assert.Equal(t, 456, result.Data[0].ID)
	assert.Equal(t, managedLinodeSettingsSSHUser, result.Data[0].Entity.Type)
	assert.Equal(t, 2, result.Page)
}

func TestClientListFirewallDevicesRejectsInvalidFirewallID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallDevices(t.Context(), 0, 0, 0)

	require.ErrorIs(t, err, linode.ErrFirewallIDPositive, "invalid firewall ID should be rejected")
	assert.Nil(t, result, "no devices should be returned")
	assert.False(t, called.Load(), "client should not call API for invalid firewall ID")
}

func TestClientListFirewallDevicesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallDevices(t.Context(), 123, 0, 0)

	require.Error(t, err, "ListFirewallDevices should fail on HTTP error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListFirewallDevicesRetriesTransientFailure(t *testing.T) {
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
		assert.Equal(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallDevice]{
			Data: []linode.FirewallDevice{{ID: 456}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallDevices(t.Context(), 123, 0, 0)

	require.NoError(t, err, "ListFirewallDevices should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1, "result should include one device")
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

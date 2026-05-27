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
	endpointFirewallDevices        = "/networking/firewalls/123/devices"
	endpointFirewallDeviceByID     = "/networking/firewalls/123/devices/456"
	caseZeroFirewallDeviceID       = "zero device id"
	caseZeroFirewallDeviceParentID = "zero firewall id"
)

func TestClientListFirewallDevicesSuccess(t *testing.T) {
	t.Parallel()

	devices := linode.PaginatedResponse[linode.FirewallDevice]{
		Data: []linode.FirewallDevice{{
			ID: 456,
			Entity: linode.FirewallDeviceEntity{
				ID:    123,
				Label: "web-01",
				Type:  managedLinodeSettingsSSHUser,
				URL:   accountMaintenanceURL,
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

func TestClientGetFirewallDeviceSuccess(t *testing.T) {
	t.Parallel()

	device := linode.FirewallDevice{
		ID: 456,
		Entity: linode.FirewallDeviceEntity{
			ID:    123,
			Label: "web-01",
			Type:  managedLinodeSettingsSSHUser,
			URL:   accountMaintenanceURL,
		},
		Created: "2025-01-01T00:01:01",
		Updated: "2025-01-02T00:01:01",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(device))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallDevice(t.Context(), 123, 456)

	require.NoError(t, err, "GetFirewallDevice should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 456, result.ID)
	assert.Equal(t, managedLinodeSettingsSSHUser, result.Entity.Type)
}

func TestClientGetFirewallDeviceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		firewallID int
		deviceID   int
		wantErr    error
	}{
		{name: caseZeroFirewallDeviceParentID, firewallID: 0, deviceID: 456, wantErr: linode.ErrFirewallIDPositive},
		{name: caseZeroFirewallDeviceID, firewallID: 123, deviceID: 0, wantErr: linode.ErrFirewallDeviceIDPositive},
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

			result, err := client.GetFirewallDevice(t.Context(), testCase.firewallID, testCase.deviceID)

			require.ErrorIs(t, err, testCase.wantErr, "invalid input should be rejected")
			assert.Nil(t, result, "no device should be returned")
			assert.False(t, called.Load(), "client should not call API for invalid input")
		})
	}
}

func TestClientGetFirewallDeviceRetriesTransientFailure(t *testing.T) {
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
		assert.Equal(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.FirewallDevice{ID: 456}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallDevice(t.Context(), 123, 456)

	require.NoError(t, err, "GetFirewallDevice should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 456, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

func TestClientCreateFirewallDeviceSuccess(t *testing.T) {
	t.Parallel()

	device := linode.FirewallDevice{
		ID: 789,
		Entity: linode.FirewallDeviceEntity{
			ID:    456,
			Label: "web-02",
			Type:  managedLinodeSettingsSSHUser,
			URL:   "/v4/linode/instances/456",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateFirewallDeviceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be JSON")
		assert.Equal(t, linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser}, got)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(device))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateFirewallDevice(t.Context(), 123, &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})

	require.NoError(t, err, "CreateFirewallDevice should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 789, result.ID)
	assert.Equal(t, managedLinodeSettingsSSHUser, result.Entity.Type)
}

func TestClientCreateFirewallDeviceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		firewallID int
		req        *linode.CreateFirewallDeviceRequest
		wantErr    error
	}{
		{name: caseZeroFirewallDeviceParentID, firewallID: 0, req: &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser}, wantErr: linode.ErrFirewallIDPositive},
		{name: "nil request", firewallID: 123, req: nil, wantErr: linode.ErrFirewallDeviceIDPositive},
		{name: caseZeroFirewallDeviceID, firewallID: 123, req: &linode.CreateFirewallDeviceRequest{Type: managedLinodeSettingsSSHUser}, wantErr: linode.ErrFirewallDeviceIDPositive},
		{name: "missing type", firewallID: 123, req: &linode.CreateFirewallDeviceRequest{ID: 456}, wantErr: linode.ErrFirewallDeviceTypeRequired},
		{name: "slash type", firewallID: 123, req: &linode.CreateFirewallDeviceRequest{ID: 456, Type: "linode/123"}, wantErr: linode.ErrInvalidFirewallDeviceType},
		{name: "query type", firewallID: 123, req: &linode.CreateFirewallDeviceRequest{ID: 456, Type: "linode?x=1"}, wantErr: linode.ErrInvalidFirewallDeviceType},
		{name: "traversal type", firewallID: 123, req: &linode.CreateFirewallDeviceRequest{ID: 456, Type: ".."}, wantErr: linode.ErrInvalidFirewallDeviceType},
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

			result, err := client.CreateFirewallDevice(t.Context(), testCase.firewallID, testCase.req)

			require.ErrorIs(t, err, testCase.wantErr, "invalid input should be rejected")
			assert.Nil(t, result, "no device should be returned")
			assert.False(t, called.Load(), "client should not call API for invalid input")
		})
	}
}

func TestClientDeleteFirewallDeviceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query params")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteFirewallDevice(t.Context(), 123, 456)

	require.NoError(t, err, "DeleteFirewallDevice should succeed on 200 response")
}

func TestClientDeleteFirewallDeviceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		firewallID int
		deviceID   int
		wantErr    error
	}{
		{name: caseZeroFirewallDeviceParentID, firewallID: 0, deviceID: 456, wantErr: linode.ErrFirewallIDPositive},
		{name: caseZeroFirewallDeviceID, firewallID: 123, deviceID: 0, wantErr: linode.ErrFirewallDeviceIDPositive},
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
			err := client.DeleteFirewallDevice(t.Context(), testCase.firewallID, testCase.deviceID)

			require.ErrorIs(t, err, testCase.wantErr, "invalid input should return expected error")
			assert.False(t, called.Load(), "client should not call API for invalid input")
		})
	}
}

func TestClientDeleteFirewallDeviceHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteFirewallDevice(t.Context(), 123, 456)

	require.Error(t, err, "DeleteFirewallDevice should fail on HTTP error")
}

func TestClientDeleteFirewallDeviceDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")

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
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	err := client.DeleteFirewallDevice(t.Context(), 123, 456)

	require.Error(t, err, "DeleteFirewallDevice should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "DELETE must not be replayed after transient failure")
}

func TestClientCreateFirewallDeviceDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointFirewallDevices, r.URL.Path, "request path should match")

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
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.CreateFirewallDevice(t.Context(), 123, &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})

	require.Error(t, err, "transient error should be returned")
	assert.Nil(t, result, "no device should be returned")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating POST should not be replayed")
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

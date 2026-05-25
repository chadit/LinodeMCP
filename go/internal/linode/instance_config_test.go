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

func TestClientCreateInstanceConfigSendsRequest(t *testing.T) {
	t.Parallel()

	diskID := 456
	wantReq := linode.CreateConfigRequest{
		Label: labelBootConfig,
		Devices: map[string]*linode.ConfigDevice{
			configDeviceSlotSDA: {DiskID: &diskID},
		},
		Kernel:     configKernelLatest,
		RootDevice: "/dev/sda",
		RunLevel:   "default",
		VirtMode:   "paravirt",
	}
	response := linode.InstanceConfig{ID: 789, Label: wantReq.Label, Devices: wantReq.Devices}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

		var got linode.CreateConfigRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
		assert.Equal(t, wantReq, got, "request body should match")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateInstanceConfig(t.Context(), 123, &wantReq)

	require.NoError(t, err, "CreateInstanceConfig should succeed")
	require.NotNil(t, got, "created config should be returned")
	assert.Equal(t, response.ID, got.ID, "config ID should match")
	assert.Equal(t, response.Label, got.Label, "config label should match")
}

func TestClientCreateInstanceConfigDoesNotRetryCreate(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	diskID := 456
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	_, err := client.CreateInstanceConfig(t.Context(), 123, &linode.CreateConfigRequest{
		Label: labelBootConfig,
		Devices: map[string]*linode.ConfigDevice{
			configDeviceSlotSDA: {DiskID: &diskID},
		},
	})

	require.Error(t, err, "server failure should be returned")
	assert.Equal(t, int32(1), calls.Load(), "POST create must not be retried")
}

func TestClientCreateInstanceConfigRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	diskID := 456
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateInstanceConfig(t.Context(), 0, &linode.CreateConfigRequest{
		Label: labelBootConfig,
		Devices: map[string]*linode.ConfigDevice{
			configDeviceSlotSDA: {DiskID: &diskID},
		},
	})

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")
	assert.False(t, called.Load(), "invalid linode ID should not issue HTTP request")
}

func TestClientDeleteInstanceConfigSendsRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Empty(t, r.URL.RawQuery, "request should not include query params")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceConfig(t.Context(), 123, 789)

	require.NoError(t, err, "DeleteInstanceConfig should succeed")
}

func TestClientDeleteInstanceConfigDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteInstanceConfig(t.Context(), 123, 789)

	require.Error(t, err, "server failure should be returned")
	assert.Equal(t, int32(1), calls.Load(), "DELETE must not be retried")
}

func TestClientDeleteInstanceConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceConfig(t.Context(), 0, 789)

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	err = client.DeleteInstanceConfig(t.Context(), 123, 0)
	require.ErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")
	assert.False(t, called.Load(), "invalid IDs should not issue HTTP request")
}

func TestClientCreateInstanceConfigRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateInstanceConfig(t.Context(), 123, nil)

	require.ErrorIs(t, err, linode.ErrCreateConfigRequestRequired, "nil request should be rejected")
	assert.False(t, called.Load(), "nil request should not issue HTTP request")
}

func TestClientUpdateInstanceConfigSendsRequest(t *testing.T) {
	t.Parallel()

	diskID := 456
	label := labelBootConfig
	kernel := configKernelLatest
	devices := map[string]*linode.ConfigDevice{
		configDeviceSlotSDA: {DiskID: &diskID},
	}
	wantReq := linode.UpdateConfigRequest{
		Label:   &label,
		Devices: &devices,
		Kernel:  &kernel,
	}
	response := linode.InstanceConfig{ID: 789, Label: label, Devices: devices}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

		var got linode.UpdateConfigRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if assert.NotNil(t, got.Label, "label should be sent") {
			assert.Equal(t, *wantReq.Label, *got.Label, "label should match")
		}

		if assert.NotNil(t, got.Devices, "devices should be sent") {
			assert.Equal(t, *wantReq.Devices, *got.Devices, "devices should match")
		}

		if assert.NotNil(t, got.Kernel, "kernel should be sent") {
			assert.Equal(t, *wantReq.Kernel, *got.Kernel, "kernel should match")
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateInstanceConfig(t.Context(), 123, 789, &wantReq)

	require.NoError(t, err, "UpdateInstanceConfig should succeed")
	require.NotNil(t, got, "updated config should be returned")
	assert.Equal(t, response.ID, got.ID, "config ID should match")
	assert.Equal(t, response.Label, got.Label, "config label should match")
}

func TestClientUpdateInstanceConfigDoesNotRetryUpdate(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	label := labelBootConfig

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateInstanceConfig(t.Context(), 123, 789, &linode.UpdateConfigRequest{Label: &label})

	require.Error(t, err, "server failure should be returned")
	assert.Equal(t, int32(1), calls.Load(), "PUT update must not be retried")
}

func TestClientUpdateInstanceConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	label := labelBootConfig

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateInstanceConfig(t.Context(), 0, 789, &linode.UpdateConfigRequest{Label: &label})
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	_, err = client.UpdateInstanceConfig(t.Context(), 123, 0, &linode.UpdateConfigRequest{Label: &label})
	require.ErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")
	assert.False(t, called.Load(), "invalid IDs should not issue HTTP request")
}

func TestClientUpdateInstanceConfigRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateInstanceConfig(t.Context(), 123, 789, nil)

	require.ErrorIs(t, err, linode.ErrUpdateConfigRequestRequired, "nil request should be rejected")
	assert.False(t, called.Load(), "nil request should not issue HTTP request")
}

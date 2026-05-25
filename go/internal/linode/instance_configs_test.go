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

func TestClientListInstanceConfigsSuccess(t *testing.T) {
	t.Parallel()

	diskID := 456
	distro := true
	configs := []linode.InstanceConfig{
		{
			ID:         77,
			Label:      labelBootConfig,
			Kernel:     configKernelLatest,
			RootDevice: "/dev/sda",
			Devices: map[string]*linode.ConfigDevice{
				configDeviceSlotSDA: {DiskID: &diskID},
			},
			Helpers: &linode.ConfigHelpers{Distro: &distro},
			Interfaces: []linode.ConfigInterface{
				{Purpose: "public"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: configs, keyPage: 2, keyPages: 3, keyResults: 1,
		}), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceConfigs(t.Context(), 123, 2, 50)

	require.NoError(t, err, "ListInstanceConfigs should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, labelBootConfig, got[0].Label)
	assert.Equal(t, configKernelLatest, got[0].Kernel)
	assert.Equal(t, diskID, *got[0].Devices[configDeviceSlotSDA].DiskID)
	require.NotNil(t, got[0].Helpers)
	require.NotNil(t, got[0].Helpers.Distro)
	assert.True(t, *got[0].Helpers.Distro)
	require.Len(t, got[0].Interfaces, 1)
	assert.Equal(t, "public", got[0].Interfaces[0].Purpose)
}

func TestClientListInstanceConfigsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceConfigs(t.Context(), 123, 0, 0)

	require.Error(t, err, "ListInstanceConfigs should fail on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListInstanceConfigsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceConfigs(t.Context(), -1, 0, 0)

	require.Error(t, err, "ListInstanceConfigs should reject invalid linode IDs before request")
	assert.False(t, called.Load(), "invalid linode ID should not reach upstream server")
	assert.ErrorIs(t, err, linode.ErrLinodeIDPositive, "error should expose invalid linode ID sentinel")
}

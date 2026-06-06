package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
			Interfaces: []linode.ConfigInterfaceResponse{
				{ID: 101, Purpose: purposePublic},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		checkEqual(t, "page=2&page_size=50", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: configs, keyPage: 2, keyPages: 3, keyResults: 1,
		}), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceConfigs(t.Context(), 123, 2, 50)

	requireNoError(t, err, "ListInstanceConfigs should succeed on 200 response")
	requireLenOne(t, got)

	config := got[0]
	checkEqual(t, labelBootConfig, config.Label)
	checkEqual(t, configKernelLatest, config.Kernel)
	device := config.Devices[configDeviceSlotSDA]
	requireNotNil(t, device, "SDA device should be present")
	requireNotNil(t, device.DiskID, "SDA disk ID should be present")
	checkEqual(t, diskID, *device.DiskID)
	requireNotNil(t, config.Helpers)
	requireNotNil(t, config.Helpers.Distro)
	checkTrue(t, *config.Helpers.Distro)
	requireLenOne(t, config.Interfaces)
	checkEqual(t, purposePublic, config.Interfaces[0].Purpose)
}

func TestClientListInstanceConfigsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceConfigs(t.Context(), 123, 0, 0)

	requireError(t, err, "ListInstanceConfigs should fail on API error")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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

	requireError(t, err, "ListInstanceConfigs should reject invalid linode IDs before request")

	if called.Load() {
		t.Fatalf("invalid linode ID should not reach upstream server")
	}

	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "error should expose invalid linode ID sentinel")
}

package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs)
		}

		if r.URL.RawQuery != tcPage2PageSize50 {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, tcPage2PageSize50)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: configs, keyPage: 2, keyPages: 3, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceConfigs(t.Context(), 123, 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	config := got[0]
	if config.Label != labelBootConfig {
		t.Errorf("config.Label = %v, want %v", config.Label, labelBootConfig)
	}

	if config.Kernel != configKernelLatest {
		t.Errorf("config.Kernel = %v, want %v", config.Kernel, configKernelLatest)
	}

	device := config.Devices[configDeviceSlotSDA]
	if device == nil {
		t.Fatal("device is nil")
	}

	if device.DiskID == nil {
		t.Fatal("device.DiskID is nil")
	}

	if *device.DiskID != diskID {
		t.Errorf("*device.DiskID = %v, want %v", *device.DiskID, diskID)
	}

	if config.Helpers == nil {
		t.Fatal("config.Helpers is nil")
	}

	if config.Helpers.Distro == nil {
		t.Fatal("config.Helpers.Distro is nil")
	}

	if !(*config.Helpers.Distro) {
		t.Error("*config.Helpers.Distro = false, want true")
	}

	if len(config.Interfaces) != 1 {
		t.Fatalf("len(config.Interfaces) = %d, want 1", len(config.Interfaces))
	}

	if config.Interfaces[0].Purpose != purposePublic {
		t.Errorf("config.Interfaces[0].Purpose = %v, want %v", config.Interfaces[0].Purpose, purposePublic)
	}
}

func TestClientListInstanceConfigsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListInstanceConfigs(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
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
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if called.Load() {
		t.Fatalf("invalid linode ID should not reach upstream server")
	}

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}
}

package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDevices)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "50" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "50")
		}

		if len(r.URL.Query()["unexpected"]) != 0 {
			t.Errorf("value = %v, want empty", r.URL.Query()["unexpected"])
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(devices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallDevices(t.Context(), 123, 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 456 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 456)
	}

	if result.Data[0].Entity.Type != managedLinodeSettingsSSHUser {
		t.Errorf("result.Data[0].Entity.Type = %v, want %v", result.Data[0].Entity.Type, managedLinodeSettingsSSHUser)
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallDeviceByID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDeviceByID)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(device); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallDevice(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 456 {
		t.Errorf("result.ID = %v, want %v", result.ID, 456)
	}

	if result.Entity.Type != managedLinodeSettingsSSHUser {
		t.Errorf("result.Entity.Type = %v, want %v", result.Entity.Type, managedLinodeSettingsSSHUser)
	}
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

			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}

			if result != nil {
				t.Errorf("result = %v, want nil", result)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestClientGetFirewallDeviceRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallDeviceByID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDeviceByID)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.FirewallDevice{ID: 456}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallDevice(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 456 {
		t.Errorf("result.ID = %v, want %v", result.ID, 456)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
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
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointFirewallDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDevices)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.CreateFirewallDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser}) {
			t.Errorf("got = %v, want %v", got, linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(device); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateFirewallDevice(t.Context(), 123, &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 789 {
		t.Errorf("result.ID = %v, want %v", result.ID, 789)
	}

	if result.Entity.Type != managedLinodeSettingsSSHUser {
		t.Errorf("result.Entity.Type = %v, want %v", result.Entity.Type, managedLinodeSettingsSSHUser)
	}
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

			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}

			if result != nil {
				t.Errorf("result = %v, want nil", result)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestClientDeleteFirewallDeviceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != endpointFirewallDeviceByID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDeviceByID)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteFirewallDevice(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestClientDeleteFirewallDeviceHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != endpointFirewallDeviceByID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDeviceByID)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteFirewallDevice(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientDeleteFirewallDeviceDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != endpointFirewallDeviceByID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDeviceByID)
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteFirewallDevice(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientCreateFirewallDeviceDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointFirewallDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDevices)
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer should support hijacking")

			return
		}

		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.CreateFirewallDevice(t.Context(), 123, &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
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

	if !errors.Is(err, linode.ErrFirewallIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrFirewallIDPositive)
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if called.Load() {
		t.Error("called.Load() = true, want false")
	}
}

func TestClientListFirewallDevicesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDevices)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallDevices(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientListFirewallDevicesRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallDevices)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallDevice]{
			Data: []linode.FirewallDevice{{ID: 456}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallDevices(t.Context(), 123, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")

		var got linode.CreateConfigRequest
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		checkEqual(t, wantReq, got, "request body should match")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateInstanceConfig(t.Context(), 123, &wantReq)

	requireNoError(t, err, "CreateInstanceConfig should succeed")
	requireNotNil(t, got, "created config should be returned")
	checkEqual(t, response.ID, got.ID, "config ID should match")
	checkEqual(t, response.Label, got.Label, "config label should match")
}

func TestClientCreateInstanceConfigDoesNotRetryCreate(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
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

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), calls.Load(), "POST create must not be retried")
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

	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	if called.Load() {
		t.Fatalf("invalid linode ID should not issue HTTP request")
	}
}

func TestClientDeleteInstanceConfigSendsRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEmpty(t, r.URL.RawQuery, "request should not include query params")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceConfig(t.Context(), 123, 789)

	requireNoError(t, err, "DeleteInstanceConfig should succeed")
}

func TestClientDeleteInstanceConfigDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteInstanceConfig(t.Context(), 123, 789)

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), calls.Load(), "DELETE must not be retried")
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

	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	err = client.DeleteInstanceConfig(t.Context(), 123, 0)
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
	}
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

	requireErrorIs(t, err, linode.ErrCreateConfigRequestRequired, "nil request should be rejected")

	if called.Load() {
		t.Fatalf("nil request should not issue HTTP request")
	}
}

func TestClientAddInstanceConfigInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	primary := true
	wantReq := linode.ConfigInterface{Purpose: purposeVPC, Primary: &primary}
	response := linode.ConfigInterface{Purpose: wantReq.Purpose, Primary: wantReq.Primary}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/789/interfaces", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")

		var got linode.ConfigInterface
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		checkEqual(t, wantReq, got, "request body should match")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.AddInstanceConfigInterface(t.Context(), 123, 789, &wantReq)

	requireNoError(t, err, "AddInstanceConfigInterface should succeed")
	requireNotNil(t, got, "created interface should be returned")
	checkEqual(t, response.Purpose, got.Purpose, "interface purpose should match")
}

func TestClientGetInstanceConfigInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	primary := true
	response := linode.ConfigInterfaceResponse{ID: 456, Active: true, Purpose: purposeVPC, Primary: primary}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEmpty(t, r.URL.RawQuery, "request should not include query params")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetInstanceConfigInterface(t.Context(), 123, 789, 456)

	requireNoError(t, err, "GetInstanceConfigInterface should succeed")
	requireNotNil(t, got, "interface should be returned")
	checkEqual(t, response.Purpose, got.Purpose, "interface purpose should match")
	checkEqual(t, response.ID, got.ID, "interface ID should match")
	checkTrue(t, got.Active, "interface active flag should match")
}

func TestClientDeleteInstanceConfigInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEmpty(t, r.URL.RawQuery, "request should not include query params")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr, "writing empty response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceConfigInterface(t.Context(), 123, 789, 456)

	requireNoError(t, err, "DeleteInstanceConfigInterface should succeed")
}

func TestClientDeleteInstanceConfigInterfaceDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteInstanceConfigInterface(t.Context(), 123, 789, 456)

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), attempts.Load(), "DELETE interface removal must not be retried")
}

func TestClientDeleteInstanceConfigInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceConfigInterface(t.Context(), 0, 789, 456)
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	err = client.DeleteInstanceConfigInterface(t.Context(), 123, 0, 456)
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	err = client.DeleteInstanceConfigInterface(t.Context(), 123, 789, 0)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "zero interface ID should be rejected")

	err = client.DeleteInstanceConfigInterface(t.Context(), 123, 789, -1)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "negative interface ID should be rejected")

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
	}
}

func TestClientGetInstanceConfigInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetInstanceConfigInterface(t.Context(), 0, 789, 456)
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	_, err = client.GetInstanceConfigInterface(t.Context(), 123, 0, 456)
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	_, err = client.GetInstanceConfigInterface(t.Context(), 123, 789, 0)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "zero interface ID should be rejected")

	_, err = client.GetInstanceConfigInterface(t.Context(), 123, 789, -1)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "negative interface ID should be rejected")

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
	}
}

func TestClientAddInstanceConfigInterfaceDoesNotRetryCreate(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/linode/instances/123/configs/789/interfaces", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	_, err := client.AddInstanceConfigInterface(t.Context(), 123, 789, &linode.ConfigInterface{Purpose: purposePublic})

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), calls.Load(), "POST interface create must not be retried")
}

func TestClientAddInstanceConfigInterfaceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.AddInstanceConfigInterface(t.Context(), 0, 789, &linode.ConfigInterface{Purpose: purposePublic})
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	_, err = client.AddInstanceConfigInterface(t.Context(), 123, 0, &linode.ConfigInterface{Purpose: purposePublic})
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	_, err = client.AddInstanceConfigInterface(t.Context(), 123, 789, nil)
	requireErrorIs(t, err, linode.ErrAddConfigInterfaceRequestRequired, "nil request should be rejected")

	if called.Load() {
		t.Fatalf("invalid input should not issue HTTP request")
	}
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
		checkEqual(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")

		var got linode.UpdateConfigRequest
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		if got.Label == nil {
			t.Errorf("label should be sent: expected non-nil value")

			return
		}

		checkEqual(t, *wantReq.Label, *got.Label, "label should match")

		if got.Devices == nil {
			t.Errorf("devices should be sent: expected non-nil value")

			return
		}

		checkEqual(t, *wantReq.Devices, *got.Devices, "devices should match")

		if got.Kernel == nil {
			t.Errorf("kernel should be sent: expected non-nil value")

			return
		}

		checkEqual(t, *wantReq.Kernel, *got.Kernel, "kernel should match")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateInstanceConfig(t.Context(), 123, 789, &wantReq)

	requireNoError(t, err, "UpdateInstanceConfig should succeed")
	requireNotNil(t, got, "updated config should be returned")
	checkEqual(t, response.ID, got.ID, "config ID should match")
	checkEqual(t, response.Label, got.Label, "config label should match")
}

func TestClientUpdateInstanceConfigDoesNotRetryUpdate(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	label := labelBootConfig

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateInstanceConfig(t.Context(), 123, 789, &linode.UpdateConfigRequest{Label: &label})

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), calls.Load(), "PUT update must not be retried")
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
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	_, err = client.UpdateInstanceConfig(t.Context(), 123, 0, &linode.UpdateConfigRequest{Label: &label})
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
	}
}

func TestClientReorderInstanceConfigInterfacesSendsRequest(t *testing.T) {
	t.Parallel()

	wantReq := linode.ReorderConfigInterfacesRequest{IDs: []int{101, 102, 103}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/789/interfaces/order", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")

		var got linode.ReorderConfigInterfacesRequest
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		checkEqual(t, wantReq, got, "request body should match")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr, "writing empty response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.ReorderInstanceConfigInterfaces(t.Context(), 123, 789, &wantReq)

	requireNoError(t, err, "ReorderInstanceConfigInterfaces should succeed")
}

func TestClientReorderInstanceConfigInterfacesDoesNotRetryReorder(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	err := client.ReorderInstanceConfigInterfaces(t.Context(), 123, 789, &linode.ReorderConfigInterfacesRequest{IDs: []int{101}})

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), attempts.Load(), "POST reorder should not be retried")
}

func TestClientReorderInstanceConfigInterfacesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called.Store(true)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.ReorderInstanceConfigInterfaces(t.Context(), 0, 789, &linode.ReorderConfigInterfacesRequest{IDs: []int{101}})
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	err = client.ReorderInstanceConfigInterfaces(t.Context(), 123, 0, &linode.ReorderConfigInterfacesRequest{IDs: []int{101}})
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	err = client.ReorderInstanceConfigInterfaces(t.Context(), 123, 789, nil)
	requireErrorIs(t, err, linode.ErrReorderConfigInterfacesRequestRequired, "nil request should be rejected")

	if called.Load() {
		t.Fatalf("invalid input should not issue HTTP request")
	}
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

	requireErrorIs(t, err, linode.ErrUpdateConfigRequestRequired, "nil request should be rejected")

	if called.Load() {
		t.Fatalf("nil request should not issue HTTP request")
	}
}

func TestClientUpdateInstanceConfigInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	primary := true
	wantReq := linode.UpdateConfigInterfaceRequest{
		Primary:  &primary,
		IPRanges: []string{"10.0.0.0/24"},
	}
	response := linode.ConfigInterfaceResponse{ID: 101, Purpose: "public", Primary: true, IPRanges: wantReq.IPRanges}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/configs/789/interfaces/101", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")

		var got linode.UpdateConfigInterfaceRequest
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		if got.Primary == nil {
			t.Errorf("primary should be set: expected non-nil value")

			return
		}

		checkTrue(t, *got.Primary, "primary should match")

		checkEqual(t, wantReq.IPRanges, got.IPRanges, "IP ranges should match")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	updated, err := client.UpdateInstanceConfigInterface(t.Context(), 123, 789, 101, &wantReq)

	requireNoError(t, err, "UpdateInstanceConfigInterface should succeed")
	requireNotNil(t, updated, "updated interface should be returned")
	checkEqual(t, 101, updated.ID, "interface ID should match")
	checkEqual(t, wantReq.IPRanges, updated.IPRanges, "IP ranges should match")
}

func TestClientUpdateInstanceConfigInterfaceDoesNotRetryUpdate(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/linode/instances/123/configs/789/interfaces/101", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	primary := true

	_, err := client.UpdateInstanceConfigInterface(t.Context(), 123, 789, 101, &linode.UpdateConfigInterfaceRequest{Primary: &primary})

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), calls.Load(), "PUT update should not be retried")
}

func TestClientUpdateInstanceConfigInterfaceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	primary := true
	req := &linode.UpdateConfigInterfaceRequest{Primary: &primary}

	_, err := client.UpdateInstanceConfigInterface(t.Context(), 0, 789, 101, req)
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	_, err = client.UpdateInstanceConfigInterface(t.Context(), 123, 0, 101, req)
	requireErrorIs(t, err, linode.ErrConfigIDPositive, "invalid config ID should be rejected")

	_, err = client.UpdateInstanceConfigInterface(t.Context(), 123, 789, 0, req)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "invalid interface ID should be rejected")

	_, err = client.UpdateInstanceConfigInterface(t.Context(), 123, 789, 101, nil)
	requireErrorIs(t, err, linode.ErrUpdateConfigInterfaceRequestRequired, "nil request should be rejected")

	if called.Load() {
		t.Fatalf("invalid input should not issue HTTP request")
	}
}

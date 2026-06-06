package linode_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

func stdHelperMessage(args ...any) string {
	if len(args) == 0 {
		return ""
	}

	if format, ok := args[0].(string); ok {
		if len(args) == 1 || !strings.Contains(format, "%") {
			return format
		}

		return fmt.Sprintf(format, args[1:]...)
	}

	return fmt.Sprint(args...)
}

func stdWithMessage(base string, args ...any) string {
	msg := stdHelperMessage(args...)
	if msg == "" {
		return base
	}

	return msg + ": " + base
}

func stdIsNil(value any) bool {
	if value == nil {
		return true
	}

	kind := reflect.ValueOf(value).Kind()
	if kind != reflect.Chan && kind != reflect.Func && kind != reflect.Interface && kind != reflect.Map && kind != reflect.Pointer && kind != reflect.Slice {
		return false
	}

	return reflect.ValueOf(value).IsNil()
}

func stdIsEmpty(value any) bool {
	if stdIsNil(value) {
		return true
	}

	reflected := reflect.ValueOf(value)

	kind := reflected.Kind()
	if kind == reflect.Array || kind == reflect.Chan || kind == reflect.Map || kind == reflect.Slice || kind == reflect.String {
		return reflected.Len() == 0
	}

	return reflected.IsZero()
}

func stdLength(value any) (int, bool) {
	if stdIsNil(value) {
		return 0, false
	}

	reflected := reflect.ValueOf(value)

	kind := reflected.Kind()
	if kind == reflect.Array || kind == reflect.Chan || kind == reflect.Map || kind == reflect.Slice || kind == reflect.String {
		return reflected.Len(), true
	}

	return 0, false
}

func stdCheckEqual(t *testing.T, want, got any, args ...any) {
	t.Helper()

	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("got %#v, want %#v", got, want), args...))
	}
}

func stdCheckInEpsilon(t *testing.T, want, got, epsilon float64, args ...any) {
	t.Helper()

	if math.Abs(want-got) > epsilon {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("got %v, want %v within %v", got, want, epsilon), args...))
	}
}

func stdCheckEmpty(t *testing.T, value any, args ...any) {
	t.Helper()

	if !stdIsEmpty(value) {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("got non-empty value %#v", value), args...))
	}
}

func stdCheckNil(t *testing.T, value any, args ...any) {
	t.Helper()

	if !stdIsNil(value) {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("got non-nil value %#v", value), args...))
	}
}

func stdCheckNotNil(t *testing.T, value any, args ...any) bool {
	t.Helper()

	if stdIsNil(value) {
		t.Errorf("%s", stdWithMessage("got nil value", args...))

		return false
	}

	return true
}

func stdMustNotNil(t *testing.T, value any, args ...any) {
	t.Helper()

	if !stdCheckNotNil(t, value, args...) {
		t.FailNow()
	}
}

func stdCheckLen(t *testing.T, value any, want int, args ...any) bool {
	t.Helper()

	got, ok := stdLength(value)
	if !ok || got != want {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("got length %d, want %d", got, want), args...))

		return false
	}

	return true
}

func stdMustLen(t *testing.T, value any, want int, args ...any) {
	t.Helper()

	if !stdCheckLen(t, value, want, args...) {
		t.FailNow()
	}
}

func stdCheckTrue(t *testing.T, value bool, args ...any) bool {
	t.Helper()

	if !value {
		t.Errorf("%s", stdWithMessage("got false, want true", args...))

		return false
	}

	return true
}

func stdCheckFalse(t *testing.T, value bool, args ...any) {
	t.Helper()

	if value {
		t.Errorf("%s", stdWithMessage("got true, want false", args...))
	}
}

func stdCheckNoError(t *testing.T, err error, args ...any) bool {
	t.Helper()

	if err != nil {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("unexpected error: %v", err), args...))

		return false
	}

	return true
}

func stdMustNoError(t *testing.T, err error, args ...any) {
	t.Helper()

	if !stdCheckNoError(t, err, args...) {
		t.FailNow()
	}
}

func stdCheckError(t *testing.T, err error, args ...any) bool {
	t.Helper()

	if err == nil {
		t.Errorf("%s", stdWithMessage("expected error", args...))

		return false
	}

	return true
}

func stdMustError(t *testing.T, err error, args ...any) {
	t.Helper()

	if !stdCheckError(t, err, args...) {
		t.FailNow()
	}
}

func stdAPIError(t *testing.T, err error, args ...any) *linode.APIError {
	t.Helper()

	for current := any(err); current != nil; {
		apiErr, matched := current.(*linode.APIError)
		if matched {
			return apiErr
		}

		wrapped, canUnwrap := current.(interface{ Unwrap() error })
		if !canUnwrap {
			break
		}

		current = wrapped.Unwrap()
	}

	t.Fatalf("%s", stdWithMessage(fmt.Sprintf("error %v does not wrap APIError", err), args...))

	return nil
}

func stdCheckErrorIs(t *testing.T, err, target error, args ...any) bool {
	t.Helper()

	if !errors.Is(err, target) {
		t.Errorf("%s", stdWithMessage(fmt.Sprintf("got error %v, want error %v", err, target), args...))

		return false
	}

	return true
}

func stdMustErrorIs(t *testing.T, err, target error, args ...any) {
	t.Helper()

	if !stdCheckErrorIs(t, err, target, args...) {
		t.FailNow()
	}
}

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
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		stdCheckEqual(t, "2", r.URL.Query().Get("page"), "page query should match")
		stdCheckEqual(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		stdCheckEmpty(t, r.URL.Query()["unexpected"], "request should not include extra query parameters")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(devices))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallDevices(t.Context(), 123, 2, 50)

	stdMustNoError(t, err, "ListFirewallDevices should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdMustLen(t, result.Data, 1, "result should include one device")
	stdCheckEqual(t, 456, result.Data[0].ID)
	stdCheckEqual(t, managedLinodeSettingsSSHUser, result.Data[0].Entity.Type)
	stdCheckEqual(t, 2, result.Page)
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
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(device))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallDevice(t.Context(), 123, 456)

	stdMustNoError(t, err, "GetFirewallDevice should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, 456, result.ID)
	stdCheckEqual(t, managedLinodeSettingsSSHUser, result.Entity.Type)
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

			stdMustErrorIs(t, err, testCase.wantErr, "invalid input should be rejected")
			stdCheckNil(t, result, "no device should be returned")
			stdCheckFalse(t, called.Load(), "client should not call API for invalid input")
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
			if !stdCheckTrue(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !stdCheckNoError(t, err) {
				return
			}

			stdCheckNoError(t, conn.Close())

			return
		}

		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.FirewallDevice{ID: 456}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallDevice(t.Context(), 123, 456)

	stdMustNoError(t, err, "GetFirewallDevice should succeed after retry")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, 456, result.ID)
	stdCheckEqual(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
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
		stdCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		stdCheckEqual(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		stdCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateFirewallDeviceRequest
		stdCheckNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be JSON")
		stdCheckEqual(t, linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser}, got)

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(device))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateFirewallDevice(t.Context(), 123, &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})

	stdMustNoError(t, err, "CreateFirewallDevice should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, 789, result.ID)
	stdCheckEqual(t, managedLinodeSettingsSSHUser, result.Entity.Type)
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

			stdMustErrorIs(t, err, testCase.wantErr, "invalid input should be rejected")
			stdCheckNil(t, result, "no device should be returned")
			stdCheckFalse(t, called.Load(), "client should not call API for invalid input")
		})
	}
}

func TestClientDeleteFirewallDeviceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		stdCheckEqual(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query params")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		stdCheckNoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteFirewallDevice(t.Context(), 123, 456)

	stdMustNoError(t, err, "DeleteFirewallDevice should succeed on 200 response")
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

			stdMustErrorIs(t, err, testCase.wantErr, "invalid input should return expected error")
			stdCheckFalse(t, called.Load(), "client should not call API for invalid input")
		})
	}
}

func TestClientDeleteFirewallDeviceHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		stdCheckEqual(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		stdCheckNoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteFirewallDevice(t.Context(), 123, 456)

	stdMustError(t, err, "DeleteFirewallDevice should fail on HTTP error")
}

func TestClientDeleteFirewallDeviceDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		stdCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		stdCheckEqual(t, endpointFirewallDeviceByID, r.URL.Path, "request path should match")

		hj, ok := w.(http.Hijacker)
		if !stdCheckTrue(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !stdCheckNoError(t, err) {
			return
		}

		stdCheckNoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	err := client.DeleteFirewallDevice(t.Context(), 123, 456)

	stdMustError(t, err, "DeleteFirewallDevice should return the transient error")
	stdCheckEqual(t, int32(1), calls.Load(), "DELETE must not be replayed after transient failure")
}

func TestClientCreateFirewallDeviceDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		stdCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		stdCheckEqual(t, endpointFirewallDevices, r.URL.Path, "request path should match")

		hj, ok := w.(http.Hijacker)
		if !stdCheckTrue(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !stdCheckNoError(t, err) {
			return
		}

		stdCheckNoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.CreateFirewallDevice(t.Context(), 123, &linode.CreateFirewallDeviceRequest{ID: 456, Type: managedLinodeSettingsSSHUser})

	stdMustError(t, err, "transient error should be returned")
	stdCheckNil(t, result, "no device should be returned")
	stdCheckEqual(t, int32(1), requestCount.Load(), "mutating POST should not be replayed")
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

	stdMustErrorIs(t, err, linode.ErrFirewallIDPositive, "invalid firewall ID should be rejected")
	stdCheckNil(t, result, "no devices should be returned")
	stdCheckFalse(t, called.Load(), "client should not call API for invalid firewall ID")
}

func TestClientListFirewallDevicesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		stdCheckNoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallDevices(t.Context(), 123, 0, 0)

	stdMustError(t, err, "ListFirewallDevices should fail on HTTP error")

	apiErr := stdAPIError(t, err, "error should wrap APIError")

	stdCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListFirewallDevicesRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hj, ok := w.(http.Hijacker)
			if !stdCheckTrue(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !stdCheckNoError(t, err) {
				return
			}

			stdCheckNoError(t, conn.Close())

			return
		}

		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointFirewallDevices, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallDevice]{
			Data: []linode.FirewallDevice{{ID: 456}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallDevices(t.Context(), 123, 0, 0)

	stdMustNoError(t, err, "ListFirewallDevices should succeed after retry")
	stdMustNotNil(t, result, "result should not be nil")
	stdMustLen(t, result.Data, 1, "result should include one device")
	stdCheckEqual(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}

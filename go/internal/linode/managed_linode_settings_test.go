package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	managedLinodeSettingsID       = 234
	managedLinodeSettingsPath     = "/managed/linode-settings/234"
	managedLinodeSettingsListPath = "/managed/linode-settings"
	managedLinodeSettingsLabel    = "linode123"
	managedLinodeSettingsGroup    = "linodes"
	managedLinodeSettingsIP       = "203.0.113.1"
	managedLinodeSettingsSSHUser  = "linode"
	managedLinodeSettingsSSHPort  = 22
)

func TestClientListManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := 2222
	user := accountMaintenanceEntityType
	settings := linode.PaginatedResponse[linode.ManagedLinodeSettings]{
		Data: []linode.ManagedLinodeSettings{{
			ID:    123,
			Label: managedLinodeSettingsLabel,
			Group: managedLinodeSettingsGroup,
			SSH: linode.ManagedLinodeSettingsSSH{
				Access: true,
				IP:     managedLinodeSettingsIP,
				Port:   &port,
				User:   &user,
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedLinodeSettingsListPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsListPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedLinodeSettings(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
	}

	if result.Data[0].Label != managedLinodeSettingsLabel {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, managedLinodeSettingsLabel)
	}

	if result.Data[0].SSH.IP != managedLinodeSettingsIP {
		t.Errorf("result.Data[0].SSH.IP = %v, want %v", result.Data[0].SSH.IP, managedLinodeSettingsIP)
	}

	if result.Data[0].SSH.Port == nil {
		t.Fatal("result.Data[0].SSH.Port is nil")
	}

	if *result.Data[0].SSH.Port != port {
		t.Errorf("*result.Data[0].SSH.Port = %v, want %v", *result.Data[0].SSH.Port, port)
	}
}

func TestClientListManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedLinodeSettingsListPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsListPath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedLinodeSettings]{
			Data:    []linode.ManagedLinodeSettings{{ID: 123, Label: managedLinodeSettingsLabel}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListManagedLinodeSettings(t.Context(), 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}

func TestClientListManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedLinodeSettingsListPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsListPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedLinodeSettings(t.Context(), 1, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, errForbidden) {
		t.Errorf("error %v is not an APIError containing %q", err, errForbidden)
	}
}

func TestClientGetManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	sshUser := managedLinodeSettingsSSHUser
	sshPort := managedLinodeSettingsSSHPort
	settings := linode.ManagedLinodeSettings{
		ID:    managedLinodeSettingsID,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH: linode.ManagedLinodeSettingsSSH{
			Access: true,
			IP:     managedLinodeSettingsIP,
			Port:   &sshPort,
			User:   &sshUser,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != managedLinodeSettingsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedLinodeSettingsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != managedLinodeSettingsID {
		t.Errorf("result.ID = %v, want %v", result.ID, managedLinodeSettingsID)
	}

	if result.Label != managedLinodeSettingsLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, managedLinodeSettingsLabel)
	}

	if result.Group != managedLinodeSettingsGroup {
		t.Errorf("result.Group = %v, want %v", result.Group, managedLinodeSettingsGroup)
	}

	if !result.SSH.Access {
		t.Error("result.SSH.Access = false, want true")
	}

	if result.SSH.IP != managedLinodeSettingsIP {
		t.Errorf("result.SSH.IP = %v, want %v", result.SSH.IP, managedLinodeSettingsIP)
	}

	if result.SSH.Port == nil {
		t.Fatal("result.SSH.Port is nil")
	}

	if *result.SSH.Port != managedLinodeSettingsSSHPort {
		t.Errorf("*result.SSH.Port = %v, want %v", *result.SSH.Port, managedLinodeSettingsSSHPort)
	}

	if result.SSH.User == nil {
		t.Fatal("result.SSH.User is nil")
	}

	if *result.SSH.User != managedLinodeSettingsSSHUser {
		t.Errorf("*result.SSH.User = %v, want %v", *result.SSH.User, managedLinodeSettingsSSHUser)
	}
}

func TestClientGetManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	settings := linode.ManagedLinodeSettings{ID: managedLinodeSettingsID, Label: managedLinodeSettingsLabel}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != managedLinodeSettingsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedLinodeSettingsPath)
		}

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", tcApplicationJSON)
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"my-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithJitter(false),
	)

	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if result.ID != managedLinodeSettingsID {
		t.Errorf("result.ID = %v, want %v", result.ID, managedLinodeSettingsID)
	}
}

func TestClientGetManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != managedLinodeSettingsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedLinodeSettingsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, "not found") {
		t.Errorf("error %v is not an APIError containing %q", err, "not found")
	}
}

func TestClientUpdateManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := managedLinodeSettingsSSHPort
	user := managedLinodeSettingsSSHUser
	settings := linode.ManagedLinodeSettings{
		ID:    managedLinodeSettingsID,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH: linode.ManagedLinodeSettingsSSH{
			Access: true,
			IP:     managedLinodeSettingsIP,
			Port:   &port,
			User:   &user,
		},
	}
	wantReq := linode.UpdateManagedLinodeSettingsRequest{
		SSH: &linode.UpdateManagedLinodeSettingsSSH{
			Access: &settings.SSH.Access,
			IP:     &settings.SSH.IP,
			Port:   settings.SSH.Port,
			User:   settings.SSH.User,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != managedLinodeSettingsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedLinodeSettingsPath)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.UpdateManagedLinodeSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, wantReq) {
			t.Errorf("got = %+v, want %+v", got, wantReq)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, wantReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual(*result, settings) {
		t.Errorf("result = %+v, want %+v", *result, settings)
	}
}

func TestClientUpdateManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != managedLinodeSettingsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedLinodeSettingsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	access := true
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &access}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, errForbidden) {
		t.Errorf("error %v is not an APIError containing %q", err, errForbidden)
	}
}

func TestClientUpdateManagedLinodeSettingsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.EscapedPath() != managedLinodeSettingsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedLinodeSettingsPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	access := true
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	_, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &access}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

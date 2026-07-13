package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

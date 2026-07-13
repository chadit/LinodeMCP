package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	managedCredentialsPath          = "/managed/credentials"
	managedCredentialsSSHKeyPath    = "/managed/credentials/sshkey"
	managedCredentialsLabel         = "prod-password-1"
	managedCredentialsLastDecrypted = "2018-01-01T00:01:01"
	managedSSHKeyValue              = "ssh-rsa managedservices-test-key"
	managedCredentialsPassword      = "stored-password-value"
	managedCredentialsUsername      = "johndoe"
)

func TestClientGetManagedSSHKeySuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsSSHKeyPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsSSHKeyPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("unexpected error: %v", readErr)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keySSHKey: managedSSHKeyValue}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetManagedSSHKeyProto(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.GetSshKey() != managedSSHKeyValue {
		t.Errorf("got.GetSshKey() = %v, want %v", got.GetSshKey(), managedSSHKeyValue)
	}
}

func TestClientGetManagedSSHKeyAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsSSHKeyPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsSSHKeyPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot access managed SSH keys"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetManagedSSHKeyProto(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &apiErr)
	}
}

func TestClientGetManagedSSHKeyRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != managedCredentialsSSHKeyPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsSSHKeyPath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keySSHKey: managedSSHKeyValue}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetManagedSSHKeyProto(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}

	if got.GetSshKey() != managedSSHKeyValue {
		t.Errorf("got.GetSshKey() = %v, want %v", got.GetSshKey(), managedSSHKeyValue)
	}
}

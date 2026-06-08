package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedCredentialUsernamePasswordUpdatePath      = "/managed/credentials/9991/update"
	managedCredentialUsernamePasswordUpdateLabel     = "prod-password-1"
	managedCredentialUsernamePasswordUpdateTimestamp = "2018-01-01T00:01:01"
	managedCredentialUsernamePasswordUpdatePassword  = "stored-password-value"
	managedCredentialUsernamePasswordUpdateUsername  = "johndoe"
)

func TestClientUpdateManagedCredentialUsernamePasswordSuccess(t *testing.T) {
	t.Parallel()

	username := managedCredentialUsernamePasswordUpdateUsername
	request := &linode.UpdateManagedCredentialUsernamePasswordRequest{
		Password: managedCredentialUsernamePasswordUpdatePassword,
		Username: &username,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialUsernamePasswordUpdatePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialUsernamePasswordUpdatePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got["password"], managedCredentialUsernamePasswordUpdatePassword) {
			t.Errorf("got %v, want %v", got["password"], managedCredentialUsernamePasswordUpdatePassword)
		}

		if !reflect.DeepEqual(got["username"], managedCredentialUsernamePasswordUpdateUsername) {
			t.Errorf("got %v, want %v", got["username"], managedCredentialUsernamePasswordUpdateUsername)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialUsernamePasswordUpdateLabel,
			LastDecrypted: managedCredentialUsernamePasswordUpdateTimestamp,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 9991 {
		t.Errorf("got.ID = %v, want %v", got.ID, 9991)
	}

	if got.Label != managedCredentialUsernamePasswordUpdateLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, managedCredentialUsernamePasswordUpdateLabel)
	}

	if got.LastDecrypted != managedCredentialUsernamePasswordUpdateTimestamp {
		t.Errorf("got.LastDecrypted = %v, want %v", got.LastDecrypted, managedCredentialUsernamePasswordUpdateTimestamp)
	}
}

func TestClientUpdateManagedCredentialUsernamePasswordNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &netErr)
	}

	if netErr.Operation != "UpdateManagedCredentialUsernamePassword" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "UpdateManagedCredentialUsernamePassword")
	}
}

func TestClientUpdateManagedCredentialUsernamePasswordAPIError(t *testing.T) {
	t.Parallel()

	request := &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialUsernamePasswordUpdatePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialUsernamePasswordUpdatePath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot update managed credential username/password"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, request)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &apiErr)
	}
}

func TestClientUpdateManagedCredentialUsernamePasswordDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialUsernamePasswordUpdatePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialUsernamePasswordUpdatePath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

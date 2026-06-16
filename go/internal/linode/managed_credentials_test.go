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
	"time"

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

func TestClientListManagedCredentialsSuccess(t *testing.T) {
	t.Parallel()

	credentials := linode.PaginatedResponse[linode.ManagedCredential]{
		Data: []linode.ManagedCredential{{
			ID:            9991,
			Label:         managedCredentialsLabel,
			LastDecrypted: managedCredentialsLastDecrypted,
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(credentials); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListManagedCredentials(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Page != 2 {
		t.Errorf("got.Page = %v, want %v", got.Page, 2)
	}

	if got.Data[0].Label != managedCredentialsLabel {
		t.Errorf("got.Data[0].Label = %v, want %v", got.Data[0].Label, managedCredentialsLabel)
	}

	if got.Data[0].LastDecrypted != managedCredentialsLastDecrypted {
		t.Errorf("got.Data[0].LastDecrypted = %v, want %v", got.Data[0].LastDecrypted, managedCredentialsLastDecrypted)
	}
}

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

		if err := json.NewEncoder(w).Encode(linode.ManagedSSHKey{SSHKey: managedSSHKeyValue}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetManagedSSHKey(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.SSHKey != managedSSHKeyValue {
		t.Errorf("got.SSHKey = %v, want %v", got.SSHKey, managedSSHKeyValue)
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

	_, err := client.GetManagedSSHKey(t.Context())
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

		if err := json.NewEncoder(w).Encode(linode.ManagedSSHKey{SSHKey: managedSSHKeyValue}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetManagedSSHKey(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}

	if got.SSHKey != managedSSHKeyValue {
		t.Errorf("got.SSHKey = %v, want %v", got.SSHKey, managedSSHKeyValue)
	}
}

func TestClientListManagedCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot access managed credentials"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListManagedCredentials(t.Context(), 1, 25)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &apiErr)
	}
}

func TestClientListManagedCredentialsRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != managedCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath)
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

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedCredential]{
			Data:    []linode.ManagedCredential{{ID: 9991, Label: managedCredentialsLabel}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListManagedCredentials(t.Context(), 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientCreateManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	username := managedCredentialsUsername
	request := &linode.CreateManagedCredentialRequest{
		Label:    managedCredentialsLabel,
		Password: managedCredentialsPassword,
		Username: &username,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath)
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

		if !reflect.DeepEqual(got["label"], managedCredentialsLabel) {
			t.Errorf("got %v, want %v", got["label"], managedCredentialsLabel)
		}

		if !reflect.DeepEqual(got["password"], managedCredentialsPassword) {
			t.Errorf("got %v, want %v", got["password"], managedCredentialsPassword)
		}

		if !reflect.DeepEqual(got["username"], managedCredentialsUsername) {
			t.Errorf("got %v, want %v", got["username"], managedCredentialsUsername)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialsLabel,
			LastDecrypted: managedCredentialsLastDecrypted,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateManagedCredential(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 9991 {
		t.Errorf("got.ID = %v, want %v", got.ID, 9991)
	}

	if got.Label != managedCredentialsLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, managedCredentialsLabel)
	}

	if got.LastDecrypted != managedCredentialsLastDecrypted {
		t.Errorf("got.LastDecrypted = %v, want %v", got.LastDecrypted, managedCredentialsLastDecrypted)
	}
}

func TestClientCreateManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	request := &linode.CreateManagedCredentialRequest{
		Label:    managedCredentialsLabel,
		Password: managedCredentialsPassword,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot create managed credentials"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateManagedCredential(t.Context(), request)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &apiErr)
	}
}

func TestClientCreateManagedCredentialDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	_, err := client.CreateManagedCredential(t.Context(), &linode.CreateManagedCredentialRequest{
		Label:    managedCredentialsLabel,
		Password: managedCredentialsPassword,
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientUpdateManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	label := "prod-password-2"
	updated := linode.ManagedCredential{ID: 9991, Label: label, LastDecrypted: managedCredentialsLastDecrypted}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedCredentialsPath+"/9991" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath+"/9991")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var got linode.UpdateManagedCredentialRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label == nil {
			t.Fatal("got.Label is nil")
		}

		if *got.Label != label {
			t.Errorf("*got.Label = %v, want %v", *got.Label, label)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 9991 {
		t.Errorf("got.ID = %v, want %v", got.ID, 9991)
	}

	if got.Label != label {
		t.Errorf("got.Label = %v, want %v", got.Label, label)
	}
}

func TestClientUpdateManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedCredentialsPath+"/9991" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath+"/9991")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot update managed credentials"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	label := managedCredentialsLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &apiErr)
	}
}

func TestClientUpdateManagedCredentialNetworkError(t *testing.T) {
	t.Parallel()

	label := managedCredentialsLabel
	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &netErr)
	}

	if netErr.Operation != "UpdateManagedCredential" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "UpdateManagedCredential")
	}
}

func TestClientUpdateManagedCredentialDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != managedCredentialsPath+"/9991" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsPath+"/9991")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	label := managedCredentialsLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	_, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedCredentialID            = 9991
	managedCredentialLabel         = "prod-password-1"
	managedCredentialLastDecrypted = "2018-01-01T00:01:01"
	managedCredentialPath          = "/managed/credentials/9991"
)

func TestClientGetManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{ID: managedCredentialID, Label: managedCredentialLabel, LastDecrypted: managedCredentialLastDecrypted}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetManagedCredential(t.Context(), managedCredentialID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != managedCredentialID {
		t.Errorf("got.ID = %v, want %v", got.ID, managedCredentialID)
	}

	if got.Label != managedCredentialLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, managedCredentialLabel)
	}

	if got.LastDecrypted != managedCredentialLastDecrypted {
		t.Errorf("got.LastDecrypted = %v, want %v", got.LastDecrypted, managedCredentialLastDecrypted)
	}
}

func TestClientGetManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != managedCredentialPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "access denied"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetManagedCredential(t.Context(), managedCredentialID)
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

func TestClientGetManagedCredentialRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != managedCredentialPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialPath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{ID: managedCredentialID, Label: managedCredentialLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	got, err := client.GetManagedCredential(t.Context(), managedCredentialID)
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

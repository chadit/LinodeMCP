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

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedContactID         = 174
	managedContactPath       = "/managed/contacts/174"
	managedContactGetName    = "John Doe"
	managedContactGetEmail   = "john.doe@example.org"
	managedContactGetPhone   = "123-456-7890"
	managedContactGetUpdated = "2018-01-01T00:01:01"
)

func TestClientGetManagedContactSuccess(t *testing.T) {
	t.Parallel()

	phone := managedContactGetPhone
	contact := linode.ManagedContact{
		ID:      managedContactID,
		Name:    managedContactGetName,
		Email:   managedContactGetEmail,
		Phone:   linode.ManagedContactPhone{Primary: &phone},
		Updated: managedContactGetUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactPath)
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

		if err := json.NewEncoder(w).Encode(contact); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetManagedContact(t.Context(), managedContactID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != managedContactID {
		t.Errorf("result.ID = %v, want %v", result.ID, managedContactID)
	}

	if result.Name != managedContactGetName {
		t.Errorf("result.Name = %v, want %v", result.Name, managedContactGetName)
	}

	if result.Email != managedContactGetEmail {
		t.Errorf("result.Email = %v, want %v", result.Email, managedContactGetEmail)
	}

	if result.Phone.Primary == nil {
		t.Fatal("result.Phone.Primary is nil")
	}

	if *result.Phone.Primary != managedContactGetPhone {
		t.Errorf("*result.Phone.Primary = %v, want %v", *result.Phone.Primary, managedContactGetPhone)
	}
}

func TestClientGetManagedContactRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	contact := linode.ManagedContact{ID: managedContactID, Name: managedContactGetName, Email: managedContactGetEmail}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactPath)
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

		if err := json.NewEncoder(w).Encode(contact); err != nil {
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

	result, err := client.GetManagedContact(t.Context(), managedContactID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if result.ID != managedContactID {
		t.Errorf("result.ID = %v, want %v", result.ID, managedContactID)
	}
}

func TestClientGetManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != managedContactPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), managedContactPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetManagedContact(t.Context(), managedContactID)
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

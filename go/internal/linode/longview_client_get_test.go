package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientGetLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          789,
			keyLabel:       longviewClientLabel,
			"api_key":      "secret-api-key",
			"install_code": "secret-install-code",
			"apps": map[string]bool{
				"apache":            true,
				databaseEngineMySQL: true,
				"nginx":             false,
			},
			keyCreated: longviewClientCreated,
			keyUpdated: longviewClientUpdated,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetLongviewClient(t.Context(), "789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 789 {
		t.Errorf("result.ID = %v, want %v", result.ID, 789)
	}

	if result.Label != longviewClientLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, longviewClientLabel)
	}

	if !result.Apps.Apache {
		t.Error("result.Apps.Apache = false, want true")
	}

	if !result.Apps.MySQL {
		t.Error("result.Apps.MySQL = false, want true")
	}

	if result.Apps.Nginx {
		t.Error("result.Apps.Nginx = true, want false")
	}

	if result.Created != longviewClientCreated {
		t.Errorf("result.Created = %v, want %v", result.Created, longviewClientCreated)
	}

	if result.Updated != longviewClientUpdated {
		t.Errorf("result.Updated = %v, want %v", result.Updated, longviewClientUpdated)
	}
}

func TestClientGetLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetLongviewClient(t.Context(), "789")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetLongviewClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/longview/clients/123%2F.." {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/longview/clients/123%2F..")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.LongviewClient{ID: 123, Label: "client123"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetLongviewClient(t.Context(), "123/..")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetLongviewClientRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.LongviewClient{ID: 789, Label: longviewClientLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetLongviewClient(t.Context(), "789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if result.ID != 789 {
		t.Errorf("result.ID = %v, want %v", result.ID, 789)
	}
}

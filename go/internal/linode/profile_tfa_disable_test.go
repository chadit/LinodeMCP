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

func TestClientDisableProfileTFASuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTfaDisable {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTfaDisable)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.ContentLength != int64(0) {
			t.Errorf("r.ContentLength = %v, want %v", r.ContentLength, int64(0))
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DisableProfileTFA(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDisableProfileTFAAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTfaDisable {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTfaDisable)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DisableProfileTFA(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}
}

func TestClientDisableProfileTFANoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTfaDisable {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTfaDisable)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.DisableProfileTFA(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

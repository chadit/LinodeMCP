package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	keyTFAConfirmExpiry = "expiry"
	tfaConfirmExpiry    = "2026-01-01T00:00:00"
)

func TestClientConfirmProfileTFAEnableSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTfaEnableConfirm {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTfaEnableConfirm)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body, map[string]string{"tfa_code": "123456"}) {
			t.Errorf("body = %v, want %v", body, map[string]string{"tfa_code": "123456"})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"scratch": "setup-token", keyTFAConfirmExpiry: tfaConfirmExpiry}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result["scratch"], "setup-token") {
		t.Errorf("got %v, want %v", result["scratch"], "setup-token")
	}

	if !reflect.DeepEqual(result[keyTFAConfirmExpiry], tfaConfirmExpiry) {
		t.Errorf("result[keyTFAConfirmExpiry] = %v, want %v", result[keyTFAConfirmExpiry], tfaConfirmExpiry)
	}
}

func TestClientConfirmProfileTFAEnableAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTfaEnableConfirm {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTfaEnableConfirm)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	_, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}
}

func TestClientConfirmProfileTFAEnableNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTfaEnableConfirm {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTfaEnableConfirm)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

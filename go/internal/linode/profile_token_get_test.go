package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const profileTokenTestLabel = "api-token"

func TestClientGetProfileTokenProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
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

		// The API may echo a token secret; the metadata-only proto has no secret
		// field, so the DiscardUnknown decode drops it.
		if err := json.NewEncoder(w).Encode(map[string]any{keyID: float64(12345), keyLabel: profileTokenTestLabel, profileTokenScopesKey: "*", "token": "super-secret-token-value"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfileTokenProto(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetId() != int32(12345) {
		t.Errorf("result.GetId() = %v, want %v", result.GetId(), int32(12345))
	}

	if result.GetLabel() != profileTokenTestLabel {
		t.Errorf("result.GetLabel() = %v, want %v", result.GetLabel(), profileTokenTestLabel)
	}

	if result.GetScopes() != "*" {
		t.Errorf("result.GetScopes() = %v, want %v", result.GetScopes(), "*")
	}
}

func TestClientGetProfileTokenProtoEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: float64(12345), keyLabel: profileTokenTestLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileTokenProto(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetProfileTokenProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileTokenProto(t.Context(), 12345)
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

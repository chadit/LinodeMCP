package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientGetProfileTokenSuccess(t *testing.T) {
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

		if err := json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token", profileTokenScopesKey: "*"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfileToken(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	tokenID, ok := (*result)[keyID].(float64)
	if !ok {
		t.Fatalf("token ID should decode as a number")
	}

	if tokenID != float64(12345) {
		t.Errorf("tokenID = %v, want %v", tokenID, float64(12345))
	}

	if !reflect.DeepEqual((*result)[keyLabel], "api-token") {
		t.Errorf("(*result)[keyLabel] = %v, want %v", (*result)[keyLabel], "api-token")
	}

	if !reflect.DeepEqual((*result)[profileTokenScopesKey], "*") {
		t.Errorf("(*result)[profileTokenScopesKey] = %v, want %v", (*result)[profileTokenScopesKey], "*")
	}
}

func TestClientGetProfileTokenEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileToken(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetProfileTokenAPIError(t *testing.T) {
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

	_, err := client.GetProfileToken(t.Context(), 12345)
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

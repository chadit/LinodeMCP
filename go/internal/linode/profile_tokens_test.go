package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	profileTokenLabel         = "ci-token"
	profileTokenUpdatedScopes = "linodes:read_write"
)

func TestClientListProfileTokensSuccess(t *testing.T) {
	t.Parallel()

	tokens := linode.PaginatedResponse[linode.ProfileToken]{
		Data: []linode.ProfileToken{{
			keyID:    float64(67890),
			keyLabel: profileTokenLabel,
			"scopes": "linodes:read_only",
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
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

		if err := json.NewEncoder(w).Encode(tokens); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileTokens(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 1 {
		t.Errorf("result.Page = %v, want %v", result.Page, 1)
	}

	if result.Pages != 1 {
		t.Errorf("result.Pages = %v, want %v", result.Pages, 1)
	}

	if result.Results != 1 {
		t.Errorf("result.Results = %v, want %v", result.Results, 1)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if !reflect.DeepEqual(result.Data[0][keyID], float64(67890)) {
		t.Errorf("result.Data[0][keyID] = %v, want %v", result.Data[0][keyID], float64(67890))
	}

	if !reflect.DeepEqual(result.Data[0][keyLabel], profileTokenLabel) {
		t.Errorf("result.Data[0][keyLabel] = %v, want %v", result.Data[0][keyLabel], profileTokenLabel)
	}
}

func TestClientListProfileTokensPagination(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
			Page: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileTokens(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}
}

func TestClientListProfileTokensAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileTokens(t.Context(), 0, 0)
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

func TestClientListProfileTokensRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileTokens(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientUpdateProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should be JSON: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], profileTokenLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], profileTokenLabel)
		}

		if !reflect.DeepEqual(body["scopes"], profileTokenUpdatedScopes) {
			t.Errorf("got %v, want %v", body["scopes"], profileTokenUpdatedScopes)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: profileTokenLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel, "scopes": profileTokenUpdatedScopes})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual((*result)[keyID], float64(12345)) {
		t.Errorf("(*result)[keyID] = %v, want %v", (*result)[keyID], float64(12345))
	}

	if !reflect.DeepEqual((*result)[keyLabel], profileTokenLabel) {
		t.Errorf("(*result)[keyLabel] = %v, want %v", (*result)[keyLabel], profileTokenLabel)
	}
}

func TestClientUpdateProfileTokenEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/profile/tokens/12%2F..%2F34%3Fx=1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/profile/tokens/12%2F..%2F34%3Fx=1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345)}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfileToken(t.Context(), "12/../34?x=1", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientUpdateProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUpdateProfileTokenNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

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

func TestClientCreateLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.CreatedLongviewClient{
		APIKey:      longviewClientAPIKey,
		Apps:        linode.LongviewApps{Apache: true, MySQL: true},
		Created:     longviewClientCreated,
		ID:          789,
		InstallCode: longviewClientInstallCode,
		Label:       longviewClientLabel,
		Updated:     longviewClientUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/longview/clients" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/longview/clients")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var got linode.CreateLongviewClientRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label != longviewClientLabel {
			t.Errorf("got.Label = %v, want %v", got.Label, longviewClientLabel)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	got, err := client.CreateLongviewClient(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !reflect.DeepEqual(*got, want) {
		t.Errorf("*got = %v, want %v", *got, want)
	}
}

func TestClientCreateLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/longview/clients" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/longview/clients")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	_, err := client.CreateLongviewClient(t.Context(), req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientCreateLongviewClientDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		http.Error(w, "transient", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	_, err := client.CreateLongviewClient(t.Context(), req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusInternalServerError)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

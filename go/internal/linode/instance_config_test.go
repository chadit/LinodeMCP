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

func TestClientGetInstanceConfigInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	primary := true
	response := linode.ConfigInterfaceResponse{ID: 456, Active: true, Purpose: purposeVPC, Primary: primary}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetInstanceConfigInterface(t.Context(), 123, 789, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Purpose != response.Purpose {
		t.Errorf("got.Purpose = %v, want %v", got.Purpose, response.Purpose)
	}

	if got.ID != response.ID {
		t.Errorf("got.ID = %v, want %v", got.ID, response.ID)
	}

	if !got.Active {
		t.Error("got.Active = false, want true")
	}
}

func TestClientGetInstanceConfigInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceConfigInterface(t.Context(), 0, 789, 456)
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.GetInstanceConfigInterface(t.Context(), 123, 0, 456)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	_, err = client.GetInstanceConfigInterface(t.Context(), 123, 789, 0)
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	_, err = client.GetInstanceConfigInterface(t.Context(), 123, 789, -1)
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
	}
}

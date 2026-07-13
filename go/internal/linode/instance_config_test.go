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

func TestClientDeleteInstanceConfigSendsRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteInstanceConfig(t.Context(), 123, 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteInstanceConfigDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != tcLinodeInstances123Configs789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789)
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteInstanceConfig(t.Context(), 123, 789)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientDeleteInstanceConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceConfig(t.Context(), 0, 789)

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	err = client.DeleteInstanceConfig(t.Context(), 123, 0)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
	}
}

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

func TestClientDeleteInstanceConfigInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteInstanceConfigInterface(t.Context(), 123, 789, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteInstanceConfigInterfaceDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteInstanceConfigInterface(t.Context(), 123, 789, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientDeleteInstanceConfigInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteInstanceConfigInterface(t.Context(), 0, 789, 456)
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	err = client.DeleteInstanceConfigInterface(t.Context(), 123, 0, 456)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	err = client.DeleteInstanceConfigInterface(t.Context(), 123, 789, 0)
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	err = client.DeleteInstanceConfigInterface(t.Context(), 123, 789, -1)
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	if called.Load() {
		t.Fatalf("invalid IDs should not issue HTTP request")
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

func TestClientReorderInstanceConfigInterfacesSendsRequest(t *testing.T) {
	t.Parallel()

	wantReq := linode.ReorderConfigInterfacesRequest{IDs: []int{101, 102, 103}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces/order" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces/order")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var got linode.ReorderConfigInterfacesRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(got, wantReq) {
			t.Errorf("got = %v, want %v", got, wantReq)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.ReorderInstanceConfigInterfaces(t.Context(), 123, 789, &wantReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientReorderInstanceConfigInterfacesDoesNotRetryReorder(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.ReorderInstanceConfigInterfaces(t.Context(), 123, 789, &linode.ReorderConfigInterfacesRequest{IDs: []int{101}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientReorderInstanceConfigInterfacesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called.Store(true)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.ReorderInstanceConfigInterfaces(t.Context(), 0, 789, &linode.ReorderConfigInterfacesRequest{IDs: []int{101}})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	err = client.ReorderInstanceConfigInterfaces(t.Context(), 123, 0, &linode.ReorderConfigInterfacesRequest{IDs: []int{101}})
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	err = client.ReorderInstanceConfigInterfaces(t.Context(), 123, 789, nil)
	if !errors.Is(err, linode.ErrReorderConfigInterfacesRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrReorderConfigInterfacesRequestRequired)
	}

	if called.Load() {
		t.Fatalf("invalid input should not issue HTTP request")
	}
}

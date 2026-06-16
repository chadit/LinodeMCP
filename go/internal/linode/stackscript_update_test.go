package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	stackScriptIDFixture      = 12345
	stackScriptLabelFixture   = "test-script"
	stackScriptUpdatedFixture = "2025-04-16T22:44:02"
)

func TestClientCreateStackScriptDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateStackScript(t.Context(), &linode.CreateStackScriptRequest{Label: stackScriptLabelFixture, Script: "#!/bin/bash", Images: []string{privateImage15Fixture}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientUpdateStackScriptSuccess(t *testing.T) {
	t.Parallel()

	label := stackScriptLabelFixture
	script := "#!/bin/bash\necho updated"
	description := "updated StackScript"
	isPublic := true
	revNote := "update revision"
	request := &linode.UpdateStackScriptRequest{
		Label:       &label,
		Script:      &script,
		Images:      []string{privateImage15Fixture},
		Description: &description,
		IsPublic:    &isPublic,
		RevNote:     &revNote,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/linode/stackscripts/12345" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/stackscripts/12345")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], label) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], label)
		}

		if !reflect.DeepEqual(body["script"], script) {
			t.Errorf("got %v, want %v", body["script"], script)
		}

		if !reflect.DeepEqual(body["images"], []any{privateImage15Fixture}) {
			t.Errorf("got %v, want %v", body["images"], []any{privateImage15Fixture})
		}

		if !reflect.DeepEqual(body[keyDescription], description) {
			t.Errorf("body[keyDescription] = %v, want %v", body[keyDescription], description)
		}

		if !reflect.DeepEqual(body["is_public"], isPublic) {
			t.Errorf("got %v, want %v", body["is_public"], isPublic)
		}

		if !reflect.DeepEqual(body["rev_note"], revNote) {
			t.Errorf("got %v, want %v", body["rev_note"], revNote)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.StackScript{
			ID:       stackScriptIDFixture,
			Label:    label,
			Script:   script,
			Images:   []string{privateImage15Fixture},
			Updated:  stackScriptUpdatedFixture,
			RevNote:  revNote,
			IsPublic: isPublic,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != stackScriptIDFixture {
		t.Errorf("result.ID = %v, want %v", result.ID, stackScriptIDFixture)
	}

	if result.Label != label {
		t.Errorf("result.Label = %v, want %v", result.Label, label)
	}
}

func TestClientUpdateStackScriptRejectsInvalidID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateStackScript(t.Context(), 0, &linode.UpdateStackScriptRequest{})
	if !errors.Is(err, linode.ErrStackScriptIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrStackScriptIDPositive)
	}

	if called.Load() != false {
		t.Errorf("called.Load() = %v, want %v", called.Load(), false)
	}
}

func TestClientUpdateStackScriptRejectsEmptyRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, nil)
	if !errors.Is(err, linode.ErrStackScriptUpdateRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrStackScriptUpdateRequired)
	}

	if called.Load() != false {
		t.Errorf("called.Load() = %v, want %v", called.Load(), false)
	}
}

func TestClientUpdateStackScriptAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "script is invalid"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != "script is invalid" {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, "script is invalid")
	}
}

func TestClientUpdateStackScriptNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.NetworkError", err)
	}

	if networkErr.Operation != "UpdateStackScript" {
		t.Errorf("networkErr.Operation = %v, want %v", networkErr.Operation, "UpdateStackScript")
	}
}

func TestClientUpdateStackScriptHonorsCircuitBreakerWithoutRetry(t *testing.T) {
	t.Parallel()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Resilience: config.ResilienceConfig{CircuitBreakerThreshold: 1, CircuitBreakerTimeout: time.Hour}}
	client := linode.NewClient(srv.URL, "test-token", cfg, linode.WithMaxRetries(2))

	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}

	_, err = client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientUpdateStackScriptDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

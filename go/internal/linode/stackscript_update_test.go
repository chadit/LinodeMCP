package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
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
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.CreateStackScript(t.Context(), &linode.CreateStackScriptRequest{Label: stackScriptLabelFixture, Script: "#!/bin/bash", Images: []string{privateImage15Fixture}})

	requireError(t, err, "CreateStackScript should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "CreateStackScript must not retry and replay a mutating request")
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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/linode/stackscripts/12345", r.URL.Path, "request path should include StackScript ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		checkEqual(t, label, body[keyLabel])
		checkEqual(t, script, body["script"])
		checkEqual(t, []any{privateImage15Fixture}, body["images"])
		checkEqual(t, description, body[keyDescription])
		checkEqual(t, isPublic, body["is_public"])
		checkEqual(t, revNote, body["rev_note"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.StackScript{
			ID:       stackScriptIDFixture,
			Label:    label,
			Script:   script,
			Images:   []string{privateImage15Fixture},
			Updated:  stackScriptUpdatedFixture,
			RevNote:  revNote,
			IsPublic: isPublic,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, stackScriptIDFixture, result.ID)
	checkEqual(t, label, result.Label)
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

	requireError(t, err, "UpdateStackScript should reject invalid IDs")
	requireErrorIs(t, err, linode.ErrStackScriptIDPositive, "error should expose invalid StackScript ID sentinel")
	checkEqual(t, false, called.Load(), "invalid StackScript ID should not reach upstream server")
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

	requireError(t, err, "UpdateStackScript should reject nil update requests")
	requireErrorIs(t, err, linode.ErrStackScriptUpdateRequired, "error should expose empty update sentinel")
	checkEqual(t, false, called.Load(), "empty StackScript update should not reach upstream server")
}

func TestClientUpdateStackScriptAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "script is invalid"}},
		}))
	}))
	defer srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	apiErr := requireAPIError(t, err, "UpdateStackScript should return API errors")
	checkEqual(t, "script is invalid", apiErr.Message)
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

	networkErr := requireNetworkError(t, err, "UpdateStackScript should wrap network errors")
	checkEqual(t, "UpdateStackScript", networkErr.Operation)
}

func TestClientUpdateStackScriptHonorsCircuitBreakerWithoutRetry(t *testing.T) {
	t.Parallel()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{Resilience: config.ResilienceConfig{CircuitBreakerThreshold: 1, CircuitBreakerTimeout: time.Hour}}
	client := linode.NewClient(srv.URL, "test-token", cfg, linode.WithMaxRetries(2))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	requireError(t, err, "first UpdateStackScript should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UpdateStackScript must not retry and replay a mutating request")

	_, err = client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	requireError(t, err, "second UpdateStackScript should be blocked by circuit breaker")
	requireErrorIs(t, err, linode.ErrCircuitOpen, "open breaker rejects without calling upstream")
	checkEqual(t, int32(1), calls.Load(), "open breaker must not invoke upstream")
}

func TestClientUpdateStackScriptDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	requireError(t, err, "UpdateStackScript should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UpdateStackScript must not retry and replay a mutating request")
}

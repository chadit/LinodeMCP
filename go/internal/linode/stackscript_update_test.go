package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.CreateStackScript(t.Context(), &linode.CreateStackScriptRequest{Label: stackScriptLabelFixture, Script: "#!/bin/bash", Images: []string{privateImage15Fixture}})

	require.Error(t, err, "CreateStackScript should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "CreateStackScript must not retry and replay a mutating request")
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
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/linode/stackscripts/12345", r.URL.Path, "request path should include StackScript ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, label, body[keyLabel])
		assert.Equal(t, script, body["script"])
		assert.Equal(t, []any{privateImage15Fixture}, body["images"])
		assert.Equal(t, description, body[keyDescription])
		assert.Equal(t, isPublic, body["is_public"])
		assert.Equal(t, revNote, body["rev_note"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.StackScript{
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

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, stackScriptIDFixture, result.ID)
	assert.Equal(t, label, result.Label)
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

	require.Error(t, err, "UpdateStackScript should reject invalid IDs")
	require.ErrorIs(t, err, linode.ErrStackScriptIDPositive, "error should expose invalid StackScript ID sentinel")
	assert.False(t, called.Load(), "invalid StackScript ID should not reach upstream server")
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

	require.Error(t, err, "UpdateStackScript should reject nil update requests")
	require.ErrorIs(t, err, linode.ErrStackScriptUpdateRequired, "error should expose empty update sentinel")
	assert.False(t, called.Load(), "empty StackScript update should not reach upstream server")
}

func TestClientUpdateStackScriptAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "script is invalid"}},
		}))
	}))
	defer srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	require.Error(t, err, "UpdateStackScript should return API errors")
	assert.ErrorContains(t, err, "script is invalid")
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

	require.Error(t, err, "UpdateStackScript should wrap network errors")

	var networkErr *linode.NetworkError
	require.ErrorAs(t, err, &networkErr, "network error should wrap as NetworkError")
	assert.Equal(t, "UpdateStackScript", networkErr.Operation)
}

func TestClientUpdateStackScriptHonorsCircuitBreakerWithoutRetry(t *testing.T) {
	t.Parallel()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{Resilience: config.ResilienceConfig{CircuitBreakerThreshold: 1, CircuitBreakerTimeout: time.Hour}}
	client := linode.NewClient(srv.URL, "test-token", cfg, linode.WithMaxRetries(2))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	require.Error(t, err, "first UpdateStackScript should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "UpdateStackScript must not retry and replay a mutating request")

	_, err = client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)
	require.Error(t, err, "second UpdateStackScript should be blocked by circuit breaker")
	require.ErrorIs(t, err, linode.ErrCircuitOpen, "open breaker rejects without calling upstream")
	assert.Equal(t, int32(1), calls.Load(), "open breaker must not invoke upstream")
}

func TestClientUpdateStackScriptDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	label := stackScriptLabelFixture
	request := &linode.UpdateStackScriptRequest{Label: &label}

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateStackScript(t.Context(), stackScriptIDFixture, request)

	require.Error(t, err, "UpdateStackScript should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "UpdateStackScript must not retry and replay a mutating request")
}

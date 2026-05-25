package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          789,
			keyLabel:       longviewClientLabel,
			"api_key":      "secret-api-key",
			"install_code": "secret-install-code",
			"apps": map[string]bool{
				"apache":            true,
				databaseEngineMySQL: true,
				"nginx":             false,
			},
			keyCreated: longviewClientCreated,
			keyUpdated: longviewClientUpdated,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetLongviewClient(t.Context(), "789")

	require.NoError(t, err, "GetLongviewClient should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 789, result.ID)
	assert.Equal(t, longviewClientLabel, result.Label)
	assert.True(t, result.Apps.Apache)
	assert.True(t, result.Apps.MySQL)
	assert.False(t, result.Apps.Nginx)
	assert.Equal(t, longviewClientCreated, result.Created)
	assert.Equal(t, longviewClientUpdated, result.Updated)
}

func TestClientGetLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetLongviewClient(t.Context(), "789")

	require.Error(t, err, "GetLongviewClient should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetLongviewClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/longview/clients/123%2F..", r.URL.EscapedPath(), "client ID should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.LongviewClient{ID: 123, Label: "client123"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetLongviewClient(t.Context(), "123/..")

	require.NoError(t, err, "client should escape path separators before sending request")
}

func TestClientGetLongviewClientRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.LongviewClient{ID: 789, Label: longviewClientLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetLongviewClient(t.Context(), "789")

	require.NoError(t, err, "GetLongviewClient should retry transient failures")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, int32(2), calls.Load(), "transient error should be retried once")
	assert.Equal(t, 789, result.ID)
}

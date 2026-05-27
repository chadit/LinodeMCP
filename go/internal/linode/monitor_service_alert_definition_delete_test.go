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

const (
	monitorServiceAlertDefinitionDeletePath        = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionEscapedDeletePath = "/monitor/services/dbaas%2Fpostgres/alert-definitions/20000"
)

func TestClientDeleteMonitorServiceAlertDefinitionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, monitorServiceAlertDefinitionDeletePath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)

	require.NoError(t, err)
}

func TestClientDeleteMonitorServiceAlertDefinitionEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, monitorServiceAlertDefinitionEscapedDeletePath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeWithSlash, monitorAlertDefinitionID)

	require.NoError(t, err)
}

func TestClientDeleteMonitorServiceAlertDefinitionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, monitorServiceAlertDefinitionDeletePath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientDeleteMonitorServiceAlertDefinitionDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, monitorServiceAlertDefinitionDeletePath, r.URL.Path, "request path should match")
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)

	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "destructive route must not retry after transient failure")
}

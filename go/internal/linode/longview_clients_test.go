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
	longviewClientAPIKey      = "longview-api-key-secret"
	longviewClientInstallCode = "longview-install-code-secret"
	longviewClientLabel       = "client789"
	longviewClientsPath       = "/longview/clients"
)

func TestClientListLongviewClientsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewClientsPath, r.URL.Path, "request path should match")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				"api_key":      longviewClientAPIKey,
				"apps":         map[string]bool{"apache": true, databaseEngineMySQL: true, "nginx": false},
				keyCreated:     "2018-01-01T00:01:01",
				keyID:          789,
				"install_code": longviewClientInstallCode,
				keyLabel:       longviewClientLabel,
				"updated":      "2018-01-02T00:01:01",
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewClients(t.Context(), 2, 25)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Data, 1)
	assert.Equal(t, longviewClientLabel, got.Data[0].Label)
	assert.True(t, got.Data[0].Apps.Apache)
	assert.True(t, got.Data[0].Apps.MySQL)
	assert.False(t, got.Data[0].Apps.Nginx)
}

func TestClientListLongviewClientsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewClientsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewClients(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListLongviewClientsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewClientsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 789, keyLabel: longviewClientLabel}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListLongviewClients(t.Context(), 0, 0)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Data, 1)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
}

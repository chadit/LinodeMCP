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

const managedLinodeSettingsPath = "/managed/linode-settings"

func TestClientListManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := 2222
	user := accountMaintenanceEntityType
	settings := linode.PaginatedResponse[linode.ManagedLinodeSettings]{
		Data: []linode.ManagedLinodeSettings{{
			ID:    123,
			Label: "linode123",
			Group: "linodes",
			SSH: linode.ManagedLinodeSettingsSSH{
				Access: true,
				IP:     "203.0.113.1",
				Port:   &port,
				User:   &user,
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.Path, "request path should match managed Linode settings endpoint")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedLinodeSettings(t.Context(), 2, 25)

	require.NoError(t, err, "ListManagedLinodeSettings should succeed")
	require.NotNil(t, result, "settings should be returned")
	require.Len(t, result.Data, 1, "one setting should be returned")
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, "linode123", result.Data[0].Label)
	assert.Equal(t, "203.0.113.1", result.Data[0].SSH.IP)
	require.NotNil(t, result.Data[0].SSH.Port)
	assert.Equal(t, port, *result.Data[0].SSH.Port)
}

func TestClientListManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.Path, "request path should match managed Linode settings endpoint")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedLinodeSettings]{
			Data:    []linode.ManagedLinodeSettings{{ID: 123, Label: "linode123"}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListManagedLinodeSettings(t.Context(), 1, 0)

	require.NoError(t, err, "read-only list should retry transient failures")
	require.NotNil(t, result, "settings should be returned after retry")
	assert.Equal(t, int32(2), calls.Load(), "request should retry once")
}

func TestClientListManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.Path, "request path should match managed Linode settings endpoint")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedLinodeSettings(t.Context(), 1, 0)

	require.Error(t, err, "ListManagedLinodeSettings should fail on API error")
	assert.Nil(t, result, "settings should not be returned")
	assert.ErrorContains(t, err, errForbidden)
}

package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedLinodeSettingsID       = 234
	managedLinodeSettingsPath     = "/managed/linode-settings/234"
	managedLinodeSettingsListPath = "/managed/linode-settings"
	managedLinodeSettingsLabel    = "linode123"
	managedLinodeSettingsGroup    = "linodes"
	managedLinodeSettingsIP       = "203.0.113.1"
	managedLinodeSettingsSSHUser  = "linode"
	managedLinodeSettingsSSHPort  = 22
)

func TestClientListManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := 2222
	user := accountMaintenanceEntityType
	settings := linode.PaginatedResponse[linode.ManagedLinodeSettings]{
		Data: []linode.ManagedLinodeSettings{{
			ID:    123,
			Label: managedLinodeSettingsLabel,
			Group: managedLinodeSettingsGroup,
			SSH: linode.ManagedLinodeSettingsSSH{
				Access: true,
				IP:     managedLinodeSettingsIP,
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
		assert.Equal(t, managedLinodeSettingsListPath, r.URL.Path, "request path should match managed Linode settings endpoint")
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
	assert.Equal(t, managedLinodeSettingsLabel, result.Data[0].Label)
	assert.Equal(t, managedLinodeSettingsIP, result.Data[0].SSH.IP)
	require.NotNil(t, result.Data[0].SSH.Port)
	assert.Equal(t, port, *result.Data[0].SSH.Port)
}

func TestClientListManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsListPath, r.URL.Path, "request path should match managed Linode settings endpoint")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedLinodeSettings]{
			Data:    []linode.ManagedLinodeSettings{{ID: 123, Label: managedLinodeSettingsLabel}},
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
		assert.Equal(t, managedLinodeSettingsListPath, r.URL.Path, "request path should match managed Linode settings endpoint")
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

func TestClientGetManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	sshUser := managedLinodeSettingsSSHUser
	sshPort := managedLinodeSettingsSSHPort
	settings := linode.ManagedLinodeSettings{
		ID:    managedLinodeSettingsID,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH: linode.ManagedLinodeSettingsSSH{
			Access: true,
			IP:     managedLinodeSettingsIP,
			Port:   &sshPort,
			User:   &sshUser,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "reading request body should not fail")
		assert.Empty(t, body, "request body should be empty")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)

	require.NoError(t, err, "GetManagedLinodeSettings should succeed")
	require.NotNil(t, result, "settings should be returned")
	assert.Equal(t, managedLinodeSettingsID, result.ID)
	assert.Equal(t, managedLinodeSettingsLabel, result.Label)
	assert.Equal(t, managedLinodeSettingsGroup, result.Group)
	assert.True(t, result.SSH.Access)
	assert.Equal(t, managedLinodeSettingsIP, result.SSH.IP)
	require.NotNil(t, result.SSH.Port)
	assert.Equal(t, managedLinodeSettingsSSHPort, *result.SSH.Port)
	require.NotNil(t, result.SSH.User)
	assert.Equal(t, managedLinodeSettingsSSHUser, *result.SSH.User)
}

func TestClientGetManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	settings := linode.ManagedLinodeSettings{ID: managedLinodeSettingsID, Label: managedLinodeSettingsLabel}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"my-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithJitter(false),
	)
	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)

	require.NoError(t, err, "GetManagedLinodeSettings should retry transient failures")
	require.NotNil(t, result, "settings should be returned after retry")
	assert.Equal(t, int32(2), calls.Load(), "one retry should be attempted")
	assert.Equal(t, managedLinodeSettingsID, result.ID)
}

func TestClientGetManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)

	require.Error(t, err, "API error should be returned")
	assert.Nil(t, result, "settings should be nil on API error")
	assert.ErrorContains(t, err, "not found")
}

func TestClientUpdateManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := managedLinodeSettingsSSHPort
	user := managedLinodeSettingsSSHUser
	settings := linode.ManagedLinodeSettings{
		ID:    managedLinodeSettingsID,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH: linode.ManagedLinodeSettingsSSH{
			Access: true,
			IP:     managedLinodeSettingsIP,
			Port:   &port,
			User:   &user,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.UpdateManagedLinodeSettingsRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if assert.NotNil(t, got.SSH, "ssh update should be sent") {
			if assert.NotNil(t, got.SSH.Access, "ssh access should be sent") {
				assert.True(t, *got.SSH.Access)
			}

			if assert.NotNil(t, got.SSH.IP, "ssh ip should be sent") {
				assert.Equal(t, managedLinodeSettingsIP, *got.SSH.IP)
			}

			if assert.NotNil(t, got.SSH.Port, "ssh port should be sent") {
				assert.Equal(t, port, *got.SSH.Port)
			}

			if assert.NotNil(t, got.SSH.User, "ssh user should be sent") {
				assert.Equal(t, user, *got.SSH.User)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{
		SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &settings.SSH.Access, IP: &settings.SSH.IP, Port: settings.SSH.Port, User: settings.SSH.User},
	})

	require.NoError(t, err, "UpdateManagedLinodeSettings should succeed on 200 response")
	require.NotNil(t, result)
	assert.Equal(t, managedLinodeSettingsID, result.ID)
	assert.Equal(t, managedLinodeSettingsLabel, result.Label)
	assert.Equal(t, managedLinodeSettingsIP, result.SSH.IP)
}

func TestClientUpdateManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	access := true
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &access}})

	require.Error(t, err, "UpdateManagedLinodeSettings should fail on API error")
	assert.Nil(t, result, "settings should not be returned")
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientUpdateManagedLinodeSettingsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	access := true
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	_, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &access}})

	require.Error(t, err, "mutating Managed Linode settings update should not retry transient failures")
	assert.Equal(t, int32(1), calls.Load(), "client should call update exactly once")
}

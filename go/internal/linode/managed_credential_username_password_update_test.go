package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedCredentialUsernamePasswordUpdatePath      = "/managed/credentials/9991/update"
	managedCredentialUsernamePasswordUpdateLabel     = "prod-password-1"
	managedCredentialUsernamePasswordUpdateTimestamp = "2018-01-01T00:01:01"
	managedCredentialUsernamePasswordUpdatePassword  = "stored-password-value"
	managedCredentialUsernamePasswordUpdateUsername  = "johndoe"
)

func TestClientUpdateManagedCredentialUsernamePasswordSuccess(t *testing.T) {
	t.Parallel()

	username := managedCredentialUsernamePasswordUpdateUsername
	request := &linode.UpdateManagedCredentialUsernamePasswordRequest{
		Password: managedCredentialUsernamePasswordUpdatePassword,
		Username: &username,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedCredentialUsernamePasswordUpdatePath, r.URL.Path, "request path should update managed credential username and password")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var got map[string]any
		assert.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, managedCredentialUsernamePasswordUpdatePassword, got["password"])
		assert.Equal(t, managedCredentialUsernamePasswordUpdateUsername, got["username"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialUsernamePasswordUpdateLabel,
			LastDecrypted: managedCredentialUsernamePasswordUpdateTimestamp,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, request)

	require.NoError(t, err, "UpdateManagedCredentialUsernamePassword should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, 9991, got.ID)
	assert.Equal(t, managedCredentialUsernamePasswordUpdateLabel, got.Label)
	assert.Equal(t, managedCredentialUsernamePasswordUpdateTimestamp, got.LastDecrypted)
}

func TestClientUpdateManagedCredentialUsernamePasswordNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword})

	require.Error(t, err, "UpdateManagedCredentialUsernamePassword should fail when the server is unreachable")

	var netErr *linode.NetworkError

	require.ErrorAs(t, err, &netErr, "error should be a NetworkError")
	assert.Equal(t, "UpdateManagedCredentialUsernamePassword", netErr.Operation)
}

func TestClientUpdateManagedCredentialUsernamePasswordAPIError(t *testing.T) {
	t.Parallel()

	request := &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedCredentialUsernamePasswordUpdatePath, r.URL.Path, "request path should update managed credential username and password")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot update managed credential username/password"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, request)

	require.Error(t, err)

	var apiErr *linode.APIError
	assert.ErrorAs(t, err, &apiErr)
}

func TestClientUpdateManagedCredentialUsernamePasswordDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedCredentialUsernamePasswordUpdatePath, r.URL.Path, "request path should update managed credential username and password")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword})

	require.Error(t, err, "UpdateManagedCredentialUsernamePassword should fail on 500 response")
	assert.Equal(t, int32(1), attempts.Load(), "UpdateManagedCredentialUsernamePassword must not retry and replay a mutating request")
}

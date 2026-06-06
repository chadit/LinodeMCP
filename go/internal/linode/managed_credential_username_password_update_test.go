package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedCredentialUsernamePasswordUpdatePath, r.URL.Path, "request path should update managed credential username and password")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err)

		var got map[string]any
		checkNoError(t, json.Unmarshal(body, &got))
		checkEqual(t, managedCredentialUsernamePasswordUpdatePassword, got["password"])
		checkEqual(t, managedCredentialUsernamePasswordUpdateUsername, got["username"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialUsernamePasswordUpdateLabel,
			LastDecrypted: managedCredentialUsernamePasswordUpdateTimestamp,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, request)

	requireNoError(t, err, "UpdateManagedCredentialUsernamePassword should succeed on 200 response")
	requireNotNil(t, got)
	checkEqual(t, 9991, got.ID)
	checkEqual(t, managedCredentialUsernamePasswordUpdateLabel, got.Label)
	checkEqual(t, managedCredentialUsernamePasswordUpdateTimestamp, got.LastDecrypted)
}

func TestClientUpdateManagedCredentialUsernamePasswordNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword})

	requireError(t, err, "UpdateManagedCredentialUsernamePassword should fail when the server is unreachable")
	netErr := requireNetworkError(t, err, "error should be a NetworkError")
	checkEqual(t, "UpdateManagedCredentialUsernamePassword", netErr.Operation)
}

func TestClientUpdateManagedCredentialUsernamePasswordAPIError(t *testing.T) {
	t.Parallel()

	request := &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedCredentialUsernamePasswordUpdatePath, r.URL.Path, "request path should update managed credential username and password")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot update managed credential username/password"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, request)

	requireError(t, err)
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateManagedCredentialUsernamePasswordDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedCredentialUsernamePasswordUpdatePath, r.URL.Path, "request path should update managed credential username and password")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateManagedCredentialUsernamePassword(t.Context(), 9991, &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: managedCredentialUsernamePasswordUpdatePassword})

	requireError(t, err, "UpdateManagedCredentialUsernamePassword should fail on 500 response")
	checkEqual(t, int32(1), attempts.Load(), "UpdateManagedCredentialUsernamePassword must not retry and replay a mutating request")
}

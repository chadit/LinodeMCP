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
	accountUserUpdateUsername       = "existing-user"
	accountUserUpdateEmail          = "updated-user@example.com"
	accountUserUpdateSSHKey         = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest"
	temporaryAccountUserUpdateError = "temporary account user update failure"
)

func TestClientUpdateAccountUserSuccess(t *testing.T) {
	t.Parallel()

	restricted := true
	newUsername := "renamed-user"
	email := accountUserUpdateEmail
	sshKeys := []string{accountUserUpdateSSHKey}
	request := &linode.UpdateAccountUserRequest{
		Email:      &email,
		Restricted: &restricted,
		SSHKeys:    &sshKeys,
		Username:   &newUsername,
	}
	updated := linode.AccountUser{Username: newUsername, Email: accountUserUpdateEmail, Restricted: restricted, SSHKeys: sshKeys, UserType: accountUserTypeDefault}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountUserUpdateUsername, r.URL.Path, "request path should include username")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, accountUserUpdateEmail, body["email"])
		assert.Equal(t, restricted, body["restricted"])
		assert.Equal(t, newUsername, body["username"])
		assert.Equal(t, []any{accountUserUpdateSSHKey}, body["ssh_keys"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updated))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, request)

	require.NoError(t, err, "UpdateAccountUser should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, updated.Username, got.Username)
	assert.Equal(t, updated.Email, got.Email)
	assert.True(t, got.Restricted)
}

func TestClientUpdateAccountUserSerializesEmptySSHKeys(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body, "ssh_keys")
		assert.Empty(t, body["ssh_keys"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUpdateUsername, Email: accountUserUpdateEmail}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	sshKeys := []string{}

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{SSHKeys: &sshKeys})

	require.NoError(t, err, "UpdateAccountUser should allow clearing SSH keys")
	require.NotNil(t, got)
}

func TestClientUpdateAccountUserEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/user%2Fname%3Fquery", r.URL.EscapedPath())
		assert.Empty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: "user/name?query", Email: accountUserUpdateEmail}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	email := accountUserUpdateEmail

	got, err := client.UpdateAccountUser(t.Context(), "user/name?query", &linode.UpdateAccountUserRequest{Email: &email})

	require.NoError(t, err, "UpdateAccountUser should URL-escape username path params")
	require.NotNil(t, got)
	assert.Equal(t, "user/name?query", got.Username)
}

func TestClientUpdateAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountUserUpdateUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	email := accountUserUpdateEmail

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{Email: &email})

	require.Error(t, err, "UpdateAccountUser should propagate API errors")
	assert.Nil(t, got)
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientUpdateAccountUserDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountUserUpdateUsername, r.URL.Path, "request path should include username")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserUpdateError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	email := accountUserUpdateEmail

	_, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{Email: &email})

	require.Error(t, err, "UpdateAccountUser should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating account user update must not be retried")
}

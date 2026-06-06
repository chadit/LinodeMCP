package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/"+accountUserUpdateUsername, r.URL.Path, "request path should include username")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "request should include bearer token")

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "decode request body")
		checkEqual(t, accountUserUpdateEmail, body["email"], "email should be serialized")
		checkEqual(t, restricted, body["restricted"], "restricted should be serialized")
		checkEqual(t, newUsername, body["username"], "username should be serialized")
		checkEqual(t, []any{accountUserUpdateSSHKey}, body["ssh_keys"], "ssh keys should be serialized")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(updated), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, request)

	requireNoError(t, err, "UpdateAccountUser should succeed on 200 response")
	requireNotNil(t, got, "result should not be nil")
	checkEqual(t, updated.Username, got.Username, "updated username should match")
	checkEqual(t, updated.Email, got.Email, "updated email should match")
	checkTrue(t, got.Restricted, "updated user should be restricted")
}

func TestClientUpdateAccountUserSerializesEmptySSHKeys(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "decode request body")
		accountCheckContains(t, body, "ssh_keys", "ssh_keys should be serialized")
		checkEmpty(t, body["ssh_keys"], "ssh keys should be empty")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUpdateUsername, Email: accountUserUpdateEmail}), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	sshKeys := []string{}

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{SSHKeys: &sshKeys})

	requireNoError(t, err, "UpdateAccountUser should allow clearing SSH keys")
	requireNotNil(t, got, "result should not be nil")
}

func TestClientUpdateAccountUserEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/user%2Fname%3Fquery", r.URL.EscapedPath(), "request path should URL-escape username")
		checkEmpty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: "user/name?query", Email: accountUserUpdateEmail}), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	email := accountUserUpdateEmail

	got, err := client.UpdateAccountUser(t.Context(), "user/name?query", &linode.UpdateAccountUserRequest{Email: &email})

	requireNoError(t, err, "UpdateAccountUser should URL-escape username path params")
	requireNotNil(t, got, "result should not be nil")
	checkEqual(t, "user/name?query", got.Username, "username should match")
}

func TestClientUpdateAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/"+accountUserUpdateUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	email := accountUserUpdateEmail

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{Email: &email})

	requireError(t, err, "UpdateAccountUser should propagate API errors")
	checkNil(t, got, "result should be nil")
	accountCheckForbiddenError(t, err)
}

func TestClientUpdateAccountUserDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/"+accountUserUpdateUsername, r.URL.Path, "request path should include username")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserUpdateError}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	email := accountUserUpdateEmail

	_, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{Email: &email})

	requireError(t, err, "UpdateAccountUser should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating account user update must not be retried")
}

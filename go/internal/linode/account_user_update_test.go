package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountUserUpdateUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountUserUpdateUsername)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["email"], accountUserUpdateEmail) {
			t.Errorf("got %v, want %v", body["email"], accountUserUpdateEmail)
		}

		if !reflect.DeepEqual(body["restricted"], restricted) {
			t.Errorf("got %v, want %v", body["restricted"], restricted)
		}

		if !reflect.DeepEqual(body["username"], newUsername) {
			t.Errorf("got %v, want %v", body["username"], newUsername)
		}

		if !reflect.DeepEqual(body["ssh_keys"], []any{accountUserUpdateSSHKey}) {
			t.Errorf("got %v, want %v", body["ssh_keys"], []any{accountUserUpdateSSHKey})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Username != updated.Username {
		t.Errorf("got.Username = %v, want %v", got.Username, updated.Username)
	}

	if got.Email != updated.Email {
		t.Errorf("got.Email = %v, want %v", got.Email, updated.Email)
	}

	if !got.Restricted {
		t.Error("got.Restricted = false, want true")
	}
}

func TestClientUpdateAccountUserSerializesEmptySSHKeys(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if _, ok := body["ssh_keys"]; !ok {
			t.Errorf("body missing key %v", "ssh_keys")
		}

		if sshKeys, ok := body["ssh_keys"].([]any); !ok || len(sshKeys) != 0 {
			t.Errorf("body[ssh_keys] = %v, want an empty array", body["ssh_keys"])
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUpdateUsername, Email: accountUserUpdateEmail}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	sshKeys := []string{}

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{SSHKeys: &sshKeys})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}
}

func TestClientUpdateAccountUserEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != tcAccountUsersUser2Fname3Fquery {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcAccountUsersUser2Fname3Fquery)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountUser{Username: "user/name?query", Email: accountUserUpdateEmail}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	email := accountUserUpdateEmail

	got, err := client.UpdateAccountUser(t.Context(), "user/name?query", &linode.UpdateAccountUserRequest{Email: &email})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Username != tcUserNameQuery {
		t.Errorf("got.Username = %v, want %v", got.Username, tcUserNameQuery)
	}
}

func TestClientUpdateAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountUserUpdateUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountUserUpdateUsername)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	email := accountUserUpdateEmail

	got, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{Email: &email})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUpdateAccountUserDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountUserUpdateUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountUserUpdateUsername)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserUpdateError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	email := accountUserUpdateEmail

	_, err := client.UpdateAccountUser(t.Context(), accountUserUpdateUsername, &linode.UpdateAccountUserRequest{Email: &email})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

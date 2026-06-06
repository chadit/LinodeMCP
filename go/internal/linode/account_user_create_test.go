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
	accountUserCreateUsername       = "new-user"
	accountUserCreateEmail          = "new-user@example.com"
	temporaryAccountUserCreateError = "temporary account user create failure"
)

func TestClientCreateAccountUserSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail}
	created := linode.AccountUser{Username: request.Username, Email: request.Email, UserType: accountUserTypeDefault}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/users", r.URL.Path, "request path should be /account/users")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "request should include bearer token")

		var got linode.CreateAccountUserRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "decode request body")
		checkEqual(t, request, &got, "request body should match")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(created), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountUser(t.Context(), request)

	requireNoError(t, err, "CreateAccountUser should succeed on 200 response")
	requireNotNil(t, got, "result should not be nil")
	checkEqual(t, created.Username, got.Username, "created username should match")
	checkEqual(t, created.Email, got.Email, "created email should match")
}

func TestClientCreateAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/users", r.URL.Path, "request path should be /account/users")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})

	requireError(t, err, "CreateAccountUser should propagate API errors")
	checkNil(t, got, "result should be nil")
	accountCheckForbiddenError(t, err)
}

func TestClientCreateAccountUserNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})

	requireError(t, err, "CreateAccountUser should fail when the server is unreachable")

	networkErr := requireNetworkError(t, err, "error should be a NetworkError")
	checkEqual(t, "CreateAccountUser", networkErr.Operation)
}

func TestClientCreateAccountUserDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/users", r.URL.Path, "request path should be /account/users")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserCreateError}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})

	requireError(t, err, "CreateAccountUser should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating account user creation must not be retried")
}

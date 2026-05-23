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
	accountUserCreateUsername       = "new-user"
	accountUserCreateEmail          = "new-user@example.com"
	temporaryAccountUserCreateError = "temporary account user create failure"
)

func TestClientCreateAccountUserSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail}
	created := linode.AccountUser{Username: request.Username, Email: request.Email, UserType: "default"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateAccountUserRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, request, &got)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountUser(t.Context(), request)

	require.NoError(t, err, "CreateAccountUser should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, created.Username, got.Username)
	assert.Equal(t, created.Email, got.Email)
}

func TestClientCreateAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})

	require.Error(t, err, "CreateAccountUser should propagate API errors")
	assert.Nil(t, got)
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientCreateAccountUserNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})

	require.Error(t, err, "CreateAccountUser should fail when the server is unreachable")

	var netErr *linode.NetworkError

	assert.ErrorAs(t, err, &netErr, "error should be a NetworkError")
}

func TestClientCreateAccountUserDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserCreateError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})

	require.Error(t, err, "CreateAccountUser should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating account user creation must not be retried")
}

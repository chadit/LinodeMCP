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
	temporaryAccountUserGrantsUpdateError = "temporary account user grants update failure"
	accountUserGrantReadOnlyError         = "read_only"
	accountUserGrantReadWriteError        = "read_write"
)

func TestClientUpdateAccountUserGrantsSuccess(t *testing.T) {
	t.Parallel()

	accountAccess := linode.GrantPermission(accountUserGrantReadOnlyError)
	linodePermission := linode.GrantPermission(accountUserGrantReadWriteError)
	lkeClusterPermission := linode.GrantPermission(accountUserGrantReadOnlyError)
	request := &linode.UpdateAccountUserGrantsRequest{
		Global:     &linode.UpdateAccountUserGlobalGrants{AccountAccess: &accountAccess},
		Linode:     &[]linode.UpdateAccountUserGrant{{ID: 123, Permissions: &linodePermission}},
		LKECluster: &[]linode.UpdateAccountUserGrant{{ID: 456, Permissions: &lkeClusterPermission}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		global, globalOK := body["global"].(map[string]any)
		if assert.True(t, globalOK, "global grants should be an object") {
			assert.Equal(t, map[string]any{"account_access": accountUserGrantReadOnlyError}, global)
		}

		assert.Equal(t, []any{map[string]any{"id": float64(123), "permissions": accountUserGrantReadWriteError}}, body["linode"])
		assert.Equal(t, []any{map[string]any{"id": float64(456), "permissions": accountUserGrantReadOnlyError}}, body["lkecluster"])
		assert.NotContains(t, body, "domain")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{
			Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission(accountUserGrantReadOnlyError)},
			Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission(accountUserGrantReadWriteError)}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)

	require.NoError(t, err, "UpdateAccountUserGrants should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, linode.GrantPermission(accountUserGrantReadOnlyError), result.Global.AccountAccess)
	require.Len(t, result.Linode, 1)
	assert.Equal(t, 123, result.Linode[0].ID)
}

func TestClientUpdateAccountUserGrantsEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/user%2Fname%3Fquery/grants", r.URL.EscapedPath())
		assert.Empty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), "user/name?query", request)

	require.NoError(t, err, "UpdateAccountUserGrants should URL-escape path parameters")
}

func TestClientUpdateAccountUserGrantsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)

	require.Error(t, err, "UpdateAccountUserGrants should fail on 403 response")
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientUpdateAccountUserGrantsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserGrantsUpdateError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)

	require.Error(t, err, "UpdateAccountUserGrants should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating grants update must not be retried")
}

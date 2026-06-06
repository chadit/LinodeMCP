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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "request should include bearer token")

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "decode request body")

		global, globalOK := body["global"].(map[string]any)
		if checkTrue(t, globalOK, "global grants should be an object") {
			checkEqual(t, map[string]any{"account_access": accountUserGrantReadOnlyError}, global, "global grants should match")
		}

		checkEqual(t, []any{map[string]any{"id": float64(123), "permissions": accountUserGrantReadWriteError}}, body["linode"], "linode grants should match")
		checkEqual(t, []any{map[string]any{"id": float64(456), "permissions": accountUserGrantReadOnlyError}}, body["lkecluster"], "lkecluster grants should match")
		accountCheckNotContains(t, body, "domain", "domain grants should be omitted")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Grants{
			Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission(accountUserGrantReadOnlyError)},
			Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission(accountUserGrantReadWriteError)}},
		}), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)

	requireNoError(t, err, "UpdateAccountUserGrants should succeed on 200 response")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, linode.GrantPermission(accountUserGrantReadOnlyError), result.Global.AccountAccess, "account access grant should match")
	requireLenOne(t, result.Linode)
	checkEqual(t, 123, result.Linode[0].ID, "linode grant ID should match")
}

func TestClientUpdateAccountUserGrantsEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/user%2Fname%3Fquery/grants", r.URL.EscapedPath(), "request path should URL-escape username grants")
		checkEmpty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Grants{}), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), "user/name?query", request)

	requireNoError(t, err, "UpdateAccountUserGrants should URL-escape path parameters")
}

func TestClientUpdateAccountUserGrantsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)

	requireError(t, err, "UpdateAccountUserGrants should fail on 403 response")
	accountCheckForbiddenError(t, err)
}

func TestClientUpdateAccountUserGrantsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserGrantsUpdateError}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)

	requireError(t, err, "UpdateAccountUserGrants should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating grants update must not be retried")
}

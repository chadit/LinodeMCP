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
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
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

		if !reflect.DeepEqual(body["global"], map[string]any{"account_access": accountUserGrantReadOnlyError}) {
			t.Errorf("body[global] = %v, want %v", body["global"], map[string]any{"account_access": accountUserGrantReadOnlyError})
		}

		if !reflect.DeepEqual(body["linode"], []any{map[string]any{keyID: float64(123), tcPermissions: accountUserGrantReadWriteError}}) {
			t.Errorf("got %v, want %v", body["linode"], []any{map[string]any{keyID: float64(123), tcPermissions: accountUserGrantReadWriteError}})
		}

		if !reflect.DeepEqual(body["lkecluster"], []any{map[string]any{keyID: float64(456), tcPermissions: accountUserGrantReadOnlyError}}) {
			t.Errorf("got %v, want %v", body["lkecluster"], []any{map[string]any{keyID: float64(456), tcPermissions: accountUserGrantReadOnlyError}})
		}

		if _, ok := body["domain"]; ok {
			t.Errorf("body has unexpected key %v", "domain")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Grants{
			Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission(accountUserGrantReadOnlyError)},
			Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission(accountUserGrantReadWriteError)}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Global.AccountAccess != linode.GrantPermission(accountUserGrantReadOnlyError) {
		t.Errorf("result.Global.AccountAccess = %v, want %v", result.Global.AccountAccess, linode.GrantPermission(accountUserGrantReadOnlyError))
	}

	if len(result.Linode) != 1 {
		t.Fatalf("len(result.Linode) = %d, want 1", len(result.Linode))
	}

	if result.Linode[0].ID != 123 {
		t.Errorf("result.Linode[0].ID = %v, want %v", result.Linode[0].ID, 123)
	}
}

func TestClientUpdateAccountUserGrantsEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/account/users/user%2Fname%3Fquery/grants" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/users/user%2Fname%3Fquery/grants")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Grants{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), "user/name?query", request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientUpdateAccountUserGrantsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUpdateAccountUserGrantsDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserGrantsUpdateError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	request := &linode.UpdateAccountUserGrantsRequest{Linode: &[]linode.UpdateAccountUserGrant{}}

	_, err := client.UpdateAccountUserGrants(t.Context(), accountLoginUsername, request)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

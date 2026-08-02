package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	accountUserGrantsProtoUsername = "grants-user"
	accountUserGrantsProtoPath     = tcAccountUsers + "/" + accountUserGrantsProtoUsername + "/grants"
	accountUserGrantsProtoLabel    = "grants-linode-1"
	accountUserGrantsProtoID       = 42

	// The read fixture leaves out every grant section except linode, matching
	// what the API sends for a user who only has access to one resource kind.
	accountUserGrantsProtoReadBody = `{"global":{"account_access":"read_only","add_linodes":true},` +
		`"linode":[{"id":42,"label":"grants-linode-1","permissions":"read_write"}]}`

	accountUserGrantsProtoWriteBody = `{"global":{"account_access":"read_write","add_linodes":false},` +
		`"linode":[{"id":42,"label":"grants-linode-1","permissions":"read_only"}]}`

	accountUserGrantsProtoReadOnly  = "read_only"
	accountUserGrantsProtoReadWrite = "read_write"
)

// TestClientGetAccountUserGrantsProtoSuccess verifies GetAccountUserGrantsProto
// sends a GET to /account/users/{username}/grants and decodes the response into
// the proto AccountUserGrants element.
func TestClientGetAccountUserGrantsProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountUserGrantsProtoPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountUserGrantsProtoPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(accountUserGrantsProtoReadBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountUserGrantsProto(t.Context(), accountUserGrantsProtoUsername)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetGlobal().GetAccountAccess() != accountUserGrantsProtoReadOnly {
		t.Errorf("got.GetGlobal().GetAccountAccess() = %v, want %v",
			got.GetGlobal().GetAccountAccess(), accountUserGrantsProtoReadOnly)
	}

	if !got.GetGlobal().GetAddLinodes() {
		t.Error("got.GetGlobal().GetAddLinodes() = false, want true")
	}

	if len(got.GetLinode()) != 1 {
		t.Fatalf("len(got.GetLinode()) = %d, want %d", len(got.GetLinode()), 1)
	}

	if got.GetLinode()[0].GetId() != accountUserGrantsProtoID {
		t.Errorf("got.GetLinode()[0].GetId() = %v, want %v", got.GetLinode()[0].GetId(), accountUserGrantsProtoID)
	}

	if got.GetLinode()[0].GetLabel() != accountUserGrantsProtoLabel {
		t.Errorf("got.GetLinode()[0].GetLabel() = %v, want %v",
			got.GetLinode()[0].GetLabel(), accountUserGrantsProtoLabel)
	}

	if got.GetLinode()[0].GetPermissions() != accountUserGrantsProtoReadWrite {
		t.Errorf("got.GetLinode()[0].GetPermissions() = %v, want %v",
			got.GetLinode()[0].GetPermissions(), accountUserGrantsProtoReadWrite)
	}

	// The API omits sections the user holds no grants in, and the decoded
	// element has to report those as empty rather than carrying stale entries.
	if len(got.GetDomain()) != 0 {
		t.Errorf("len(got.GetDomain()) = %d, want %d", len(got.GetDomain()), 0)
	}
}

// TestClientGetAccountUserGrantsProtoEscapesUsername verifies the proto read
// path encodes path separators in the username.
func TestClientGetAccountUserGrantsProtoEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != tcAccountUsersUser2Fname3Fquery+"/grants" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v",
				r.URL.EscapedPath(), tcAccountUsersUser2Fname3Fquery+"/grants")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(accountUserGrantsProtoReadBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	if _, err := client.GetAccountUserGrantsProto(t.Context(), tcUserNameQuery); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountUserGrantsProtoRetriesTransientError verifies the
// read-only proto grants lookup retries transient failures.
func TestClientGetAccountUserGrantsProtoRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryPaymentError}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(accountUserGrantsProtoReadBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountUserGrantsProto(t.Context(), accountUserGrantsProtoUsername)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetGlobal().GetAccountAccess() != accountUserGrantsProtoReadOnly {
		t.Errorf("got.GetGlobal().GetAccountAccess() = %v, want %v",
			got.GetGlobal().GetAccountAccess(), accountUserGrantsProtoReadOnly)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientUpdateAccountUserGrantsProtoSuccess verifies
// UpdateAccountUserGrantsProto sends a PUT to /account/users/{username}/grants
// carrying the requested sections and decodes the response into the proto
// AccountUserGrants element.
func TestClientUpdateAccountUserGrantsProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != accountUserGrantsProtoPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountUserGrantsProtoPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		assertAccountUserGrantsProtoRequest(t, body)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(accountUserGrantsProtoWriteBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateAccountUserGrantsProto(t.Context(), accountUserGrantsProtoUsername,
		newAccountUserGrantsProtoRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetGlobal().GetAccountAccess() != accountUserGrantsProtoReadWrite {
		t.Errorf("got.GetGlobal().GetAccountAccess() = %v, want %v",
			got.GetGlobal().GetAccountAccess(), accountUserGrantsProtoReadWrite)
	}

	if got.GetGlobal().GetAddLinodes() {
		t.Error("got.GetGlobal().GetAddLinodes() = true, want false")
	}

	if len(got.GetLinode()) != 1 {
		t.Fatalf("len(got.GetLinode()) = %d, want %d", len(got.GetLinode()), 1)
	}

	if got.GetLinode()[0].GetPermissions() != accountUserGrantsProtoReadOnly {
		t.Errorf("got.GetLinode()[0].GetPermissions() = %v, want %v",
			got.GetLinode()[0].GetPermissions(), accountUserGrantsProtoReadOnly)
	}
}

// TestClientUpdateAccountUserGrantsProtoDoesNotRetryTransientError verifies the
// mutating proto grants update is sent exactly once so a transient failure
// cannot replay a permission change.
func TestClientUpdateAccountUserGrantsProtoDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: temporaryPaymentError}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	got, err := client.UpdateAccountUserGrantsProto(t.Context(), accountUserGrantsProtoUsername,
		newAccountUserGrantsProtoRequest())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusInternalServerError)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// newAccountUserGrantsProtoRequest builds the grants update both proto write
// tests send, so the request assertion and the call site cannot drift apart.
func newAccountUserGrantsProtoRequest() *linode.UpdateAccountUserGrantsRequest {
	access := linode.GrantPermission(accountUserGrantsProtoReadWrite)
	permissions := linode.GrantPermission(accountUserGrantsProtoReadOnly)

	var addLinodes bool

	sections := []linode.UpdateAccountUserGrant{{ID: accountUserGrantsProtoID, Permissions: &permissions}}

	return &linode.UpdateAccountUserGrantsRequest{
		Global: &linode.UpdateAccountUserGlobalGrants{AccountAccess: &access, AddLinodes: &addLinodes},
		Linode: &sections,
	}
}

// assertAccountUserGrantsProtoRequest checks the body the server received
// carries the global and per-resource sections the caller asked for.
func assertAccountUserGrantsProtoRequest(t *testing.T, body []byte) {
	t.Helper()

	var sent struct {
		Global map[string]any   `json:"global"`
		Linode []map[string]any `json:"linode"`
	}

	if err := json.Unmarshal(body, &sent); err != nil {
		t.Errorf("unexpected error: %v", err)

		return
	}

	if sent.Global["account_access"] != accountUserGrantsProtoReadWrite {
		t.Errorf("sent.Global[account_access] = %v, want %v",
			sent.Global["account_access"], accountUserGrantsProtoReadWrite)
	}

	if sent.Global["add_linodes"] != false {
		t.Errorf("sent.Global[add_linodes] = %v, want %v", sent.Global["add_linodes"], false)
	}

	if len(sent.Linode) != 1 {
		t.Fatalf("len(sent.Linode) = %d, want %d", len(sent.Linode), 1)
	}

	if sent.Linode[0][keyID] != float64(accountUserGrantsProtoID) {
		t.Errorf("sent.Linode[0][%v] = %v, want %v", keyID, sent.Linode[0][keyID], accountUserGrantsProtoID)
	}

	if sent.Linode[0]["permissions"] != accountUserGrantsProtoReadOnly {
		t.Errorf("sent.Linode[0][permissions] = %v, want %v",
			sent.Linode[0]["permissions"], accountUserGrantsProtoReadOnly)
	}
}

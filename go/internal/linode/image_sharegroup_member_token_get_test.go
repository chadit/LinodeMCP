package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetImageShareGroupMemberTokenSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-05T10:09:09"
	member := linode.ImageShareGroupMember{
		TokenUUID: shareGroupTokenUUIDFixture,
		Status:    oauthClientStatus,
		Label:     "Engineering - Backend",
		Created:   imageShareGroupTokenCreated,
		Updated:   &updated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/123/members/"+shareGroupTokenUUIDFixture, r.URL.Path, "request path should include share group ID and token UUID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(member))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupMemberToken(t.Context(), 123, shareGroupTokenUUIDFixture)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, shareGroupTokenUUIDFixture, result.TokenUUID)
	checkEqual(t, "Engineering - Backend", result.Label)
}

func TestClientGetImageShareGroupMemberTokenEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/123/members/token%2F..%3Fquery%23frag", r.URL.EscapedPath(), "token UUID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: "token/..?query#frag"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupMemberToken(t.Context(), 123, "token/..?query#frag")

	requireNoError(t, err)
	requireNotNil(t, result)
}

func TestClientGetImageShareGroupMemberTokenRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/123/members/"+shareGroupTokenUUIDFixture, r.URL.Path, "request path should include share group ID and token UUID")

		if requestCount.Add(1) == 1 {
			http.Error(w, errTemporaryFailure, http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: shareGroupTokenUUIDFixture}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetImageShareGroupMemberToken(t.Context(), 123, shareGroupTokenUUIDFixture)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, int32(2), requestCount.Load(), "read-only GET route should retry once")
}

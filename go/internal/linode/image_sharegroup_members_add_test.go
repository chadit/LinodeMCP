package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAddImageShareGroupMembersSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/images/sharegroups/123/members", r.URL.Path, "request path should include share group ID and members suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]string
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		checkEqual(t, memberLabelFixture, body["label"], "label should be sent")
		checkEqual(t, memberTokenFixture, body["token"], "token should be sent")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{ID: 123, Label: imageShareGroupLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	requireNoError(t, err)
	requireNotNil(t, shareGroup)
	checkEqual(t, 123, shareGroup.ID)
	checkEqual(t, int32(1), requestCount.Load(), "request should be sent once")
}

func TestClientAddImageShareGroupMembersEscapesShareGroupID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/123/members", r.URL.EscapedPath(), "share group ID should be one encoded path segment")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{ID: 123, Label: imageShareGroupLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	requireNoError(t, err)
}

func TestClientAddImageShareGroupMembersError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	requireError(t, err)
	checkNil(t, shareGroup)
}

func TestClientAddImageShareGroupMembersDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	requireError(t, err)
	checkNil(t, shareGroup)
	checkEqual(t, int32(1), requestCount.Load(), "mutating member add should not be retried")
}

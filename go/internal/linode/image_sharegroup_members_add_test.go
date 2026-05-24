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

func TestClientAddImageShareGroupMembersSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/images/sharegroups/123/members", r.URL.Path, "request path should include share group ID and members suffix")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]string
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, memberLabelFixture, body["label"], "label should be sent")
		assert.Equal(t, memberTokenFixture, body["token"], "token should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{ID: 123, Label: imageShareGroupLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	require.NoError(t, err)
	require.NotNil(t, shareGroup)
	assert.Equal(t, 123, shareGroup.ID)
	assert.Equal(t, int32(1), requestCount.Load(), "request should be sent once")
}

func TestClientAddImageShareGroupMembersEscapesShareGroupID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/sharegroups/123/members", r.URL.EscapedPath(), "share group ID should be one encoded path segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{ID: 123, Label: imageShareGroupLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	require.NoError(t, err)
}

func TestClientAddImageShareGroupMembersError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	require.Error(t, err)
	assert.Nil(t, shareGroup)
}

func TestClientAddImageShareGroupMembersDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})

	require.Error(t, err)
	assert.Nil(t, shareGroup)
	assert.Equal(t, int32(1), requestCount.Load(), "mutating member add should not be retried")
}

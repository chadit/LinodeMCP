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

const imageShareGroupMemberUpdateLabel = "Engineering - Backend"

func TestClientUpdateImageShareGroupMemberSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/123/members/"+shareGroupUUIDExample, r.URL.Path, "request path should include share group ID and token UUID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, imageShareGroupMemberUpdateLabel, body[keyLabel], "label should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{
			TokenUUID: shareGroupUUIDExample,
			Status:    oauthClientStatus,
			Label:     imageShareGroupMemberUpdateLabel,
			Created:   imageShareGroupTokenCreated,
		}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	member, err := client.UpdateImageShareGroupMember(t.Context(), 123, shareGroupUUIDExample, &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})

	require.NoError(t, err, "UpdateImageShareGroupMember should succeed")
	require.NotNil(t, member, "member should be returned")
	assert.Equal(t, shareGroupUUIDExample, member.TokenUUID, "response should decode token UUID")
	assert.Equal(t, imageShareGroupMemberUpdateLabel, member.Label, "response should decode label")
}

func TestClientUpdateImageShareGroupMemberEscapesTokenUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/123/members/token%2Fuuid%3Fquery", r.URL.EscapedPath(), "token UUID should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: "token/uuid?query", Label: imageShareGroupMemberUpdateLabel}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroupMember(t.Context(), 123, "token/uuid?query", &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})

	require.NoError(t, err, "UpdateImageShareGroupMember should escape token path parameters")
}

func TestClientUpdateImageShareGroupMemberEscapesDotSegments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		tokenUUID   string
		escapedPath string
	}{
		{name: "single dot", tokenUUID: ".", escapedPath: "/images/sharegroups/123/members/%2E"},
		{name: "double dot", tokenUUID: pathTraversalDotDot, escapedPath: "/images/sharegroups/123/members/%2E%2E"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
				assert.Equal(t, testCase.escapedPath, r.URL.EscapedPath(), "dot segment should stay encoded")
				w.Header().Set("Content-Type", "application/json")
				assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: testCase.tokenUUID, Label: imageShareGroupMemberUpdateLabel}), "encoding response should succeed")
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
			_, err := client.UpdateImageShareGroupMember(t.Context(), 123, testCase.tokenUUID, &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})

			require.NoError(t, err, "UpdateImageShareGroupMember should encode dot segments")
		})
	}
}

func TestUpdateImageShareGroupMemberNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"try again"}]}`))
		assert.NoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateImageShareGroupMember(t.Context(), 123, shareGroupUUIDExample, &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})

	require.Error(t, err, "transient failure should return an error")
	assert.Equal(t, int32(1), calls.Load(), "mutating member update should not be retried")
}

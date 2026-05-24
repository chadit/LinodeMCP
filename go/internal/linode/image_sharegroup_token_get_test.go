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
	imageShareGroupTokenUUID = "00000000-0000-4000-8000-000000000001"
	imageShareGroupUUID      = "e1d0e58b-f89f-4237-84ab-b82077342359"
	imageShareGroupLabel     = "DevOps Base Images"
)

func TestClientGetImageShareGroupTokenSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-04T11:09:09"
	expiry := "2025-09-04T10:09:09"
	token := linode.ImageShareGroupToken{
		TokenUUID:              imageShareGroupTokenUUID,
		Status:                 oauthClientStatus,
		Label:                  "Backend Services - Engineering",
		Created:                "2025-08-04T10:09:09",
		Updated:                &updated,
		Expiry:                 &expiry,
		ValidForShareGroupUUID: imageShareGroupUUID,
		ShareGroupUUID:         imageShareGroupUUID,
		ShareGroupLabel:        imageShareGroupLabel,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/tokens/"+imageShareGroupTokenUUID, r.URL.Path, "request path should include token UUID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(token))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupToken(t.Context(), imageShareGroupTokenUUID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Backend Services - Engineering", result.Label)
	assert.Equal(t, imageShareGroupTokenUUID, result.TokenUUID)
}

func TestClientGetImageShareGroupTokenEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/sharegroups/tokens/token%2F..%3Fquery%23frag", r.URL.EscapedPath(), "token UUID should be one encoded path segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{TokenUUID: "token/..?query#frag"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupToken(t.Context(), "token/..?query#frag")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "token/..?query#frag", result.TokenUUID)
}

func TestClientGetImageShareGroupTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/sharegroups/tokens/%2E%2E", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{TokenUUID: ".."}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupToken(t.Context(), "..")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "..", result.TokenUUID)
}

func TestClientGetImageShareGroupTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupToken(t.Context(), imageShareGroupTokenUUID)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientGetImageShareGroupTokenRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{TokenUUID: imageShareGroupTokenUUID}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetImageShareGroupToken(t.Context(), imageShareGroupTokenUUID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

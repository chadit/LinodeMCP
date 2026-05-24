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

func TestClientListImagesByShareGroupTokenSuccess(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: "private/123", Label: "shared-ubuntu", Type: "manual", Status: "available", Created: "2025-01-01T00:00:00", Size: 2500},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup/images", r.URL.Path, "request path should include token UUID and sharegroup images suffix")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 2, 25)

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "private/123", result.Data[0].ID)
	assert.Equal(t, "shared-ubuntu", result.Data[0].Label)
}

func TestClientListImagesByShareGroupTokenEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup/images", r.URL.EscapedPath(), "token UUID should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), "token/..?query#frag", 0, 0)

	require.NoError(t, err)
	assert.Empty(t, result.Data)
}

func TestClientListImagesByShareGroupTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/sharegroups/tokens/%2E%2E/sharegroup/images", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), "..", 0, 0)

	require.NoError(t, err)
	assert.Empty(t, result.Data)
}

func TestClientListImagesByShareGroupTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 0, 0)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientListImagesByShareGroupTokenRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{{ID: "private/123"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 0, 0)

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}

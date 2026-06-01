package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, int64(0), r.ContentLength, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token", profileTokenScopesKey: "*"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfileToken(t.Context(), 12345)

	require.NoError(t, err, "GetProfileToken should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	tokenID, ok := (*result)[keyID].(float64)
	require.True(t, ok, "token ID should decode as a number")
	assert.InEpsilon(t, float64(12345), tokenID, 0.0)
	assert.Equal(t, "api-token", (*result)[keyLabel])
	assert.Equal(t, "*", (*result)[profileTokenScopesKey])
}

func TestClientGetProfileTokenEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "numeric token ID should remain a single path segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileToken(t.Context(), 12345)

	require.NoError(t, err)
}

func TestClientGetProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileToken(t.Context(), 12345)

	require.Error(t, err, "GetProfileToken should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "GetProfileToken should return APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

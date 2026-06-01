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
	profileTokenLabel         = "ci-token"
	profileTokenUpdatedScopes = "linodes:read_write"
)

func TestClientListProfileTokensSuccess(t *testing.T) {
	t.Parallel()

	tokens := linode.PaginatedResponse[linode.ProfileToken]{
		Data: []linode.ProfileToken{{
			keyID:    float64(67890),
			keyLabel: profileTokenLabel,
			"scopes": "linodes:read_only",
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, int64(0), r.ContentLength, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(tokens))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileTokens(t.Context(), 0, 0)

	require.NoError(t, err, "ListProfileTokens should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 1, result.Pages)
	assert.Equal(t, 1, result.Results)
	require.Len(t, result.Data, 1)
	assert.InEpsilon(t, float64(67890), result.Data[0][keyID], 0.001)
	assert.Equal(t, profileTokenLabel, result.Data[0][keyLabel])
}

func TestClientListProfileTokensPagination(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
			Page: 2,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileTokens(t.Context(), 2, 25)

	require.NoError(t, err, "ListProfileTokens should accept pagination")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
}

func TestClientListProfileTokensAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileTokens(t.Context(), 0, 0)

	require.Error(t, err, "ListProfileTokens should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "ListProfileTokens should return APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListProfileTokensRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileTokens(t.Context(), 0, 0)

	require.NoError(t, err, "ListProfileTokens should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientUpdateProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON") {
			return
		}

		assert.Equal(t, profileTokenLabel, body[keyLabel])
		assert.Equal(t, profileTokenUpdatedScopes, body["scopes"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: profileTokenLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel, "scopes": profileTokenUpdatedScopes})

	require.NoError(t, err, "UpdateProfileToken should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.InEpsilon(t, float64(12345), (*result)[keyID], 0.001)
	assert.Equal(t, profileTokenLabel, (*result)[keyLabel])
}

func TestClientUpdateProfileTokenEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/profile/tokens/12%2F..%2F34%3Fx=1", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "escaped query marker must stay in path")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345)}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfileToken(t.Context(), "12/../34?x=1", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})

	require.NoError(t, err, "UpdateProfileToken should escape path parameters")
}

func TestClientUpdateProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})

	require.Error(t, err, "non-transient API failure should return an error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientUpdateProfileTokenNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})

	require.Error(t, err, "transient failure should return an error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating token update must not retry")
}

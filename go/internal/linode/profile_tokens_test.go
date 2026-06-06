package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")
		checkEqual(t, int64(0), r.ContentLength, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(tokens), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileTokens(t.Context(), 0, 0)

	requireNoError(t, err, "ListProfileTokens should succeed on 200 response")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, 1, result.Page, "values differ")
	checkEqual(t, 1, result.Pages, "values differ")
	checkEqual(t, 1, result.Results, "values differ")
	requireLenOne(t, result.Data)
	checkEqual(t, float64(67890), result.Data[0][keyID], "values differ")
	checkEqual(t, profileTokenLabel, result.Data[0][keyLabel], "values differ")
}

func TestClientListProfileTokensPagination(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
			Page: 2,
		}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileTokens(t.Context(), 2, 25)

	requireNoError(t, err, "ListProfileTokens should accept pagination")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page, "values differ")
}

func TestClientListProfileTokensAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileTokens(t.Context(), 0, 0)

	requireError(t, err, "ListProfileTokens should fail on 403 response")

	apiErr := requireAPIError(t, err, "ListProfileTokens should return APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode, "values differ")
}

func TestClientListProfileTokensRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}), "expected no error")

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
		}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileTokens(t.Context(), 0, 0)

	requireNoError(t, err, "ListProfileTokens should succeed after retry")
	requireNotNil(t, result, "result should not be nil")
	requireLenOne(t, result.Data)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientUpdateProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")

		var body map[string]any
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON") {
			return
		}

		checkEqual(t, profileTokenLabel, body[keyLabel], "values differ")
		checkEqual(t, profileTokenUpdatedScopes, body["scopes"], "values differ")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: profileTokenLabel}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel, "scopes": profileTokenUpdatedScopes})

	requireNoError(t, err, "UpdateProfileToken should succeed on 200 response")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, float64(12345), (*result)[keyID], "values differ")
	checkEqual(t, profileTokenLabel, (*result)[keyLabel], "values differ")
}

func TestClientUpdateProfileTokenEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/profile/tokens/12%2F..%2F34%3Fx=1", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "escaped query marker must stay in path")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345)}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfileToken(t.Context(), "12/../34?x=1", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})

	requireNoError(t, err, "UpdateProfileToken should escape path parameters")
}

func TestClientUpdateProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})

	requireError(t, err, "non-transient API failure should return an error")

	apiErr := requireAPIError(t, err, "expected API error")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode, "values differ")
	checkEqual(t, errForbidden, apiErr.Message, "values differ")
}

func TestClientUpdateProfileTokenNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdateProfileToken(t.Context(), "12345", linode.UpdateProfileTokenRequest{keyLabel: profileTokenLabel})

	requireError(t, err, "transient failure should return an error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating token update must not retry")
}

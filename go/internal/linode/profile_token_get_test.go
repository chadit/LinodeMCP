package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")
		checkEqual(t, int64(0), r.ContentLength, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token", profileTokenScopesKey: "*"}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfileToken(t.Context(), 12345)

	requireNoError(t, err, "GetProfileToken should succeed on 200 response")
	requireNotNil(t, result, "result should not be nil")

	tokenID, ok := (*result)[keyID].(float64)
	if !ok {
		t.Fatalf("token ID should decode as a number")
	}

	checkEqual(t, float64(12345), tokenID, "values differ")
	checkEqual(t, "api-token", (*result)[keyLabel], "values differ")
	checkEqual(t, "*", (*result)[profileTokenScopesKey], "values differ")
}

func TestClientGetProfileTokenEscapesTokenID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "numeric token ID should remain a single path segment")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token"}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileToken(t.Context(), 12345)

	requireNoError(t, err, "expected no error")
}

func TestClientGetProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileToken(t.Context(), 12345)

	requireError(t, err, "GetProfileToken should fail on 403 response")

	apiErr := requireAPIError(t, err, "GetProfileToken should return APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode, "values differ")
}

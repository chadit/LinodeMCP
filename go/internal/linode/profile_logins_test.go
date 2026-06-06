package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListProfileLoginsSuccess(t *testing.T) {
	t.Parallel()

	logins := linode.PaginatedResponse[linode.AccountLogin]{
		Data: []linode.AccountLogin{{
			Datetime:   accountUserPasswordCreated,
			ID:         321,
			IP:         "203.0.113.20",
			Restricted: false,
			Status:     statusSuccessful,
			Username:   accountLoginUsername,
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")
		checkEqual(t, int64(0), r.ContentLength, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(logins), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileLogins(t.Context(), 2, 25)

	requireNoError(t, err, "ListProfileLogins should succeed on 200 response")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page, "values differ")
	checkEqual(t, 3, result.Pages, "values differ")
	checkEqual(t, 7, result.Results, "values differ")
	requireLenOne(t, result.Data)
	checkEqual(t, 321, result.Data[0].ID, "values differ")
	checkEqual(t, accountLoginUsername, result.Data[0].Username, "values differ")
	checkEqual(t, "203.0.113.20", result.Data[0].IP, "values differ")
}

func TestClientListProfileLoginsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileLogins(t.Context(), 0, 0)

	requireError(t, err, "ListProfileLogins should fail on 403 response")

	apiErr := requireAPIError(t, err, "ListProfileLogins should return APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode, "values differ")
}

func TestClientListProfileLoginsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary profile login error"}}}), "expected no error")

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
			Data: []linode.AccountLogin{{ID: 321, Username: accountLoginUsername}},
		}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileLogins(t.Context(), 0, 0)

	requireNoError(t, err, "ListProfileLogins should succeed after retry")
	requireNotNil(t, result, "result should not be nil")
	requireLenOne(t, result.Data)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

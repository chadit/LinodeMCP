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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, int64(0), r.ContentLength, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(logins))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileLogins(t.Context(), 2, 25)

	require.NoError(t, err, "ListProfileLogins should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 3, result.Pages)
	assert.Equal(t, 7, result.Results)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 321, result.Data[0].ID)
	assert.Equal(t, accountLoginUsername, result.Data[0].Username)
	assert.Equal(t, "203.0.113.20", result.Data[0].IP)
}

func TestClientListProfileLoginsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileLogins(t.Context(), 0, 0)

	require.Error(t, err, "ListProfileLogins should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "ListProfileLogins should return APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListProfileLoginsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary profile login error"}}}))

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
			Data: []linode.AccountLogin{{ID: 321, Username: accountLoginUsername}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileLogins(t.Context(), 0, 0)

	require.NoError(t, err, "ListProfileLogins should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

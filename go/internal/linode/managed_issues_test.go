package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedIssuesPath      = "/managed/issues"
	managedIssueCreated    = "2018-01-01T00:01:01"
	managedIssuePath       = "/managed/issues/823"
	managedIssueLabel      = "Managed Issue opened!"
	managedIssueEntityType = "ticket"
	managedIssueEntityURL  = "/support/tickets/98765"
	managedIssuesForbidden = "Forbidden"
	managedIssueAuthHeader = "Bearer token"
)

func TestClientGetManagedIssueSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedIssuePath, r.URL.Path, "request path should include issue ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedIssue{
			ID:       823,
			Created:  managedIssueCreated,
			Services: []int{654},
			Entity: linode.ManagedIssueEntity{
				ID:    98765,
				Label: managedIssueLabel,
				Type:  managedIssueEntityType,
				URL:   managedIssueEntityURL,
			},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedIssue(t.Context(), 823)

	require.NoError(t, err, "GetManagedIssue should succeed on 200 response")
	require.NotNil(t, result)
	assert.Equal(t, 823, result.ID)
	assert.Equal(t, managedIssueCreated, result.Created)
	assert.Equal(t, []int{654}, result.Services)
	assert.Equal(t, 98765, result.Entity.ID)
	assert.Equal(t, managedIssueLabel, result.Entity.Label)
	assert.Equal(t, managedIssueEntityType, result.Entity.Type)
	assert.Equal(t, managedIssueEntityURL, result.Entity.URL)
}

func TestClientGetManagedIssueRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, managedIssuePath, r.URL.Path, "request path should include issue ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedIssue{ID: 823}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.GetManagedIssue(t.Context(), 823)

	require.NoError(t, err, "read-only Managed issue get should retry transient failures")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientGetManagedIssueAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedIssuePath, r.URL.Path, "request path should include issue ID")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedIssuesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetManagedIssue(t.Context(), 823)

	require.Error(t, err, "GetManagedIssue should fail on API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListManagedIssuesSuccess(t *testing.T) {
	t.Parallel()

	issues := linode.PaginatedResponse[linode.ManagedIssue]{
		Data: []linode.ManagedIssue{{
			ID:       823,
			Created:  managedIssueCreated,
			Services: []int{654},
			Entity: linode.ManagedIssueEntity{
				ID:    98765,
				Label: managedIssueLabel,
				Type:  managedIssueEntityType,
				URL:   managedIssueEntityURL,
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 51,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedIssuesPath, r.URL.Path, "request path should be /managed/issues")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(issues))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListManagedIssues(t.Context(), 2, 25)

	require.NoError(t, err, "ListManagedIssues should succeed on 200 response")
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 823, result.Data[0].ID)
	assert.Equal(t, managedIssueCreated, result.Data[0].Created)
	assert.Equal(t, []int{654}, result.Data[0].Services)
	assert.Equal(t, 98765, result.Data[0].Entity.ID)
	assert.Equal(t, managedIssueLabel, result.Data[0].Entity.Label)
	assert.Equal(t, managedIssueEntityType, result.Data[0].Entity.Type)
	assert.Equal(t, managedIssueEntityURL, result.Data[0].Entity.URL)
}

func TestClientListManagedIssuesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, managedIssuesPath, r.URL.Path, "request path should be /managed/issues")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedIssue]{
			Data:    []linode.ManagedIssue{{ID: 823}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListManagedIssues(t.Context(), 0, 0)

	require.NoError(t, err, "read-only Managed issues list should retry transient failures")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListManagedIssuesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedIssuesPath, r.URL.Path, "request path should be /managed/issues")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedIssuesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedIssues(t.Context(), 0, 0)

	require.Error(t, err, "ListManagedIssues should fail on API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

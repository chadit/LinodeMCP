package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedIssuePath, r.URL.Path, "request path should include issue ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedIssue{
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

	requireNoError(t, err, "GetManagedIssue should succeed on 200 response")
	requireNotNil(t, result)
	checkEqual(t, 823, result.ID)
	checkEqual(t, managedIssueCreated, result.Created)
	checkEqual(t, []int{654}, result.Services)
	checkEqual(t, 98765, result.Entity.ID)
	checkEqual(t, managedIssueLabel, result.Entity.Label)
	checkEqual(t, managedIssueEntityType, result.Entity.Type)
	checkEqual(t, managedIssueEntityURL, result.Entity.URL)
}

func TestClientGetManagedIssueRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, managedIssuePath, r.URL.Path, "request path should include issue ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedIssue{ID: 823}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.GetManagedIssue(t.Context(), 823)

	requireNoError(t, err, "read-only Managed issue get should retry transient failures")
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientGetManagedIssueAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedIssuePath, r.URL.Path, "request path should include issue ID")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedIssuesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetManagedIssue(t.Context(), 823)

	requireError(t, err, "GetManagedIssue should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedIssuesPath, r.URL.Path, "request path should be /managed/issues")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(issues))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListManagedIssues(t.Context(), 2, 25)

	requireNoError(t, err, "ListManagedIssues should succeed on 200 response")
	requireNotNil(t, result)
	requireLenOne(t, result.Data)
	checkEqual(t, 823, result.Data[0].ID)
	checkEqual(t, managedIssueCreated, result.Data[0].Created)
	checkEqual(t, []int{654}, result.Data[0].Services)
	checkEqual(t, 98765, result.Data[0].Entity.ID)
	checkEqual(t, managedIssueLabel, result.Data[0].Entity.Label)
	checkEqual(t, managedIssueEntityType, result.Data[0].Entity.Type)
	checkEqual(t, managedIssueEntityURL, result.Data[0].Entity.URL)
}

func TestClientListManagedIssuesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, managedIssuesPath, r.URL.Path, "request path should be /managed/issues")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedIssue]{
			Data:    []linode.ManagedIssue{{ID: 823}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListManagedIssues(t.Context(), 0, 0)

	requireNoError(t, err, "read-only Managed issues list should retry transient failures")
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListManagedIssuesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedIssuesPath, r.URL.Path, "request path should be /managed/issues")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedIssuesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedIssues(t.Context(), 0, 0)

	requireError(t, err, "ListManagedIssues should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

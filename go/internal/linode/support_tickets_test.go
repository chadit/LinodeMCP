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

// TestClientListSupportTicketsSuccess verifies ListSupportTickets sends a GET
// request to /support/tickets with pagination query parameters.
func TestClientListSupportTicketsSuccess(t *testing.T) {
	t.Parallel()

	tickets := linode.PaginatedResponse[linode.SupportTicket]{
		Data: []linode.SupportTicket{{
			ID:          11111,
			Summary:     "Cannot reach managed instance",
			Description: "The managed instance is unreachable.",
			Status:      "ticket-open",
			OpenedBy:    "adevi",
			Entity: &linode.SupportTicketEntity{
				ID:    float64(1234),
				Label: "linode1234",
				Type:  "instance",
				URL:   "/v4/linode/instances/1234",
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(tickets))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListSupportTickets(t.Context(), 2, 25)

	require.NoError(t, err, "ListSupportTickets should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 11111, result.Data[0].ID)
	assert.Equal(t, "Cannot reach managed instance", result.Data[0].Summary)
	assert.Equal(t, "ticket-open", result.Data[0].Status)
	require.NotNil(t, result.Data[0].Entity)
	assert.Equal(t, "instance", result.Data[0].Entity.Type)
}

// TestClientListSupportTicketsAPIError verifies ListSupportTickets propagates API errors.
func TestClientListSupportTicketsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListSupportTickets(t.Context(), 0, 0)

	require.Error(t, err, "ListSupportTickets should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientListSupportTicketsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListSupportTicketsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.SupportTicket]{
			Data: []linode.SupportTicket{{ID: 11111, Summary: "Cannot reach managed instance"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListSupportTickets(t.Context(), 0, 0)

	require.NoError(t, err, "ListSupportTickets should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, 11111, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	supportTicketLookupSummary  = "Cannot reach managed instance"
	supportTicketLookupOpenedBy = "adevi"
)

func supportCheckEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("%s: expected %#v, got %#v", supportFailureMessage("values differ", msgAndArgs...), expected, actual)
}

func supportCheckEmpty(t *testing.T, value string, msgAndArgs ...any) {
	t.Helper()

	if value == "" {
		return
	}

	t.Errorf("%s: expected empty value, got %q", supportFailureMessage("value is not empty", msgAndArgs...), value)
}

func supportCheckNoError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	t.Errorf("unexpected error: %v", err)
}

func supportRequireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf("%s: unexpected error: %v", supportFailureMessage("expected no error", msgAndArgs...), err)
}

func supportRequireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		return
	}

	t.Fatalf("%s: expected error", supportFailureMessage("expected error", msgAndArgs...))
}

func supportRequireAPIError(t *testing.T, err error, msgAndArgs ...any) *linode.APIError {
	t.Helper()

	var apiErr *linode.APIError

	asError := errors.As

	if asError(err, &apiErr) {
		return apiErr
	}

	t.Fatalf("%s: expected API error, got %v", supportFailureMessage("expected API error", msgAndArgs...), err)

	return nil
}

func supportCheckNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if supportIsNil(value) {
		return
	}

	t.Errorf("%s: expected nil, got %#v", supportFailureMessage("expected nil", msgAndArgs...), value)
}

func supportRequireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if !supportIsNil(value) {
		return
	}

	t.Fatalf("%s: expected non-nil value", supportFailureMessage("expected non-nil value", msgAndArgs...))
}

func supportRequireLenOne[T any](t *testing.T, value []T) {
	t.Helper()

	if len(value) == 1 {
		return
	}

	t.Fatalf("length differs: expected length 1, got %d for %#v", len(value), value)
}

func supportIsNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	nilableKinds := map[reflect.Kind]struct{}{
		reflect.Chan:      {},
		reflect.Func:      {},
		reflect.Interface: {},
		reflect.Map:       {},
		reflect.Pointer:   {},
		reflect.Slice:     {},
	}

	if _, ok := nilableKinds[reflected.Kind()]; !ok {
		return false
	}

	return reflected.IsNil()
}

func supportFailureMessage(defaultMsg string, msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return defaultMsg
	}

	msg, ok := msgAndArgs[0].(string)
	if ok && msg != "" {
		return msg
	}

	return defaultMsg
}

// TestClientGetSupportTicketSuccess verifies GetSupportTicket sends a GET request to /support/tickets/{ticket_id}.
func TestClientGetSupportTicketSuccess(t *testing.T) {
	t.Parallel()

	wantTicket := linode.SupportTicket{ID: 11111, Summary: supportTicketLookupSummary, Status: "ticket-open", OpenedBy: supportTicketLookupOpenedBy}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets/11111", r.URL.Path, "request path should include ticket ID")
		supportCheckEmpty(t, r.URL.RawQuery, "get ticket should not include query parameters")
		supportCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(wantTicket))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetSupportTicket(t.Context(), 11111)

	supportRequireNoError(t, err, "GetSupportTicket should succeed on 200 response")
	supportCheckEqual(t, wantTicket.ID, result.ID)
	supportCheckEqual(t, wantTicket.Summary, result.Summary)
	supportCheckEqual(t, wantTicket.Status, result.Status)
}

// TestClientGetSupportTicketAPIError verifies GetSupportTicket propagates API errors.
func TestClientGetSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets/11111", r.URL.Path, "request path should include ticket ID")
		supportCheckEmpty(t, r.URL.RawQuery, "get ticket should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		supportCheckNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetSupportTicket(t.Context(), 11111)

	supportRequireError(t, err, "GetSupportTicket should fail on 403 response")

	apiErr := supportRequireAPIError(t, err, "error should wrap APIError")
	supportCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	supportCheckEqual(t, errForbidden, apiErr.Message)
}

// TestClientGetSupportTicketRetriesTransientError verifies the read-only get retries transient failures.
func TestClientGetSupportTicketRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			supportCheckNoError(t, writeErr)

			return
		}

		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets/11111", r.URL.Path, "request path should include ticket ID")
		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(linode.SupportTicket{ID: 11111, Summary: supportTicketLookupSummary}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetSupportTicket(t.Context(), 11111)

	supportRequireNoError(t, err, "GetSupportTicket should succeed after retry")
	supportCheckEqual(t, 11111, result.ID)
	supportCheckEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

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
		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		supportCheckEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		supportCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(tickets))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListSupportTickets(t.Context(), 2, 25)

	supportRequireNoError(t, err, "ListSupportTickets should succeed on 200 response")
	supportRequireNotNil(t, result, "result should not be nil")
	supportCheckEqual(t, 2, result.Page)
	supportRequireLenOne(t, result.Data)
	supportCheckEqual(t, 11111, result.Data[0].ID)
	supportCheckEqual(t, "Cannot reach managed instance", result.Data[0].Summary)
	supportCheckEqual(t, "ticket-open", result.Data[0].Status)
	supportRequireNotNil(t, result.Data[0].Entity)
	supportCheckEqual(t, "instance", result.Data[0].Entity.Type)
}

// TestClientListSupportTicketRepliesSuccess verifies ListSupportTicketReplies sends a GET
// request to /support/tickets/{ticket_id}/replies with pagination query parameters.
func TestClientListSupportTicketRepliesSuccess(t *testing.T) {
	t.Parallel()

	replies := linode.PaginatedResponse[linode.SupportTicketReply]{
		Data: []linode.SupportTicketReply{{
			ID:          22222,
			Description: "We are investigating this ticket.",
			CreatedBy:   supportTicketLookupOpenedBy,
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets/11111/replies", r.URL.Path, "request path should include ticket ID and replies")
		supportCheckEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		supportCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(replies))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListSupportTicketReplies(t.Context(), 11111, 2, 25)

	supportRequireNoError(t, err, "ListSupportTicketReplies should succeed on 200 response")
	supportRequireNotNil(t, result, "result should not be nil")
	supportCheckEqual(t, 2, result.Page)
	supportRequireLenOne(t, result.Data)
	supportCheckEqual(t, 22222, result.Data[0].ID)
	supportCheckEqual(t, "We are investigating this ticket.", result.Data[0].Description)
}

// TestClientListSupportTicketRepliesAPIError verifies ListSupportTicketReplies propagates API errors.
func TestClientListSupportTicketRepliesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets/11111/replies", r.URL.Path, "request path should include ticket ID and replies")
		supportCheckEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		supportCheckNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListSupportTicketReplies(t.Context(), 11111, 0, 0)

	supportRequireError(t, err, "ListSupportTicketReplies should fail on 403 response")

	apiErr := supportRequireAPIError(t, err, "error should wrap APIError")
	supportCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	supportCheckEqual(t, errForbidden, apiErr.Message)
}

// TestClientListSupportTicketRepliesRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListSupportTicketRepliesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			supportCheckNoError(t, writeErr)

			return
		}

		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets/11111/replies", r.URL.Path, "request path should include ticket ID and replies")
		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.SupportTicketReply]{
			Data: []linode.SupportTicketReply{{ID: 22222, Description: "We are investigating this ticket."}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListSupportTicketReplies(t.Context(), 11111, 0, 0)

	supportRequireNoError(t, err, "ListSupportTicketReplies should succeed after retry")
	supportRequireNotNil(t, result, "result should not be nil")
	supportRequireLenOne(t, result.Data)
	supportCheckEqual(t, 22222, result.Data[0].ID)
	supportCheckEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientListSupportTicketsAPIError verifies ListSupportTickets propagates API errors.
func TestClientListSupportTicketsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		supportCheckEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		supportCheckNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListSupportTickets(t.Context(), 0, 0)

	supportRequireError(t, err, "ListSupportTickets should fail on 403 response")

	apiErr := supportRequireAPIError(t, err, "error should wrap APIError")
	supportCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	supportCheckEqual(t, errForbidden, apiErr.Message)
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
			supportCheckNoError(t, writeErr)

			return
		}

		supportCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		supportCheckEqual(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.SupportTicket]{
			Data: []linode.SupportTicket{{ID: 11111, Summary: supportTicketLookupSummary}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListSupportTickets(t.Context(), 0, 0)

	supportRequireNoError(t, err, "ListSupportTickets should succeed after retry")
	supportRequireNotNil(t, result, "result should not be nil")
	supportRequireLenOne(t, result.Data)
	supportCheckEqual(t, 11111, result.Data[0].ID)
	supportCheckEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientCloseSupportTicketSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets/11111/close", r.URL.Path, "request path should close the support ticket")
		supportCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		supportCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CloseSupportTicket(t.Context(), 11111)

	supportRequireNoError(t, err, "CloseSupportTicket should succeed on 200 response")
}

func TestClientCloseSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets/11111/close", r.URL.Path, "request path should close the support ticket")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CloseSupportTicket(t.Context(), 11111)

	supportRequireError(t, err, "CloseSupportTicket should propagate API errors")
	apiErr := supportRequireAPIError(t, err, "error should wrap APIError")
	supportCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCloseSupportTicketDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets/11111/close", r.URL.Path, "request path should close the support ticket")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary support ticket close failure"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.CloseSupportTicket(t.Context(), 11111)

	supportRequireError(t, err, "CloseSupportTicket should return the transient error")
	supportCheckEqual(t, int32(1), requestCount.Load(), "mutating support ticket close must not be retried")
}

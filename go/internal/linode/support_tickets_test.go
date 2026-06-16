package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	supportTicketLookupSummary  = "Cannot reach managed instance"
	supportTicketLookupOpenedBy = "adevi"
)

func TestClientGetSupportTicketSuccess(t *testing.T) {
	t.Parallel()

	wantTicket := linode.SupportTicket{ID: 11111, Summary: supportTicketLookupSummary, Status: "ticket-open", OpenedBy: supportTicketLookupOpenedBy}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets11111 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(wantTicket); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetSupportTicket(t.Context(), 11111)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != wantTicket.ID {
		t.Errorf("result.ID = %v, want %v", result.ID, wantTicket.ID)
	}

	if result.Summary != wantTicket.Summary {
		t.Errorf("result.Summary = %v, want %v", result.Summary, wantTicket.Summary)
	}

	if result.Status != wantTicket.Status {
		t.Errorf("result.Status = %v, want %v", result.Status, wantTicket.Status)
	}
}

// TestClientGetSupportTicketAPIError verifies GetSupportTicket propagates API errors.
func TestClientGetSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets11111 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetSupportTicket(t.Context(), 11111)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
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
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets11111 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.SupportTicket{ID: 11111, Summary: supportTicketLookupSummary}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetSupportTicket(t.Context(), 11111)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != 11111 {
		t.Errorf("result.ID = %v, want %v", result.ID, 11111)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(tickets); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListSupportTickets(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].ID != 11111 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 11111)
	}

	if result.Data[0].Summary != "Cannot reach managed instance" {
		t.Errorf("result.Data[0].Summary = %v, want %v", result.Data[0].Summary, "Cannot reach managed instance")
	}

	if result.Data[0].Status != tcTicketOpen {
		t.Errorf("result.Data[0].Status = %v, want %v", result.Data[0].Status, tcTicketOpen)
	}

	if result.Data[0].Entity == nil {
		t.Fatal("result.Data[0].Entity is nil")
	}

	if result.Data[0].Entity.Type != "instance" {
		t.Errorf("result.Data[0].Entity.Type = %v, want %v", result.Data[0].Entity.Type, "instance")
	}
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets11111Replies {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111Replies)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(replies); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListSupportTicketReplies(t.Context(), 11111, 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].ID != 22222 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 22222)
	}

	if result.Data[0].Description != tcWeAreInvestigatingThisTicket {
		t.Errorf("result.Data[0].Description = %v, want %v", result.Data[0].Description, tcWeAreInvestigatingThisTicket)
	}
}

// TestClientListSupportTicketRepliesAPIError verifies ListSupportTicketReplies propagates API errors.
func TestClientListSupportTicketRepliesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets11111Replies {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111Replies)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListSupportTicketReplies(t.Context(), 11111, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
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
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets11111Replies {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111Replies)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.SupportTicketReply]{
			Data: []linode.SupportTicketReply{{ID: 22222, Description: "We are investigating this ticket."}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListSupportTicketReplies(t.Context(), 11111, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].ID != 22222 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 22222)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientListSupportTicketsAPIError verifies ListSupportTickets propagates API errors.
func TestClientListSupportTicketsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListSupportTickets(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
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
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.SupportTicket]{
			Data: []linode.SupportTicket{{ID: 11111, Summary: supportTicketLookupSummary}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListSupportTickets(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].ID != 11111 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 11111)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientCloseSupportTicketSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets11111Close {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111Close)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CloseSupportTicket(t.Context(), 11111)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientCloseSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets11111Close {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111Close)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CloseSupportTicket(t.Context(), 11111)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientCloseSupportTicketDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets11111Close {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets11111Close)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary support ticket close failure"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.CloseSupportTicket(t.Context(), 11111)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

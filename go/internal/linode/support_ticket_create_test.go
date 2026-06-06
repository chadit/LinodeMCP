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
	supportTicketSummary        = "Need help"
	supportTicketDescription    = "Instance is unreachable"
	supportTicketStatusOpen     = "open"
	temporarySupportTicketError = "temporary support ticket failure"
	supportTicketSeverity       = "major"
)

func TestClientCreateSupportTicketSuccess(t *testing.T) {
	t.Parallel()

	linodeID := 12345
	severity := supportTicketSeverity
	request := &linode.CreateSupportTicketRequest{
		Summary:     supportTicketSummary,
		Description: supportTicketDescription,
		LinodeID:    &linodeID,
		Severity:    &severity,
	}
	created := linode.SupportTicket{ID: 987, Summary: request.Summary, Description: request.Description, Status: supportTicketStatusOpen}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		supportCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		supportCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got map[string]any
		supportCheckNoError(t, json.NewDecoder(r.Body).Decode(&got))
		supportCheckEqual(t, supportTicketSummary, got["summary"])
		supportCheckEqual(t, supportTicketDescription, got["description"])
		supportCheckEqual(t, float64(12345), got["linode_id"])
		supportCheckEqual(t, supportTicketSeverity, got["severity"])

		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicket(t.Context(), request)

	supportRequireNoError(t, err, "CreateSupportTicket should succeed on 200 response")
	supportRequireNotNil(t, got, "result should not be nil")
	supportCheckEqual(t, created.ID, got.ID)
	supportCheckEqual(t, created.Summary, got.Summary)
}

func TestClientCreateSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicket(t.Context(), &linode.CreateSupportTicketRequest{Summary: supportTicketSummary, Description: supportTicketDescription})

	supportRequireError(t, err, "CreateSupportTicket should propagate API errors")
	supportCheckNil(t, got)
	apiErr := supportRequireAPIError(t, err, "error should wrap APIError")
	supportCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateSupportTicketDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		w.WriteHeader(http.StatusInternalServerError)
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporarySupportTicketError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateSupportTicket(t.Context(), &linode.CreateSupportTicketRequest{Summary: supportTicketSummary, Description: supportTicketDescription})

	supportRequireError(t, err, "CreateSupportTicket should return the transient error")
	supportCheckEqual(t, int32(1), requestCount.Load(), "mutating support ticket creation must not be retried")
}

package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	supportTicketSummary        = "Need help"
	supportTicketDescription    = "Instance is unreachable"
	supportTicketStatusOpen     = "open"
	temporarySupportTicketError = "temporary support ticket failure"
	supportTicketSeverity       = 2
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
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got["summary"], supportTicketSummary) {
			t.Errorf("got %v, want %v", got["summary"], supportTicketSummary)
		}

		if !reflect.DeepEqual(got["description"], supportTicketDescription) {
			t.Errorf("got %v, want %v", got["description"], supportTicketDescription)
		}

		if !reflect.DeepEqual(got["linode_id"], float64(12345)) {
			t.Errorf("got %v, want %v", got["linode_id"], float64(12345))
		}

		if !reflect.DeepEqual(got["severity"], float64(supportTicketSeverity)) {
			t.Errorf("got %v, want %v", got["severity"], float64(supportTicketSeverity))
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicket(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != created.ID {
		t.Errorf("got.ID = %v, want %v", got.ID, created.ID)
	}

	if got.Summary != created.Summary {
		t.Errorf("got.Summary = %v, want %v", got.Summary, created.Summary)
	}
}

func TestClientCreateSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicket(t.Context(), &linode.CreateSupportTicketRequest{Summary: supportTicketSummary, Description: supportTicketDescription})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientCreateSupportTicketDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporarySupportTicketError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateSupportTicket(t.Context(), &linode.CreateSupportTicketRequest{Summary: supportTicketSummary, Description: supportTicketDescription})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

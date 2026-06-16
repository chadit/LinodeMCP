package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	supportTicketReplyDescription = "Thanks, here is more detail."
	supportTicketReplyError       = "temporary reply failure"
)

func TestClientCreateSupportTicketReplySuccess(t *testing.T) {
	t.Parallel()

	created := linode.SupportTicketReply{ID: 456, Description: supportTicketReplyDescription, CreatedBy: supportTicketLookupOpenedBy}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets123Replies {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets123Replies)
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

		if !reflect.DeepEqual(got["description"], supportTicketReplyDescription) {
			t.Errorf("got %v, want %v", got["description"], supportTicketReplyDescription)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicketReply(t.Context(), 123, &linode.CreateSupportTicketReplyRequest{Description: supportTicketReplyDescription})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != created.ID {
		t.Errorf("got.ID = %v, want %v", got.ID, created.ID)
	}

	if got.Description != supportTicketReplyDescription {
		t.Errorf("got.Description = %v, want %v", got.Description, supportTicketReplyDescription)
	}
}

func TestClientCreateSupportTicketReplyAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets123Replies {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets123Replies)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "denied"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicketReply(t.Context(), 123, &linode.CreateSupportTicketReplyRequest{Description: supportTicketReplyDescription})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientCreateSupportTicketReplyDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets123Replies {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets123Replies)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusGatewayTimeout)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: supportTicketReplyError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateSupportTicketReply(t.Context(), 123, &linode.CreateSupportTicketReplyRequest{Description: supportTicketReplyDescription})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

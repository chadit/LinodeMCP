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
	supportTicketAttachmentFile     = "attachment-content"
	supportTicketAttachmentFilename = "diagnostics.txt"
	supportTicketAttachmentError    = "temporary attachment failure"
)

func TestClientCreateSupportTicketAttachmentSuccess(t *testing.T) {
	t.Parallel()

	created := linode.SupportTicketAttachment{ID: 654, Filename: supportTicketAttachmentFilename, Size: 128}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets/123/attachments", r.URL.Path, "request path should include ticket ID")
		supportCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		supportCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got map[string]any
		supportCheckNoError(t, json.NewDecoder(r.Body).Decode(&got))
		supportCheckEqual(t, supportTicketAttachmentFile, got["file"])

		w.Header().Set("Content-Type", "application/json")
		supportCheckNoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicketAttachment(t.Context(), 123, &linode.CreateSupportTicketAttachmentRequest{File: supportTicketAttachmentFile})

	supportRequireNoError(t, err, "CreateSupportTicketAttachment should succeed on 200 response")
	supportRequireNotNil(t, got, "result should not be nil")
	supportCheckEqual(t, created.ID, got.ID)
	supportCheckEqual(t, created.Filename, got.Filename)
	supportCheckEqual(t, created.Size, got.Size)
}

func TestClientCreateSupportTicketAttachmentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets/123/attachments", r.URL.Path, "request path should include ticket ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicketAttachment(t.Context(), 123, &linode.CreateSupportTicketAttachmentRequest{File: supportTicketAttachmentFile})

	supportRequireError(t, err, "CreateSupportTicketAttachment should propagate API errors")
	supportCheckNil(t, got)
	apiErr := supportRequireAPIError(t, err, "error should wrap APIError")
	supportCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateSupportTicketAttachmentDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		supportCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		supportCheckEqual(t, "/support/tickets/123/attachments", r.URL.Path, "request path should include ticket ID")
		w.WriteHeader(http.StatusInternalServerError)
		supportCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: supportTicketAttachmentError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateSupportTicketAttachment(t.Context(), 123, &linode.CreateSupportTicketAttachmentRequest{File: supportTicketAttachmentFile})

	supportRequireError(t, err, "CreateSupportTicketAttachment should return the transient error")
	supportCheckEqual(t, int32(1), requestCount.Load(), "mutating support ticket attachment creation must not be retried")
}

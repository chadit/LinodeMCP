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

const (
	supportTicketAttachmentFile     = "attachment-content"
	supportTicketAttachmentFilename = "diagnostics.txt"
	supportTicketAttachmentError    = "temporary attachment failure"
)

func TestClientCreateSupportTicketAttachmentSuccess(t *testing.T) {
	t.Parallel()

	created := linode.SupportTicketAttachment{ID: 654, Filename: supportTicketAttachmentFilename, Size: 128}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets/123/attachments", r.URL.Path, "request path should include ticket ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, supportTicketAttachmentFile, got["file"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicketAttachment(t.Context(), 123, &linode.CreateSupportTicketAttachmentRequest{File: supportTicketAttachmentFile})

	require.NoError(t, err, "CreateSupportTicketAttachment should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Filename, got.Filename)
	assert.Equal(t, created.Size, got.Size)
}

func TestClientCreateSupportTicketAttachmentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets/123/attachments", r.URL.Path, "request path should include ticket ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicketAttachment(t.Context(), 123, &linode.CreateSupportTicketAttachmentRequest{File: supportTicketAttachmentFile})

	require.Error(t, err, "CreateSupportTicketAttachment should propagate API errors")
	assert.Nil(t, got)
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientCreateSupportTicketAttachmentDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets/123/attachments", r.URL.Path, "request path should include ticket ID")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: supportTicketAttachmentError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateSupportTicketAttachment(t.Context(), 123, &linode.CreateSupportTicketAttachmentRequest{File: supportTicketAttachmentFile})

	require.Error(t, err, "CreateSupportTicketAttachment should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating support ticket attachment creation must not be retried")
}

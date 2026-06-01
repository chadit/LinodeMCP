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
	supportTicketReplyDescription = "Thanks, here is more detail."
	supportTicketReplyError       = "temporary reply failure"
)

func TestClientCreateSupportTicketReplySuccess(t *testing.T) {
	t.Parallel()

	created := linode.SupportTicketReply{ID: 456, Description: supportTicketReplyDescription, CreatedBy: supportTicketLookupOpenedBy}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets/123/replies", r.URL.Path, "request path should include ticket ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, supportTicketReplyDescription, got["description"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateSupportTicketReply(t.Context(), 123, &linode.CreateSupportTicketReplyRequest{Description: supportTicketReplyDescription})

	require.NoError(t, err, "CreateSupportTicketReply should succeed on 200 response")
	require.NotNil(t, got, "created reply should not be nil")
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, supportTicketReplyDescription, got.Description)
}

func TestClientCreateSupportTicketReplyAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets/123/replies", r.URL.Path, "request path should include ticket ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "denied"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateSupportTicketReply(t.Context(), 123, &linode.CreateSupportTicketReplyRequest{Description: supportTicketReplyDescription})

	require.Error(t, err, "CreateSupportTicketReply should propagate API errors")
	assert.Nil(t, got, "failed request should not return a reply")
}

func TestClientCreateSupportTicketReplyDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets/123/replies", r.URL.Path, "request path should include ticket ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGatewayTimeout)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: supportTicketReplyError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateSupportTicketReply(t.Context(), 123, &linode.CreateSupportTicketReplyRequest{Description: supportTicketReplyDescription})

	require.Error(t, err, "CreateSupportTicketReply should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating support ticket reply creation must not be retried")
}

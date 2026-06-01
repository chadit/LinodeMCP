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
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, supportTicketSummary, got["summary"])
		assert.Equal(t, supportTicketDescription, got["description"])
		assert.InDelta(t, float64(12345), got["linode_id"], 0)
		assert.Equal(t, supportTicketSeverity, got["severity"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicket(t.Context(), request)

	require.NoError(t, err, "CreateSupportTicket should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Summary, got.Summary)
}

func TestClientCreateSupportTicketAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateSupportTicket(t.Context(), &linode.CreateSupportTicketRequest{Summary: supportTicketSummary, Description: supportTicketDescription})

	require.Error(t, err, "CreateSupportTicket should propagate API errors")
	assert.Nil(t, got)
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientCreateSupportTicketDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporarySupportTicketError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateSupportTicket(t.Context(), &linode.CreateSupportTicketRequest{Summary: supportTicketSummary, Description: supportTicketDescription})

	require.Error(t, err, "CreateSupportTicket should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating support ticket creation must not be retried")
}

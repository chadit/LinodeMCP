package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientApplyInstanceFirewallsSuccess(t *testing.T) {
	t.Parallel()

	var bodySeen atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/firewalls/apply", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query params")

		if r.Body != nil && r.ContentLength != 0 {
			bodySeen.Store(true)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("{}"))
		assert.NoError(t, err, "writing response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.ApplyInstanceFirewalls(t.Context(), 123)

	require.NoError(t, err, "apply firewalls should succeed")
	assert.False(t, bodySeen.Load(), "request should not send a body")
}

func TestClientApplyInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil, linode.WithMaxRetries(0))
	err := client.ApplyInstanceFirewalls(t.Context(), 0)

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")
}

func TestClientApplyInstanceFirewallsDoesNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/firewalls/apply", r.URL.Path, "request path should match")
		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
	err := client.ApplyInstanceFirewalls(t.Context(), 123)

	require.Error(t, err, "transient error should be returned")
	assert.Equal(t, int32(1), calls.Load(), "mutating POST must not be replayed")
}

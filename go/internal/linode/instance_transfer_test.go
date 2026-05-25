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

func TestClientGetInstanceTransferSuccess(t *testing.T) {
	t.Parallel()

	transfer := linode.InstanceTransfer{
		Billable: 0,
		Quota:    2000,
		Used:     22956600198,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/transfer", r.URL.EscapedPath(), "request path should include encoded Linode ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(transfer))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetInstanceTransfer(t.Context(), 123)

	require.NoError(t, err, "GetInstanceTransfer should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 0, result.Billable)
	assert.Equal(t, 2000, result.Quota)
	assert.Equal(t, int64(22956600198), result.Used)
}

func TestClientGetInstanceTransferRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://example.invalid", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceTransfer(t.Context(), 0)

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid Linode ID should be rejected before request")
}

func TestClientGetInstanceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/transfer", r.URL.Path, "request path should include Linode ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceTransfer(t.Context(), 123)

	require.Error(t, err, "GetInstanceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should expose APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, "forbidden", apiErr.Message)
}

func TestClientGetInstanceTransferRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/transfer", r.URL.Path, "request path should include Linode ID")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceTransfer{Used: 123}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetInstanceTransfer(t.Context(), 123)

	require.NoError(t, err, "GetInstanceTransfer should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, int64(123), result.Used)
	assert.GreaterOrEqual(t, calls.Load(), int32(2), "read-only transfer lookup should retry transient failure")
}

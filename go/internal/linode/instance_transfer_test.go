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

func TestClientGetInstanceTransferByYearMonthSuccess(t *testing.T) {
	t.Parallel()

	transfer := linode.Transfer{In: 1.5, Out: 2.5, Total: 4}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/transfer/2024/1", r.URL.EscapedPath(), "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(transfer))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetInstanceTransferByYearMonth(t.Context(), 123, 2024, 1)

	require.NoError(t, err, "GetInstanceTransferByYearMonth should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, transfer, *result)
}

func TestClientGetInstanceTransferByYearMonthAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/transfer/2024/1", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceTransferByYearMonth(t.Context(), 123, 2024, 1)

	require.Error(t, err, "GetInstanceTransferByYearMonth should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetInstanceTransferByYearMonthRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			hj, ok := w.(http.Hijacker)
			if !assert.True(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !assert.NoError(t, err, "hijack should succeed") {
				return
			}

			assert.NoError(t, conn.Close(), "closing hijacked connection should succeed")

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/transfer/2024/1", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Transfer{Total: 4}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetInstanceTransferByYearMonth(t.Context(), 123, 2024, 1)

	require.NoError(t, err, "GetInstanceTransferByYearMonth should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.InDelta(t, float64(4), result.Total, 0.001)
	assert.Equal(t, int32(2), attempts.Load(), "read-only transfer should retry once")
}

func TestClientGetInstanceTransferByYearMonthValidatesPathParams(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://example.invalid", "my-token", nil, linode.WithMaxRetries(0))

	cases := []struct {
		name    string
		id      int
		year    int
		mon     int
		wantErr error
	}{
		{name: "zero linode id", id: 0, year: 2024, mon: 1, wantErr: linode.ErrLinodeIDPositive},
		{name: "zero year", id: 123, year: 0, mon: 1, wantErr: linode.ErrTransferYearPositive},
		{name: "zero month", id: 123, year: 2024, mon: 0, wantErr: linode.ErrTransferMonthRange},
		{name: "month too large", id: 123, year: 2024, mon: 13, wantErr: linode.ErrTransferMonthRange},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.GetInstanceTransferByYearMonth(t.Context(), tc.id, tc.year, tc.mon)
			require.ErrorIs(t, err, tc.wantErr, "invalid path params should fail before dispatch with sentinel error")
		})
	}
}

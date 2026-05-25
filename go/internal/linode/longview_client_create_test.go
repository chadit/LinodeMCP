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
	longviewClientCreated = "2018-01-01T00:01:01"
	longviewClientUpdated = "2018-01-02T00:01:01"
)

func TestClientCreateLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.CreatedLongviewClient{
		APIKey:      longviewClientAPIKey,
		Apps:        linode.LongviewApps{Apache: true, MySQL: true},
		Created:     longviewClientCreated,
		ID:          789,
		InstallCode: longviewClientInstallCode,
		Label:       longviewClientLabel,
		Updated:     longviewClientUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/longview/clients", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var got linode.CreateLongviewClientRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, longviewClientLabel, got.Label)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	got, err := client.CreateLongviewClient(t.Context(), req)

	require.NoError(t, err, "CreateLongviewClient should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientCreateLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/longview/clients", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	_, err := client.CreateLongviewClient(t.Context(), req)

	require.Error(t, err, "CreateLongviewClient should surface API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientCreateLongviewClientDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		http.Error(w, "transient", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	_, err := client.CreateLongviewClient(t.Context(), req)

	require.Error(t, err, "CreateLongviewClient should return the transient error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating Longview client creation must not be retried")
}

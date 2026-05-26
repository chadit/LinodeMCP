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
	longviewSubscriptionID   = "longview-10"
	longviewSubscriptionPath = "/longview/subscriptions/" + longviewSubscriptionID
)

func TestClientGetLongviewSubscriptionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewSubscriptionPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 10,
			keyID:              longviewSubscriptionID,
			keyLabel:           "Longview Pro 10 pack",
			keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, longviewSubscriptionID, got.ID)
	assert.Equal(t, "Longview Pro 10 pack", got.Label)
	assert.Equal(t, 10, got.ClientsIncluded)
	assert.InEpsilon(t, 0.06, got.Price.Hourly, 0.001)
	assert.InEpsilon(t, 40.0, got.Price.Monthly, 0.001)
}

func TestClientGetLongviewSubscriptionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewSubscriptionPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetLongviewSubscriptionEscapesSubscriptionID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/longview/subscriptions/longview-10%2F..", r.URL.EscapedPath(), "subscription ID should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: longviewSubscriptionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewSubscription(t.Context(), "longview-10/..")

	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestClientGetLongviewSubscriptionRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewSubscriptionPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: longviewSubscriptionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	assert.Equal(t, longviewSubscriptionID, got.ID)
}

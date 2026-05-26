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
	longviewSubscriptionsPath  = "/longview/subscriptions"
	longviewSubscriptionsQuery = "page=2&page_size=25"
	longviewPlan10ID           = "longview-10"
	longviewPlan10Label        = "Longview Pro 10 pack"
)

func TestClientListLongviewSubscriptionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewSubscriptionsPath, r.URL.Path, "request path should match")
		assert.Equal(t, longviewSubscriptionsQuery, r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyClientsIncluded: 10,
				keyID:              longviewPlan10ID,
				keyLabel:           longviewPlan10Label,
				keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewSubscriptions(t.Context(), 2, 25)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 2, got.Page)
	assert.Equal(t, 3, got.Pages)
	assert.Equal(t, 75, got.Results)
	require.Len(t, got.Data, 1)
	assert.Equal(t, longviewPlan10ID, got.Data[0].ID)
	assert.Equal(t, longviewPlan10Label, got.Data[0].Label)
	assert.Equal(t, 10, got.Data[0].ClientsIncluded)
	assert.InEpsilon(t, 0.06, got.Data[0].Price.Hourly, 0.001)
	assert.InEpsilon(t, 40.0, got.Data[0].Price.Monthly, 0.001)
}

func TestClientListLongviewSubscriptionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewSubscriptionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewSubscriptions(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListLongviewSubscriptionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, longviewSubscriptionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: longviewPlan10ID, keyLabel: longviewPlan10Label}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListLongviewSubscriptions(t.Context(), 0, 0)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.Equal(t, longviewPlan10ID, got.Data[0].ID)
}

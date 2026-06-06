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
	longviewSubscriptionID   = "longview-10"
	longviewSubscriptionPath = "/longview/subscriptions/" + longviewSubscriptionID
)

func TestClientGetLongviewSubscriptionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewSubscriptionPath, r.URL.Path, "request path should match")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 10,
			keyID:              longviewSubscriptionID,
			keyLabel:           longviewPlan10Label,
			keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, longviewSubscriptionID, got.ID)
	longviewCheckEqual(t, longviewPlan10Label, got.Label)
	longviewCheckEqual(t, 10, got.ClientsIncluded)
	longviewCheckInEpsilon(t, 0.06, got.Price.Hourly)
	longviewCheckInEpsilon(t, 40.0, got.Price.Monthly)
}

func TestClientGetLongviewSubscriptionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewSubscriptionPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetLongviewSubscriptionEscapesSubscriptionID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, "/longview/subscriptions/longview-10%2F..", r.URL.EscapedPath(), "subscription ID should be escaped")
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: longviewSubscriptionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewSubscription(t.Context(), "longview-10/..")

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
}

func TestClientGetLongviewSubscriptionRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewSubscriptionPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: longviewSubscriptionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	longviewCheckEqual(t, longviewSubscriptionID, got.ID)
}

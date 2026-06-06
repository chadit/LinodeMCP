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
	longviewPlanPath               = "/longview/plan"
	longviewSubscriptionPlan       = "longview-40"
	keyLongviewClientsIncluded     = "clients_included"
	keyLongviewSubscriptionPrice   = "price"
	keyLongviewSubscriptionHourly  = "hourly"
	keyLongviewSubscriptionMonthly = "monthly"
)

func TestClientGetLongviewPlanSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 10,
			keyID:              "longview-10",
			keyLabel:           longviewPlan10Label,
			keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewPlan(t.Context())

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, "longview-10", got.ID)
	longviewCheckEqual(t, longviewPlan10Label, got.Label)
	longviewCheckEqual(t, 10, got.ClientsIncluded)
	longviewCheckInEpsilon(t, 0.06, got.Price.Hourly)
	longviewCheckInEpsilon(t, 40.0, got.Price.Monthly)
}

func TestClientGetLongviewPlanFreePlan(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewPlan(t.Context())

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEmpty(t, got.ID)
	longviewCheckEmpty(t, got.ClientsIncluded)
}

func TestClientGetLongviewPlanAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetLongviewPlan(t.Context())

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetLongviewPlanRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: "longview-10"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetLongviewPlan(t.Context())

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	longviewCheckEqual(t, "longview-10", got.ID)
}

func TestClientUpdateLongviewPlanSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		longviewCheckNoError(t, json.NewDecoder(r.Body).Decode(&body))
		longviewCheckEqual(t, map[string]any{"longview_subscription": longviewSubscriptionPlan}, body, "request body should match")

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 40,
			keyID:              longviewSubscriptionPlan,
			keyLabel:           "Longview Pro 40 pack",
			keyPrice:           map[string]float64{keyHourly: 0.12, keyMonthly: 80},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateLongviewPlan(t.Context(), &linode.UpdateLongviewPlanRequest{LongviewSubscription: longviewSubscriptionPlan})

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, longviewSubscriptionPlan, got.ID)
	longviewCheckEqual(t, "Longview Pro 40 pack", got.Label)
	longviewCheckEqual(t, 40, got.ClientsIncluded)
}

func TestClientUpdateLongviewPlanAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateLongviewPlan(t.Context(), &linode.UpdateLongviewPlanRequest{LongviewSubscription: longviewSubscriptionPlan})

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientUpdateLongviewPlanDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		longviewCheckEqual(t, longviewPlanPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateLongviewPlan(t.Context(), &linode.UpdateLongviewPlanRequest{LongviewSubscription: longviewSubscriptionPlan})

	longviewRequireError(t, err, "UpdateLongviewPlan should fail on 503 response")
	longviewCheckEqual(t, int32(1), calls.Load(), "UpdateLongviewPlan must not retry and replay a mutating request")
}

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
	longviewSubscriptionsPath  = "/longview/subscriptions"
	longviewSubscriptionsQuery = "page=2&page_size=25"
	longviewPlan10ID           = "longview-10"
	longviewPlan10Label        = "Longview Pro 10 pack"
)

func TestClientListLongviewSubscriptionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewSubscriptionsPath, r.URL.Path, "request path should match")
		longviewCheckEqual(t, longviewSubscriptionsQuery, r.URL.RawQuery, "request query should include pagination")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, 2, got.Page)
	longviewCheckEqual(t, 3, got.Pages)
	longviewCheckEqual(t, 75, got.Results)
	longviewRequireLenOne(t, got.Data)
	longviewCheckEqual(t, longviewPlan10ID, got.Data[0].ID)
	longviewCheckEqual(t, longviewPlan10Label, got.Data[0].Label)
	longviewCheckEqual(t, 10, got.Data[0].ClientsIncluded)
	longviewCheckInEpsilon(t, 0.06, got.Data[0].Price.Hourly)
	longviewCheckInEpsilon(t, 40.0, got.Data[0].Price.Monthly)
}

func TestClientListLongviewSubscriptionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewSubscriptionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewSubscriptions(t.Context(), 0, 0)

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListLongviewSubscriptionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewSubscriptionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: longviewPlan10ID, keyLabel: longviewPlan10Label}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListLongviewSubscriptions(t.Context(), 0, 0)

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	longviewRequireLenOne(t, got.Data)
	longviewCheckEqual(t, longviewPlan10ID, got.Data[0].ID)
}

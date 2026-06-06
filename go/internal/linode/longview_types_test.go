package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const longviewTypesPath = "/longview/types"

func TestClientListLongviewTypesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewTypesPath, r.URL.Path, "request path should match")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyClientsIncluded: 10,
				keyID:              longviewPlan10,
				keyLabel:           longviewPlan10Label,
				keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewTypes(t.Context())

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewRequireLenOne(t, got.Data)
	longviewCheckEqual(t, longviewPlan10, got.Data[0].ID)
	longviewCheckEqual(t, longviewPlan10Label, got.Data[0].Label)
	longviewCheckEqual(t, 10, got.Data[0].ClientsIncluded)
	longviewCheckInEpsilon(t, 0.06, got.Data[0].Price.Hourly)
	longviewCheckInEpsilon(t, 40.0, got.Data[0].Price.Monthly)
}

func TestClientListLongviewTypesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewTypesPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewTypes(t.Context())

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListLongviewTypesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewTypesPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: longviewPlan10}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListLongviewTypes(t.Context())

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	longviewRequireLenOne(t, got.Data)
	longviewCheckEqual(t, longviewPlan10, got.Data[0].ID)
}

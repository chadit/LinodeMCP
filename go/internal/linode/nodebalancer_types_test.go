package linode_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const nodeBalancerTypesPath = "/nodebalancers/types"

func TestClientListNodeBalancerTypesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerTypesPath, r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:      "common",
				keyLabel:   "Common",
				keyPrice:   map[string]float64{keyHourly: 0.015, keyMonthly: 10},
				"transfer": 1000,
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerTypes(t.Context())

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbRequireLenOne(t, got.Data)

	nbCheckEqual(t, "common", got.Data[0].ID)
	nbCheckEqual(t, "Common", got.Data[0].Label)

	if math.Abs(got.Data[0].Price.Hourly-0.015) > 0.001 {
		t.Errorf("%s: expected %v +/- %v, got %v", "float value should be within epsilon", 0.015, 0.001, got.Data[0].Price.Hourly)
	}

	if math.Abs(got.Data[0].Price.Monthly-10.0) > 0.001 {
		t.Errorf("%s: expected %v +/- %v, got %v", "float value should be within epsilon", 10.0, 0.001, got.Data[0].Price.Monthly)
	}

	nbCheckEqual(t, 1000, got.Data[0].Transfer)
}

func TestClientListNodeBalancerTypesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerTypesPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerTypes(t.Context())

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListNodeBalancerTypesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerTypesPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: "common"}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListNodeBalancerTypes(t.Context())

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	nbRequireLenOne(t, got.Data)

	nbCheckEqual(t, "common", got.Data[0].ID)
}

package linode_test

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 10,
			keyID:              "longview-10",
			keyLabel:           longviewPlan10Label,
			keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetLongviewPlan(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != longviewSubscriptionID {
		t.Errorf("got.ID = %v, want %v", got.ID, longviewSubscriptionID)
	}

	if got.Label != longviewPlan10Label {
		t.Errorf("got.Label = %v, want %v", got.Label, longviewPlan10Label)
	}

	if got.ClientsIncluded != 10 {
		t.Errorf("got.ClientsIncluded = %v, want %v", got.ClientsIncluded, 10)
	}

	if math.Abs(0.06-got.Price.Hourly) > math.Abs(0.06)*0.001 {
		t.Errorf("got %v, want %v", got.Price.Hourly, 0.06)
	}

	if math.Abs(40.0-got.Price.Monthly) > math.Abs(40.0)*0.001 {
		t.Errorf("got %v, want %v", got.Price.Monthly, 40.0)
	}
}

func TestClientGetLongviewPlanFreePlan(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetLongviewPlan(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != "" {
		t.Errorf("got.ID = %v, want empty", got.ID)
	}

	if got.ClientsIncluded != 0 {
		t.Errorf("got.ClientsIncluded = %v, want empty", got.ClientsIncluded)
	}
}

func TestClientGetLongviewPlanAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetLongviewPlan(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientGetLongviewPlanRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: "longview-10"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetLongviewPlan(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if got.ID != longviewSubscriptionID {
		t.Errorf("got.ID = %v, want %v", got.ID, longviewSubscriptionID)
	}
}

func TestClientUpdateLongviewPlanSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body, map[string]any{"longview_subscription": longviewSubscriptionPlan}) {
			t.Errorf("body = %v, want %v", body, map[string]any{"longview_subscription": longviewSubscriptionPlan})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyClientsIncluded: 40,
			keyID:              longviewSubscriptionPlan,
			keyLabel:           "Longview Pro 40 pack",
			keyPrice:           map[string]float64{keyHourly: 0.12, keyMonthly: 80},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateLongviewPlan(t.Context(), &linode.UpdateLongviewPlanRequest{LongviewSubscription: longviewSubscriptionPlan})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != longviewSubscriptionPlan {
		t.Errorf("got.ID = %v, want %v", got.ID, longviewSubscriptionPlan)
	}

	if got.Label != "Longview Pro 40 pack" {
		t.Errorf("got.Label = %v, want %v", got.Label, "Longview Pro 40 pack")
	}

	if got.ClientsIncluded != 40 {
		t.Errorf("got.ClientsIncluded = %v, want %v", got.ClientsIncluded, 40)
	}
}

func TestClientUpdateLongviewPlanAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateLongviewPlan(t.Context(), &linode.UpdateLongviewPlanRequest{LongviewSubscription: longviewSubscriptionPlan})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUpdateLongviewPlanDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != longviewPlanPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewPlanPath)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateLongviewPlan(t.Context(), &linode.UpdateLongviewPlanRequest{LongviewSubscription: longviewSubscriptionPlan})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

package linode_test

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewSubscriptionsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewSubscriptionsPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyClientsIncluded: 10,
				keyID:              longviewPlan10ID,
				keyLabel:           longviewPlan10Label,
				keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListLongviewSubscriptions(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Page != 2 {
		t.Errorf("got.Page = %v, want %v", got.Page, 2)
	}

	if got.Pages != 3 {
		t.Errorf("got.Pages = %v, want %v", got.Pages, 3)
	}

	if got.Results != 75 {
		t.Errorf("got.Results = %v, want %v", got.Results, 75)
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != longviewPlan10ID {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, longviewPlan10ID)
	}

	if got.Data[0].Label != longviewPlan10Label {
		t.Errorf("got.Data[0].Label = %v, want %v", got.Data[0].Label, longviewPlan10Label)
	}

	if got.Data[0].ClientsIncluded != 10 {
		t.Errorf("got.Data[0].ClientsIncluded = %v, want %v", got.Data[0].ClientsIncluded, 10)
	}

	if math.Abs(0.06-got.Data[0].Price.Hourly) > math.Abs(0.06)*0.001 {
		t.Errorf("got %v, want %v", got.Data[0].Price.Hourly, 0.06)
	}

	if math.Abs(40.0-got.Data[0].Price.Monthly) > math.Abs(40.0)*0.001 {
		t.Errorf("got %v, want %v", got.Data[0].Price.Monthly, 40.0)
	}
}

func TestClientListLongviewSubscriptionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewSubscriptionsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewSubscriptionsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListLongviewSubscriptions(t.Context(), 0, 0)
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

func TestClientListLongviewSubscriptionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewSubscriptionsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewSubscriptionsPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: longviewPlan10ID, keyLabel: longviewPlan10Label}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListLongviewSubscriptions(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != longviewPlan10ID {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, longviewPlan10ID)
	}
}

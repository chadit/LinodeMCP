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
	longviewSubscriptionID   = "longview-10"
	longviewSubscriptionPath = "/longview/subscriptions/" + longviewSubscriptionID
)

func TestClientGetLongviewSubscriptionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewSubscriptionPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewSubscriptionPath)
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
			keyID:              longviewSubscriptionID,
			keyLabel:           longviewPlan10Label,
			keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)
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

func TestClientGetLongviewSubscriptionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewSubscriptionPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewSubscriptionPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)
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

func TestClientGetLongviewSubscriptionEscapesSubscriptionID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/longview/subscriptions/longview-10%2F.." {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/longview/subscriptions/longview-10%2F..")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: longviewSubscriptionID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetLongviewSubscription(t.Context(), "longview-10/..")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}
}

func TestClientGetLongviewSubscriptionRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewSubscriptionPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewSubscriptionPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: longviewSubscriptionID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetLongviewSubscription(t.Context(), longviewSubscriptionID)
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

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

const nodeBalancerTypesPath = "/nodebalancers/types"

func TestClientListNodeBalancerTypesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerTypesPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:      "common",
				keyLabel:   "Common",
				keyPrice:   map[string]float64{keyHourly: 0.015, keyMonthly: 10},
				"transfer": 1000,
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerTypes(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != tcCommon {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, tcCommon)
	}

	if got.Data[0].Label != "Common" {
		t.Errorf("got.Data[0].Label = %v, want %v", got.Data[0].Label, "Common")
	}

	if math.Abs(got.Data[0].Price.Hourly-0.015) > 0.001 {
		t.Errorf("%s: expected %v +/- %v, got %v", "float value should be within epsilon", 0.015, 0.001, got.Data[0].Price.Hourly)
	}

	if math.Abs(got.Data[0].Price.Monthly-10.0) > 0.001 {
		t.Errorf("%s: expected %v +/- %v, got %v", "float value should be within epsilon", 10.0, 0.001, got.Data[0].Price.Monthly)
	}

	if got.Data[0].Transfer != 1000 {
		t.Errorf("got.Data[0].Transfer = %v, want %v", got.Data[0].Transfer, 1000)
	}
}

func TestClientListNodeBalancerTypesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerTypesPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerTypes(t.Context())
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

func TestClientListNodeBalancerTypesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerTypesPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: "common"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListNodeBalancerTypes(t.Context())
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

	if got.Data[0].ID != tcCommon {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, tcCommon)
	}
}

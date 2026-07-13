package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	regionAvailabilityPlanStandard = "g6-standard-1"
	regionAvailabilityHyphenRegion = "br-gru"
)

func singleRetryOpts() []linode.Option {
	return []linode.Option{
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(1 * time.Millisecond),
		linode.WithMaxDelay(1 * time.Millisecond),
	}
}

func TestClientGetRegionAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointRegionUSEastAvailability {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointRegionUSEastAvailability)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    availability,
			keyPage:    1,
			keyPages:   1,
			keyResults: len(availability),
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.GetRegionAvailabilityProto(t.Context(), managedServiceRegion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if result[0].GetRegion() != managedServiceRegion {
		t.Errorf("result[0].GetRegion() = %v, want %v", result[0].GetRegion(), managedServiceRegion)
	}

	if result[0].GetPlan() != regionAvailabilityPlanStandard {
		t.Errorf("result[0].GetPlan() = %v, want %v", result[0].GetPlan(), regionAvailabilityPlanStandard)
	}

	if !result[0].GetAvailable() {
		t.Error("result[0].GetAvailable() = false, want true")
	}
}

func TestClientGetRegionAvailabilityValidSlugWithHyphen(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/regions/br-gru/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/regions/br-gru/availability")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: regionAvailabilityHyphenRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.GetRegionAvailabilityProto(t.Context(), regionAvailabilityHyphenRegion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if result[0].GetRegion() != regionAvailabilityHyphenRegion {
		t.Errorf("result[0].GetRegion() = %v, want %v", result[0].GetRegion(), regionAvailabilityHyphenRegion)
	}
}

func TestClientGetRegionAvailabilityEscapesRegionID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/regions/us-east%2Fbad%3Fx=1/availability" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/regions/us-east%2Fbad%3Fx=1/availability")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []linode.RegionAvailability{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetRegionAvailabilityProto(t.Context(), "us-east/bad?x=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetRegionAvailabilityRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)

			return
		}

		if r.URL.Path != endpointRegionUSEastAvailability {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointRegionUSEastAvailability)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, singleRetryOpts()...)

	result, err := client.GetRegionAvailabilityProto(t.Context(), managedServiceRegion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if attempts != int32(2) {
		t.Errorf("attempts = %v, want %v", attempts, int32(2))
	}
}

func TestClientGetRegionAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointRegionUSEastAvailability {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointRegionUSEastAvailability)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetRegionAvailabilityProto(t.Context(), managedServiceRegion)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

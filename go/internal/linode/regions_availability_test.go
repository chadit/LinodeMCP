package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
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

func TestClientListRegionsAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := []linode.RegionAvailability{
		{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true},
		{Region: "us-west", Plan: "g1-gpu-rtx6000-1", Available: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    availability,
			keyPage:    1,
			keyPages:   1,
			keyResults: len(availability),
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListRegionsAvailability(t.Context())

	stdMustNoError(t, err, "ListRegionsAvailability should succeed on 200 response")
	stdMustLen(t, result, 2, "should return all availability entries")
	stdCheckEqual(t, managedServiceRegion, result[0].Region)
	stdCheckEqual(t, regionAvailabilityPlanStandard, result[0].Plan)
	stdCheckTrue(t, result[0].Available)
	stdCheckFalse(t, result[1].Available)
}

func TestClientListRegionsAvailabilityRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)

			return
		}

		stdCheckEqual(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, singleRetryOpts()...)
	result, err := client.ListRegionsAvailability(t.Context())

	stdMustNoError(t, err, "read-only availability list should succeed after retry")
	stdMustLen(t, result, 1, "should return availability entry after retry")
	stdCheckEqual(t, int32(2), attempts, "should retry once after transient failure")
}

func TestClientListRegionsAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
		w.WriteHeader(http.StatusForbidden)
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListRegionsAvailability(t.Context())

	stdMustError(t, err, "ListRegionsAvailability should fail on 403 response")

	apiErr := stdAPIError(t, err, "error should be APIError")

	stdCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetRegionAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    availability,
			keyPage:    1,
			keyPages:   1,
			keyResults: len(availability),
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetRegionAvailability(t.Context(), managedServiceRegion)

	stdMustNoError(t, err, "GetRegionAvailability should succeed on 200 response")
	stdMustLen(t, result, 1, "should return all availability entries for the region")
	stdCheckEqual(t, managedServiceRegion, result[0].Region)
	stdCheckEqual(t, regionAvailabilityPlanStandard, result[0].Plan)
	stdCheckTrue(t, result[0].Available)
}

func TestClientGetRegionAvailabilityValidSlugWithHyphen(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/regions/br-gru/availability", r.URL.Path, "valid region slug should not be double encoded")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: regionAvailabilityHyphenRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetRegionAvailability(t.Context(), regionAvailabilityHyphenRegion)

	stdMustNoError(t, err, "GetRegionAvailability should accept a valid hyphenated region slug")
	stdMustLen(t, result, 1, "should return availability entry for the region")
	stdCheckEqual(t, regionAvailabilityHyphenRegion, result[0].Region)
}

func TestClientGetRegionAvailabilityEscapesRegionID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/regions/us-east%2Fbad%3Fx=1/availability", r.URL.EscapedPath(), "request path should escape the region ID")
		stdCheckEmpty(t, r.URL.RawQuery, "escaped path parameter should not become query parameters")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.RegionAvailability{}}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetRegionAvailability(t.Context(), "us-east/bad?x=1")

	stdMustNoError(t, err, "GetRegionAvailability should escape path parameters")
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

		stdCheckEqual(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, singleRetryOpts()...)
	result, err := client.GetRegionAvailability(t.Context(), managedServiceRegion)

	stdMustNoError(t, err, "read-only availability lookup should succeed after retry")
	stdMustLen(t, result, 1, "should return availability entry after retry")
	stdCheckEqual(t, int32(2), attempts, "should retry once after transient failure")
}

func TestClientGetRegionAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
		w.WriteHeader(http.StatusForbidden)
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetRegionAvailability(t.Context(), managedServiceRegion)

	stdMustError(t, err, "GetRegionAvailability should fail on 403 response")

	apiErr := stdAPIError(t, err, "error should be APIError")

	stdCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

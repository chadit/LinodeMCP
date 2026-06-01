package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const regionAvailabilityPlanStandard = "g6-standard-1"

func TestClientListRegionsAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := []linode.RegionAvailability{
		{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true},
		{Region: "us-west", Plan: "g1-gpu-rtx6000-1", Available: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    availability,
			keyPage:    1,
			keyPages:   1,
			keyResults: len(availability),
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListRegionsAvailability(t.Context())

	require.NoError(t, err, "ListRegionsAvailability should succeed on 200 response")
	require.Len(t, result, 2, "should return all availability entries")
	assert.Equal(t, managedServiceRegion, result[0].Region)
	assert.Equal(t, regionAvailabilityPlanStandard, result[0].Plan)
	assert.True(t, result[0].Available)
	assert.False(t, result[1].Available)
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

		assert.Equal(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, fastRetryOpts()...)
	result, err := client.ListRegionsAvailability(t.Context())

	require.NoError(t, err, "read-only availability list should succeed after retry")
	require.Len(t, result, 1, "should return availability entry after retry")
	assert.Equal(t, int32(2), attempts, "should retry once after transient failure")
}

func TestClientListRegionsAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListRegionsAvailability(t.Context())

	require.Error(t, err, "ListRegionsAvailability should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetRegionAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    availability,
			keyPage:    1,
			keyPages:   1,
			keyResults: len(availability),
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetRegionAvailability(t.Context(), managedServiceRegion)

	require.NoError(t, err, "GetRegionAvailability should succeed on 200 response")
	require.Len(t, result, 1, "should return all availability entries for the region")
	assert.Equal(t, managedServiceRegion, result[0].Region)
	assert.Equal(t, regionAvailabilityPlanStandard, result[0].Plan)
	assert.True(t, result[0].Available)
}

func TestClientGetRegionAvailabilityEscapesRegionID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/regions/us-east%2Fbad%3Fx=1/availability", r.URL.EscapedPath(), "request path should escape the region ID")
		assert.Empty(t, r.URL.RawQuery, "escaped path parameter should not become query parameters")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.RegionAvailability{}}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetRegionAvailability(t.Context(), "us-east/bad?x=1")

	require.NoError(t, err, "GetRegionAvailability should escape path parameters")
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

		assert.Equal(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.RegionAvailability{{Region: managedServiceRegion, Plan: regionAvailabilityPlanStandard, Available: true}},
		}), "encoding availability response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, fastRetryOpts()...)
	result, err := client.GetRegionAvailability(t.Context(), managedServiceRegion)

	require.NoError(t, err, "read-only availability lookup should succeed after retry")
	require.Len(t, result, 1, "should return availability entry after retry")
	assert.Equal(t, int32(2), attempts, "should retry once after transient failure")
}

func TestClientGetRegionAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetRegionAvailability(t.Context(), managedServiceRegion)

	require.Error(t, err, "GetRegionAvailability should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

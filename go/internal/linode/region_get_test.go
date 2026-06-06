package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const linodeRegionLabelNewark = "Newark, NJ"

func TestClientGetRegionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, "/regions/us-east", r.URL.EscapedPath(), "request path should match the documented route")
		stdCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.Region{ID: regionUSEast, Label: linodeRegionLabelNewark, Country: "us", Status: managedServiceStatus}), "encoding region response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	region, err := client.GetRegion(t.Context(), regionUSEast)

	stdMustNoError(t, err, "GetRegion should succeed on 200 response")
	stdMustNotNil(t, region, "region should not be nil")
	stdCheckEqual(t, regionUSEast, region.ID)
	stdCheckEqual(t, linodeRegionLabelNewark, region.Label)
}

func TestClientGetRegionEscapesPathParameter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/regions/us-east%2Fbad%3Fquery", r.URL.EscapedPath(), "region ID path segment should be escaped")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.Region{ID: "us-east/bad?query"}), "encoding region response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	region, err := client.GetRegion(t.Context(), "us-east/bad?query")

	stdMustNoError(t, err, "GetRegion should escape path parameters")
	stdMustNotNil(t, region, "region should not be nil")
}

func TestClientGetRegionRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current == 1 {
			http.Error(w, errTemporaryFailure, http.StatusBadGateway)

			return
		}

		stdCheckEqual(t, "/regions/us-east", r.URL.Path, "request path should match the documented route")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(linode.Region{ID: regionUSEast}), "encoding region response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, fastRetryOpts()...)
	region, err := client.GetRegion(t.Context(), regionUSEast)

	stdMustNoError(t, err, "read-only region get should succeed after retry")
	stdMustNotNil(t, region, "region should not be nil")
	stdCheckEqual(t, int32(2), attempts, "should retry once after transient failure")
}

func TestClientGetRegionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/regions/us-east", r.URL.Path, "request path should match the documented route")
		w.WriteHeader(http.StatusNotFound)
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errNotFound}}}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetRegion(t.Context(), regionUSEast)

	stdMustError(t, err, "GetRegion should fail on 404 response")

	apiErr := stdAPIError(t, err, "error should be APIError")

	stdCheckEqual(t, http.StatusNotFound, apiErr.StatusCode)
}

package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const linodeRegionLabelNewark = "Newark, NJ"

func TestClientGetRegionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != endpointRegionUSEast {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointRegionUSEast)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Region{ID: regionUSEast, Label: linodeRegionLabelNewark, Country: "us", Status: managedServiceStatus}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	region, err := client.GetRegion(t.Context(), regionUSEast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if region == nil {
		t.Fatal("region is nil")
	}

	if region.ID != regionUSEast {
		t.Errorf("region.ID = %v, want %v", region.ID, regionUSEast)
	}

	if region.Label != linodeRegionLabelNewark {
		t.Errorf("region.Label = %v, want %v", region.Label, linodeRegionLabelNewark)
	}
}

func TestClientGetRegionEscapesPathParameter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/regions/us-east%2Fbad%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/regions/us-east%2Fbad%3Fquery")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Region{ID: "us-east/bad?query"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	region, err := client.GetRegion(t.Context(), "us-east/bad?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if region == nil {
		t.Fatal("region is nil")
	}
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

		if r.URL.Path != endpointRegionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointRegionUSEast)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Region{ID: regionUSEast}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, fastRetryOpts()...)

	region, err := client.GetRegion(t.Context(), regionUSEast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if region == nil {
		t.Fatal("region is nil")
	}

	if attempts != int32(2) {
		t.Errorf("attempts = %v, want %v", attempts, int32(2))
	}
}

func TestClientGetRegionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointRegionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointRegionUSEast)
		}

		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errNotFound}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetRegion(t.Context(), regionUSEast)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusNotFound)
	}
}

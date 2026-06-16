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

const endpointNetworkTransferPrices = "/network-transfer/prices"

func TestClientListNetworkTransferPricesSuccess(t *testing.T) {
	t.Parallel()

	prices := linode.PaginatedResponse[linode.NetworkTransferPrice]{
		Data: []linode.NetworkTransferPrice{{
			ID:       networkTransferPriceID,
			Label:    "Network Transfer",
			Price:    linode.Price{Hourly: 0.005},
			Transfer: 0,
			RegionPrices: []linode.NetworkTransferRegionPrice{{
				ID:      managedServiceRegion,
				Hourly:  0.015,
				Monthly: 0,
			}},
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkTransferPrices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkTransferPrices)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(prices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListNetworkTransferPrices(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != networkTransferPriceID {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, networkTransferPriceID)
	}

	if result.Data[0].Label != "Network Transfer" {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, "Network Transfer")
	}

	if math.Abs(result.Data[0].Price.Hourly-0.005)/math.Abs(0.005) > 0.0001 {
		t.Errorf("result.Data[0].Price.Hourly = %v, want %v", result.Data[0].Price.Hourly, 0.005)
	}

	if len(result.Data[0].RegionPrices) != 1 {
		t.Fatalf("len(result.Data[0].RegionPrices) = %d, want %d", len(result.Data[0].RegionPrices), 1)
	}

	if result.Data[0].RegionPrices[0].ID != managedServiceRegion {
		t.Errorf("result.Data[0].RegionPrices[0].ID = %v, want %v", result.Data[0].RegionPrices[0].ID, managedServiceRegion)
	}
}

func TestClientListNetworkTransferPricesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkTransferPrices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkTransferPrices)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListNetworkTransferPrices(t.Context())
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

func TestClientListNetworkTransferPricesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkTransferPrices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkTransferPrices)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.NetworkTransferPrice]{
			Data: []linode.NetworkTransferPrice{{ID: networkTransferPriceID}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListNetworkTransferPrices(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != networkTransferPriceID {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, networkTransferPriceID)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

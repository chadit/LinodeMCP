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

// writeNodeBalancerStatsFixture writes the real GET /nodebalancers/{id}/stats
// body, which nests the graphs under a top-level "data" object beside "title".
func writeNodeBalancerStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", tcApplicationJSON)

	_, err := w.Write([]byte(`{
		"title":"nodebalancer.example.com (nodebalancer123) - day (5 min avg)",
		"data":{
			"connections":[[1521483600000,12.5]],
			"traffic":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]]}
		}
	}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClientGetNodeBalancerStatsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers444Stats {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers444Stats)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		writeNodeBalancerStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerStatsProto(t.Context(), 444)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.GetTitle() != "nodebalancer.example.com (nodebalancer123) - day (5 min avg)" {
		t.Errorf("got.GetTitle() = %v, want %v", got.GetTitle(), "nodebalancer.example.com (nodebalancer123) - day (5 min avg)")
	}

	if v := got.GetData().GetConnections()[0].GetValues()[1].GetNumberValue(); math.Abs(v-12.5) > 0.001 {
		t.Errorf("connections value = %v, want %v", v, 12.5)
	}

	if v := got.GetData().GetTraffic().GetIn()[0].GetValues()[1].GetNumberValue(); math.Abs(v-2004.36) > 0.001 {
		t.Errorf("traffic.in value = %v, want %v", v, 2004.36)
	}

	if v := got.GetData().GetTraffic().GetOut()[0].GetValues()[1].GetNumberValue(); math.Abs(v-3928.91) > 0.001 {
		t.Errorf("traffic.out value = %v, want %v", v, 3928.91)
	}
}

func TestClientGetNodeBalancerStatsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers444Stats {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers444Stats)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNodeBalancerStatsProto(t.Context(), 444)
	if err == nil {
		t.Fatal("expected an error, got nil")
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

func TestClientGetNodeBalancerStatsRejectsInvalidPathParam(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNodeBalancerStatsProto(t.Context(), 0)
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	_, err = client.GetNodeBalancerStatsProto(t.Context(), -1)
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if called.Load() != false {
		t.Errorf("called.Load() = %v, want %v", called.Load(), false)
	}
}

func TestClientGetNodeBalancerStatsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers444Stats {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers444Stats)
		}

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		writeNodeBalancerStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetNodeBalancerStatsProto(t.Context(), 444)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}

	if got.GetTitle() != "nodebalancer.example.com (nodebalancer123) - day (5 min avg)" {
		t.Errorf("got.GetTitle() = %v, want %v", got.GetTitle(), "nodebalancer.example.com (nodebalancer123) - day (5 min avg)")
	}
}

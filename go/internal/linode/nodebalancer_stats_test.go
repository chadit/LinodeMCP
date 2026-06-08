package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func writeNodeBalancerStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", tcApplicationJSON)

	_, err := w.Write([]byte(`{
		"title":"nodebalancer.example.com (nodebalancer123) - day (5 min avg)",
		"connections":[[1521483600000,12.5]],
		"traffic":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]]}
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

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		writeNodeBalancerStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerStats(t.Context(), 444)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Title != "nodebalancer.example.com (nodebalancer123) - day (5 min avg)" {
		t.Errorf("got.Title = %v, want %v", got.Title, "nodebalancer.example.com (nodebalancer123) - day (5 min avg)")
	}

	if !reflect.DeepEqual(got.Connections, [][]float64{{1521483600000, 12.5}}) {
		t.Errorf("got.Connections = %v, want %v", got.Connections, [][]float64{{1521483600000, 12.5}})
	}

	if !reflect.DeepEqual(got.Traffic.In, [][]float64{{1521484800000, 2004.36}}) {
		t.Errorf("got.Traffic.In = %v, want %v", got.Traffic.In, [][]float64{{1521484800000, 2004.36}})
	}

	if !reflect.DeepEqual(got.Traffic.Out, [][]float64{{1521484800000, 3928.91}}) {
		t.Errorf("got.Traffic.Out = %v, want %v", got.Traffic.Out, [][]float64{{1521484800000, 3928.91}})
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

	_, err := client.GetNodeBalancerStats(t.Context(), 444)
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

	_, err := client.GetNodeBalancerStats(t.Context(), 0)
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	_, err = client.GetNodeBalancerStats(t.Context(), -1)
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

	got, err := client.GetNodeBalancerStats(t.Context(), 444)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}

	if got.Title != "nodebalancer.example.com (nodebalancer123) - day (5 min avg)" {
		t.Errorf("got.Title = %v, want %v", got.Title, "nodebalancer.example.com (nodebalancer123) - day (5 min avg)")
	}
}

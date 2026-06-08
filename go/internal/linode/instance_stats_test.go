package linode_test

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func writeInstanceStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", tcApplicationJSON)

	_, err := w.Write([]byte(`{
		"title":"linode.com - my-linode (linode123456) - day (5 min avg)",
		"cpu":[[1521483600000,0.42]],
		"io":{"io":[[1521484800000,0.19]],"swap":[[1521484800000,0]]},
		"netv4":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]],"private_in":[[1521484800000,0]],"private_out":[[1521484800000,5.6]]},
		"netv6":{"in":[[1521484800000,0]],"out":[[1521484800000,0]],"private_in":[[1521484800000,195.18]],"private_out":[[1521484800000,5.6]]}
	}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClientGetInstanceStatsSuccess(t *testing.T) {
	t.Parallel()

	want := linode.InstanceStats{
		Title: instanceStatsTitle,
		CPU:   [][]float64{{1521483600000, 0.42}},
		IO: linode.InstanceIOStats{
			IO:   [][]float64{{1521484800000, 0.19}},
			Swap: [][]float64{{1521484800000, 0}},
		},
		NetV4: linode.InstanceNetV4Stats{
			In:         [][]float64{{1521484800000, 2004.36}},
			Out:        [][]float64{{1521484800000, 3928.91}},
			PrivateIn:  [][]float64{{1521484800000, 0}},
			PrivateOut: [][]float64{{1521484800000, 5.6}},
		},
		NetV6: linode.InstanceNetV6Stats{
			In:         [][]float64{{1521484800000, 0}},
			Out:        [][]float64{{1521484800000, 0}},
			PrivateIn:  [][]float64{{1521484800000, 195.18}},
			PrivateOut: [][]float64{{1521484800000, 5.6}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceStatsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceStatsPath)
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

		writeInstanceStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetInstanceStats(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Title != want.Title {
		t.Errorf("got.Title = %v, want %v", got.Title, want.Title)
	}

	if !reflect.DeepEqual(got.CPU, want.CPU) {
		t.Errorf("got.CPU = %v, want %v", got.CPU, want.CPU)
	}

	if !reflect.DeepEqual(got.IO, want.IO) {
		t.Errorf("got.IO = %v, want %v", got.IO, want.IO)
	}

	if !reflect.DeepEqual(got.NetV4, want.NetV4) {
		t.Errorf("got.NetV4 = %v, want %v", got.NetV4, want.NetV4)
	}

	if !reflect.DeepEqual(got.NetV6, want.NetV6) {
		t.Errorf("got.NetV6 = %v, want %v", got.NetV6, want.NetV6)
	}
}

func TestClientGetInstanceStatsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceStatsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceStatsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceStats(t.Context(), 123)
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

func TestClientGetInstanceStatsRetriesTransientError(t *testing.T) {
	t.Parallel()

	want := linode.InstanceStats{Title: instanceStatsTitle}

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceStatsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceStatsPath)
		}

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		writeInstanceStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetInstanceStats(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Title != want.Title {
		t.Errorf("got.Title = %v, want %v", got.Title, want.Title)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetInstanceStatsByYearMonthSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceStatsYearMonth {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceStatsYearMonth)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"cpu": [][]float64{{1521483600000, 0.42}},
			"io": map[string]any{
				"io":   [][]float64{{1521484800000, 0.19}},
				"swap": [][]float64{{1521484800000, 0}},
			},
			"netv4": map[string]any{
				"in":          [][]float64{{1521484800000, 2004.36}},
				"out":         [][]float64{{1521484800000, 3928.91}},
				"private_in":  [][]float64{{1521484800000, 0}},
				"private_out": [][]float64{{1521484800000, 5.6}},
			},
			"netv6": map[string]any{
				"in":          [][]float64{{1521484800000, 10}},
				"out":         [][]float64{{1521484800000, 20}},
				"private_in":  [][]float64{{1521484800000, 0}},
				"private_out": [][]float64{{1521484800000, 0}},
			},
			"title": "linode123 stats",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetInstanceStatsByYearMonth(t.Context(), 123, 2024, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Title != "linode123 stats" {
		t.Errorf("got.Title = %v, want %v", got.Title, "linode123 stats")
	}

	if math.Abs(got.CPU[0][1]-0.42) > 0.001 {
		t.Errorf("got.CPU[0][1] = %v, want %v", got.CPU[0][1], 0.42)
	}

	if math.Abs(got.IO.IO[0][1]-0.19) > 0.001 {
		t.Errorf("got.IO.IO[0][1] = %v, want %v", got.IO.IO[0][1], 0.19)
	}

	if math.Abs(got.NetV4.In[0][1]-2004.36) > 0.001 {
		t.Errorf("got.NetV4.In[0][1] = %v, want %v", got.NetV4.In[0][1], 2004.36)
	}

	if math.Abs(got.NetV6.Out[0][1]-20.0) > 0.001 {
		t.Errorf("got.NetV6.Out[0][1] = %v, want %v", got.NetV6.Out[0][1], 20.0)
	}
}

func TestClientGetInstanceStatsByYearMonthAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointInstanceStatsYearMonth {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceStatsYearMonth)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceStatsByYearMonth(t.Context(), 123, 2024, 8)
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientGetInstanceStatsByYearMonthRejectsInvalidPathParams(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceStatsByYearMonth(t.Context(), 0, 2024, 8)
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.GetInstanceStatsByYearMonth(t.Context(), 123, 1999, 8)
	if !errors.Is(err, linode.ErrStatsYearRange) {
		t.Fatalf("error = %v, want %v", err, linode.ErrStatsYearRange)
	}

	_, err = client.GetInstanceStatsByYearMonth(t.Context(), 123, 2024, 13)
	if !errors.Is(err, linode.ErrStatsMonthRange) {
		t.Fatalf("error = %v, want %v", err, linode.ErrStatsMonthRange)
	}

	if called.Load() {
		t.Error("called.Load() = true, want false")
	}
}

func TestClientGetInstanceStatsByYearMonthRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceStatsYearMonth {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceStatsYearMonth)
		}

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		writeInstanceStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	got, err := client.GetInstanceStatsByYearMonth(t.Context(), 123, 2024, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Title != instanceStatsTitle {
		t.Errorf("got.Title = %v, want %v", got.Title, instanceStatsTitle)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	monitorServicesPath                        = "/monitor/services"
	monitorServiceTypePath                     = "/monitor/services/dbaas"
	monitorServiceMetricDefinitionsPath        = "/monitor/services/dbaas/metric-definitions"
	monitorServiceAlertDefinitionsPath         = "/monitor/services/dbaas/alert-definitions"
	monitorServiceMetricsPath                  = "/monitor/services/dbaas/metrics"
	monitorServiceEscapedTypePath              = "/monitor/services/dbaas%2Fpostgres"
	monitorServiceEscapedMetricDefinitionsPath = "/monitor/services/dbaas%2Fpostgres/metric-definitions"
	monitorServiceEscapedAlertDefinitionsPath  = "/monitor/services/dbaas%2Fpostgres/alert-definitions"
	monitorServiceEscapedMetricsPath           = "/monitor/services/dbaas%2Fpostgres/metrics"
	monitorServiceLabel                        = "Databases"
	monitorMetricDefinitionLabel               = "CPU Usage"
	monitorMetricDefinitionMetric              = "cpu_usage"
	monitorMetricDefinitionType                = "gauge"
	monitorServiceTypeDatabase                 = "dbaas"
	monitorServiceTypeWithSlash                = "dbaas/postgres"
)

func TestClientListMonitorServicesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServicesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServicesPath)
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
				keyLabel:       monitorServiceLabel,
				keyServiceType: monitorServiceTypeDatabase,
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

	got, err := client.ListMonitorServicesProto(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].GetLabel() != monitorServiceLabel {
		t.Errorf("got[0].GetLabel() = %v, want %v", got[0].GetLabel(), monitorServiceLabel)
	}

	if got[0].GetServiceType() != monitorServiceTypeDatabase {
		t.Errorf("got[0].GetServiceType() = %v, want %v", got[0].GetServiceType(), monitorServiceTypeDatabase)
	}
}

func TestClientListMonitorServicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServicesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServicesPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorServicesProto(t.Context())
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

func TestClientListMonitorServicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServicesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServicesPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:       monitorServiceLabel,
				keyServiceType: monitorServiceTypeDatabase,
			}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListMonitorServicesProto(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].GetServiceType() != monitorServiceTypeDatabase {
		t.Errorf("got[0].GetServiceType() = %v, want %v", got[0].GetServiceType(), monitorServiceTypeDatabase)
	}
}

func TestClientGetMonitorServiceMetricsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceMetricsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceMetricsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var gotJSON any
		if err := json.Unmarshal(body, &gotJSON); err != nil {
			t.Fatalf("request body is not valid JSON: %v", err)
		}

		if !reflect.DeepEqual(gotJSON, map[string]any{}) {
			t.Errorf("request body = %s, want an empty JSON object", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"cpu": []float64{1.5}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if _, ok := got["cpu"]; !ok {
		t.Errorf("got missing key %v", "cpu")
	}
}

func TestClientGetMonitorServiceMetricsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != monitorServiceEscapedMetricsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), monitorServiceEscapedMetricsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"service_type": monitorServiceTypeWithSlash}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeWithSlash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !reflect.DeepEqual(got["service_type"], monitorServiceTypeWithSlash) {
		t.Errorf("got %v, want %v", got["service_type"], monitorServiceTypeWithSlash)
	}
}

func TestClientGetMonitorServiceMetricsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceMetricsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceMetricsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)
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

func TestClientGetMonitorServiceMetricsDoesNotReplayTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != monitorServiceMetricsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceMetricsPath)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

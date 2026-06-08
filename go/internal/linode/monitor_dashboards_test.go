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

const (
	monitorDashboardsPath  = "/monitor/dashboards"
	monitorDashboardPath   = "/monitor/dashboards/30000"
	monitorDashboardsQuery = "page=2&page_size=25"
	monitorDashboardID     = 30000
	monitorDashboardLabel  = "Resource Usage"
	monitorDashboardType   = "dashboard"
	monitorDashboardWidget = "cpu"
)

func TestClientListMonitorDashboardsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorDashboardsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorDashboardsPath)
		}

		if r.URL.RawQuery != monitorDashboardsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, monitorDashboardsQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:      monitorDashboardID,
				keyLabel:   monitorDashboardLabel,
				keyType:    monitorDashboardType,
				keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorDashboards(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Page != 2 {
		t.Errorf("got.Page = %v, want %v", got.Page, 2)
	}

	if got.Pages != 3 {
		t.Errorf("got.Pages = %v, want %v", got.Pages, 3)
	}

	if got.Results != 1 {
		t.Errorf("got.Results = %v, want %v", got.Results, 1)
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if numVal, numOK := got.Data[0][keyID].(float64); !numOK || math.Abs(numVal-float64(monitorDashboardID)) > math.Abs(float64(monitorDashboardID))*0.001 {
		t.Errorf("got.Data[0][keyID] = %v, want ~%v", got.Data[0][keyID], monitorDashboardID)
	}

	if !reflect.DeepEqual(got.Data[0][keyLabel], monitorDashboardLabel) {
		t.Errorf("got.Data[0][keyLabel] = %v, want %v", got.Data[0][keyLabel], monitorDashboardLabel)
	}

	if !reflect.DeepEqual(got.Data[0][keyType], monitorDashboardType) {
		t.Errorf("got.Data[0][keyType] = %v, want %v", got.Data[0][keyType], monitorDashboardType)
	}
}

func TestClientGetMonitorDashboardSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorDashboardPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorDashboardPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:      monitorDashboardID,
			keyLabel:   monitorDashboardLabel,
			keyType:    monitorDashboardType,
			keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if numVal, numOK := got[keyID].(float64); !numOK || math.Abs(numVal-float64(monitorDashboardID)) > math.Abs(float64(monitorDashboardID))*0.001 {
		t.Errorf("got[keyID] = %v, want ~%v", got[keyID], monitorDashboardID)
	}

	if !reflect.DeepEqual(got[keyLabel], monitorDashboardLabel) {
		t.Errorf("got[keyLabel] = %v, want %v", got[keyLabel], monitorDashboardLabel)
	}

	if !reflect.DeepEqual(got[keyType], monitorDashboardType) {
		t.Errorf("got[keyType] = %v, want %v", got[keyType], monitorDashboardType)
	}
}

func TestClientGetMonitorDashboardAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorDashboardPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorDashboardPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)
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

func TestClientGetMonitorDashboardRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorDashboardPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorDashboardPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: monitorDashboardID, keyLabel: monitorDashboardLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if numVal, numOK := got[keyID].(float64); !numOK || math.Abs(numVal-float64(monitorDashboardID)) > math.Abs(float64(monitorDashboardID))*0.001 {
		t.Errorf("got[keyID] = %v, want ~%v", got[keyID], monitorDashboardID)
	}
}

func TestClientListMonitorDashboardsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorDashboardsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorDashboardsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorDashboards(t.Context(), 0, 0)
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

func TestClientListMonitorDashboardsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorDashboardsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorDashboardsPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorDashboardID, keyLabel: monitorDashboardLabel}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListMonitorDashboards(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if numVal, numOK := got.Data[0][keyID].(float64); !numOK || math.Abs(numVal-float64(monitorDashboardID)) > math.Abs(float64(monitorDashboardID))*0.001 {
		t.Errorf("got.Data[0][keyID] = %v, want ~%v", got.Data[0][keyID], monitorDashboardID)
	}
}

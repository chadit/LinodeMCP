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
	monitorServiceDashboardsPath        = "/monitor/services/dbaas/dashboards"
	monitorServiceEscapedDashboardsPath = "/monitor/services/dbaas%2Fpostgres/dashboards"
)

func TestClientListMonitorServiceDashboardsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceDashboardsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceDashboardsPath)
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
				keyID:      monitorDashboardID,
				keyLabel:   monitorDashboardLabel,
				keyType:    monitorDashboardType,
				keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
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

	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeDatabase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Page != 1 {
		t.Errorf("got.Page = %v, want %v", got.Page, 1)
	}

	if got.Pages != 1 {
		t.Errorf("got.Pages = %v, want %v", got.Pages, 1)
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

func TestClientListMonitorServiceDashboardsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != monitorServiceEscapedDashboardsPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), monitorServiceEscapedDashboardsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyLabel: monitorDashboardLabel}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeWithSlash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if !reflect.DeepEqual(got.Data[0][keyLabel], monitorDashboardLabel) {
		t.Errorf("got.Data[0][keyLabel] = %v, want %v", got.Data[0][keyLabel], monitorDashboardLabel)
	}
}

func TestClientListMonitorServiceDashboardsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceDashboardsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceDashboardsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeDatabase)
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

func TestClientListMonitorServiceDashboardsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceDashboardsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceDashboardsPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: monitorDashboardID}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeDatabase)
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

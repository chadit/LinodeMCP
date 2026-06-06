package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorDashboardsPath, r.URL.Path, "request path should match")
		monitorCheckEqual(t, monitorDashboardsQuery, r.URL.RawQuery, "request query should include pagination")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:      monitorDashboardID,
				keyLabel:   monitorDashboardLabel,
				keyType:    monitorDashboardType,
				keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorDashboards(t.Context(), 2, 25)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 2, got.Page)
	monitorCheckEqual(t, 3, got.Pages)
	monitorCheckEqual(t, 1, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckNumericClose(t, monitorDashboardID, got.Data[0][keyID])
	monitorCheckEqual(t, monitorDashboardLabel, got.Data[0][keyLabel])
	monitorCheckEqual(t, monitorDashboardType, got.Data[0][keyType])
}

func TestClientGetMonitorDashboardSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorDashboardPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:      monitorDashboardID,
			keyLabel:   monitorDashboardLabel,
			keyType:    monitorDashboardType,
			keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckNumericClose(t, monitorDashboardID, got[keyID])
	monitorCheckEqual(t, monitorDashboardLabel, got[keyLabel])
	monitorCheckEqual(t, monitorDashboardType, got[keyType])
}

func TestClientGetMonitorDashboardAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorDashboardPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorDashboardRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorDashboardPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: monitorDashboardID, keyLabel: monitorDashboardLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorCheckNumericClose(t, monitorDashboardID, got[keyID])
}

func TestClientListMonitorDashboardsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorDashboardsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorDashboards(t.Context(), 0, 0)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorDashboardsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorDashboardsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorDashboardID, keyLabel: monitorDashboardLabel}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorDashboards(t.Context(), 0, 0)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckNumericClose(t, monitorDashboardID, got.Data[0][keyID])
}

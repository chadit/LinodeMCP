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
	monitorServiceDashboardsPath        = "/monitor/services/dbaas/dashboards"
	monitorServiceEscapedDashboardsPath = "/monitor/services/dbaas%2Fpostgres/dashboards"
)

func TestClientListMonitorServiceDashboardsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceDashboardsPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:      monitorDashboardID,
				keyLabel:   monitorDashboardLabel,
				keyType:    monitorDashboardType,
				keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 1, got.Page)
	monitorCheckEqual(t, 1, got.Pages)
	monitorCheckEqual(t, 1, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckNumericClose(t, monitorDashboardID, got.Data[0][keyID])
	monitorCheckEqual(t, monitorDashboardLabel, got.Data[0][keyLabel])
	monitorCheckEqual(t, monitorDashboardType, got.Data[0][keyType])
}

func TestClientListMonitorServiceDashboardsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceEscapedDashboardsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyLabel: monitorDashboardLabel}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeWithSlash)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorDashboardLabel, got.Data[0][keyLabel])
}

func TestClientListMonitorServiceDashboardsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceDashboardsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeDatabase)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServiceDashboardsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceDashboardsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: monitorDashboardID}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServiceDashboards(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckNumericClose(t, monitorDashboardID, got.Data[0][keyID])
}

package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorDashboardsPath, r.URL.Path, "request path should match")
		assert.Equal(t, monitorDashboardsQuery, r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 2, got.Page)
	assert.Equal(t, 3, got.Pages)
	assert.Equal(t, 1, got.Results)
	require.Len(t, got.Data, 1)
	assert.InEpsilon(t, monitorDashboardID, got.Data[0][keyID], 0.001)
	assert.Equal(t, monitorDashboardLabel, got.Data[0][keyLabel])
	assert.Equal(t, monitorDashboardType, got.Data[0][keyType])
}

func TestClientGetMonitorDashboardSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorDashboardPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:      monitorDashboardID,
			keyLabel:   monitorDashboardLabel,
			keyType:    monitorDashboardType,
			keyWidgets: []map[string]any{{keyLabel: monitorDashboardWidget}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.InEpsilon(t, monitorDashboardID, got[keyID], 0.001)
	assert.Equal(t, monitorDashboardLabel, got[keyLabel])
	assert.Equal(t, monitorDashboardType, got[keyType])
}

func TestClientGetMonitorDashboardAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorDashboardPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorDashboardRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorDashboardPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: monitorDashboardID, keyLabel: monitorDashboardLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorDashboard(t.Context(), monitorDashboardID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	assert.InEpsilon(t, monitorDashboardID, got[keyID], 0.001)
}

func TestClientListMonitorDashboardsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorDashboardsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorDashboards(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorDashboardsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorDashboardsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorDashboardID, keyLabel: monitorDashboardLabel}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorDashboards(t.Context(), 0, 0)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.InEpsilon(t, monitorDashboardID, got.Data[0][keyID], 0.001)
}

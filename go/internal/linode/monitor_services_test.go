package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServicesPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:       monitorServiceLabel,
				keyServiceType: monitorServiceTypeDatabase,
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServices(t.Context())

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 1, got.Pages)
	assert.Equal(t, 1, got.Results)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorServiceLabel, got.Data[0].Label)
	assert.Equal(t, monitorServiceTypeDatabase, got.Data[0].ServiceType)
}

func TestClientGetMonitorServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceTypePath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	assert.Equal(t, monitorServiceLabel, got.Label)
	assert.Equal(t, monitorServiceTypeDatabase, got.ServiceType)
}

func TestClientGetMonitorServiceEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, monitorServiceEscapedTypePath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceLabel,
			keyServiceType: monitorServiceTypeWithSlash,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeWithSlash)

	require.NoError(t, err)
	assert.Equal(t, monitorServiceTypeWithSlash, got.ServiceType)
}

func TestClientGetMonitorServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceTypePath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeDatabase)

	require.Error(t, err)
	assert.Empty(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorServiceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceTypePath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	assert.Equal(t, monitorServiceTypeDatabase, got.ServiceType)
}

func TestClientListMonitorServiceMetricDefinitionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceMetricDefinitionsPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:      monitorMetricDefinitionLabel,
				keyMetric:     monitorMetricDefinitionMetric,
				keyMetricType: monitorMetricDefinitionType,
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 1, got.Pages)
	assert.Equal(t, 1, got.Results)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorMetricDefinitionLabel, got.Data[0].Label)
	assert.Equal(t, monitorMetricDefinitionMetric, got.Data[0].Metric)
	assert.Equal(t, monitorMetricDefinitionType, got.Data[0].MetricType)
}

func TestClientListMonitorServiceMetricDefinitionsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, monitorServiceEscapedMetricDefinitionsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyMetric: monitorMetricDefinitionMetric}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeWithSlash)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorMetricDefinitionMetric, got.Data[0].Metric)
}

func TestClientListMonitorServiceMetricDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceMetricDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeDatabase)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServiceMetricDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceMetricDefinitionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyMetric: monitorMetricDefinitionMetric}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorMetricDefinitionMetric, got.Data[0].Metric)
}

func TestClientListMonitorServiceAlertDefinitionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:          monitorAlertDefinitionID,
				keyLabel:       monitorAlertDefinitionLabel,
				keyServiceType: monitorServiceTypeDatabase,
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 1, got.Pages)
	assert.Equal(t, 1, got.Results)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorAlertDefinitionID, got.Data[0].ID)
	assert.Equal(t, monitorAlertDefinitionLabel, got.Data[0].Label)
	assert.Equal(t, monitorServiceTypeDatabase, got.Data[0].ServiceType)
}

func TestClientListMonitorServiceAlertDefinitionsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, monitorServiceEscapedAlertDefinitionsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyServiceType: monitorServiceTypeWithSlash}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeWithSlash)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorServiceTypeWithSlash, got.Data[0].ServiceType)
}

func TestClientListMonitorServiceAlertDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeDatabase)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServiceAlertDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: monitorAlertDefinitionID}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorAlertDefinitionID, got.Data[0].ID)
}

func TestClientListMonitorServicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServicesPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServices(t.Context())

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorServicesPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:       monitorServiceLabel,
				keyServiceType: monitorServiceTypeDatabase,
			}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServices(t.Context())

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorServiceTypeDatabase, got.Data[0].ServiceType)
}

func TestClientGetMonitorServiceMetricsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, monitorServiceMetricsPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{}`, string(body), "request body should be empty JSON object")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"cpu": []float64{1.5}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Contains(t, got, "cpu")
}

func TestClientGetMonitorServiceMetricsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, monitorServiceEscapedMetricsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"service_type": monitorServiceTypeWithSlash}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeWithSlash)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, monitorServiceTypeWithSlash, got["service_type"])
}

func TestClientGetMonitorServiceMetricsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, monitorServiceMetricsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorServiceMetricsDoesNotReplayTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, monitorServiceMetricsPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, int32(1), calls.Load(), "POST metrics route should not replay after transient failure")
}

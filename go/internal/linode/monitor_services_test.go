package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServicesPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 1, got.Page)
	monitorCheckEqual(t, 1, got.Pages)
	monitorCheckEqual(t, 1, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorServiceLabel, got.Data[0].Label)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.Data[0].ServiceType)
}

func TestClientGetMonitorServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceTypePath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorCheckEqual(t, monitorServiceLabel, got.Label)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.ServiceType)
}

func TestClientGetMonitorServiceEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceEscapedTypePath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceLabel,
			keyServiceType: monitorServiceTypeWithSlash,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeWithSlash)

	monitorRequireNoError(t, err)
	monitorCheckEqual(t, monitorServiceTypeWithSlash, got.ServiceType)
}

func TestClientGetMonitorServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceTypePath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeDatabase)

	monitorRequireError(t, err)
	monitorCheckEmpty(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorServiceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceTypePath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyLabel:       monitorServiceLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorService(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.ServiceType)
}

func TestClientListMonitorServiceMetricDefinitionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceMetricDefinitionsPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 1, got.Page)
	monitorCheckEqual(t, 1, got.Pages)
	monitorCheckEqual(t, 1, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorMetricDefinitionLabel, got.Data[0].Label)
	monitorCheckEqual(t, monitorMetricDefinitionMetric, got.Data[0].Metric)
	monitorCheckEqual(t, monitorMetricDefinitionType, got.Data[0].MetricType)
}

func TestClientListMonitorServiceMetricDefinitionsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceEscapedMetricDefinitionsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyMetric: monitorMetricDefinitionMetric}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeWithSlash)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorMetricDefinitionMetric, got.Data[0].Metric)
}

func TestClientListMonitorServiceMetricDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceMetricDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeDatabase)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServiceMetricDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceMetricDefinitionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyMetric: monitorMetricDefinitionMetric}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServiceMetricDefinitions(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorMetricDefinitionMetric, got.Data[0].Metric)
}

func TestClientListMonitorServiceAlertDefinitionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 1, got.Page)
	monitorCheckEqual(t, 1, got.Pages)
	monitorCheckEqual(t, 1, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.Data[0].ID)
	monitorCheckEqual(t, monitorAlertDefinitionLabel, got.Data[0].Label)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.Data[0].ServiceType)
}

func TestClientListMonitorServiceAlertDefinitionsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceEscapedAlertDefinitionsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyServiceType: monitorServiceTypeWithSlash}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeWithSlash)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorServiceTypeWithSlash, got.Data[0].ServiceType)
}

func TestClientListMonitorServiceAlertDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeDatabase)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServiceAlertDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: monitorAlertDefinitionID}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServiceAlertDefinitions(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.Data[0].ID)
}

func TestClientListMonitorServicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServicesPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorServices(t.Context())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorServicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServicesPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:       monitorServiceLabel,
				keyServiceType: monitorServiceTypeDatabase,
			}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorServices(t.Context())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.Data[0].ServiceType)
}

func TestClientGetMonitorServiceMetricsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceMetricsPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		monitorCheckNoError(t, err)
		monitorCheckJSONEqual(t, `{}`, string(body), "request body should be empty JSON object")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{"cpu": []float64{1.5}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckHasKey(t, got, "cpu")
}

func TestClientGetMonitorServiceMetricsEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceEscapedMetricsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{"service_type": monitorServiceTypeWithSlash}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeWithSlash)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, monitorServiceTypeWithSlash, got["service_type"])
}

func TestClientGetMonitorServiceMetricsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceMetricsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorServiceMetricsDoesNotReplayTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceMetricsPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorServiceMetrics(t.Context(), monitorServiceTypeDatabase)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)
	monitorCheckEqual(t, int32(1), calls.Load(), "POST metrics route should not replay after transient failure")
}

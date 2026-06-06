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
	monitorAlertDefinitionsPath       = "/monitor/alert-definitions"
	monitorAlertDefinitionsQuery      = "page=2&page_size=25"
	monitorAlertDefinitionID          = 20000
	monitorAlertDefinitionLabel       = "High CPU Usage"
	monitorAlertDefinitionType        = "alerts-definitions"
	monitorAlertDefinitionServiceType = "linode"
	monitorAlertDefinitionDescription = "CPU usage is high"
)

func TestClientListMonitorAlertDefinitionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorAlertDefinitionsPath, r.URL.Path, "request path should match")
		monitorCheckEqual(t, monitorAlertDefinitionsQuery, r.URL.RawQuery, "request query should include pagination")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:          monitorAlertDefinitionID,
				keyLabel:       monitorAlertDefinitionLabel,
				keyType:        monitorAlertDefinitionType,
				keyServiceType: monitorAlertDefinitionServiceType,
				"description":  monitorAlertDefinitionDescription,
				keySeverity:    2,
				"criteria":     map[string]any{keyThreshold: 90},
			}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorAlertDefinitions(t.Context(), 2, 25)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 1, got.Page)
	monitorCheckEqual(t, 1, got.Pages)
	monitorCheckEqual(t, 1, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.Data[0].ID)
	monitorCheckEqual(t, monitorAlertDefinitionLabel, got.Data[0].Label)
	monitorCheckEqual(t, monitorAlertDefinitionType, got.Data[0].Type)
	monitorCheckEqual(t, monitorAlertDefinitionServiceType, got.Data[0].ServiceType)
	monitorCheckEqual(t, monitorAlertDefinitionDescription, got.Data[0].Description)
	monitorCheckEqual(t, 2, got.Data[0].Severity)
	monitorCheckNumericClose(t, 90, got.Data[0].Criteria[keyThreshold])
}

func TestClientListMonitorAlertDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorAlertDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorAlertDefinitions(t.Context(), 0, 0)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorAlertDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorAlertDefinitionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorAlertDefinitionID, keyLabel: monitorAlertDefinitionLabel}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorAlertDefinitions(t.Context(), 0, 0)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.Data[0].ID)
}

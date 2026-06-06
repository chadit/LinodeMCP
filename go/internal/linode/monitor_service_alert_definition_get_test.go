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
	monitorServiceAlertDefinitionGetPath        = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionEscapedGetPath = "/monitor/services/dbaas%2Fpostgres/alert-definitions/20000"
)

func TestClientGetMonitorServiceAlertDefinitionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID,
			keyLabel:       monitorAlertDefinitionLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)

	monitorRequireNoError(t, err)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
	monitorCheckEqual(t, monitorAlertDefinitionLabel, got.Label)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.ServiceType)
}

func TestClientGetMonitorServiceAlertDefinitionEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceAlertDefinitionEscapedGetPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID, keyServiceType: monitorServiceTypeWithSlash}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeWithSlash, monitorAlertDefinitionID)

	monitorRequireNoError(t, err)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
	monitorCheckEqual(t, monitorServiceTypeWithSlash, got.ServiceType)
}

func TestClientGetMonitorServiceAlertDefinitionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)

	monitorRequireError(t, err)
	monitorCheckEmpty(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetMonitorServiceAlertDefinitionRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)

	monitorRequireNoError(t, err)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
}

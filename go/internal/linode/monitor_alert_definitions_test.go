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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorAlertDefinitionsPath, r.URL.Path, "request path should match")
		assert.Equal(t, monitorAlertDefinitionsQuery, r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 1, got.Pages)
	assert.Equal(t, 1, got.Results)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorAlertDefinitionID, got.Data[0].ID)
	assert.Equal(t, monitorAlertDefinitionLabel, got.Data[0].Label)
	assert.Equal(t, monitorAlertDefinitionType, got.Data[0].Type)
	assert.Equal(t, monitorAlertDefinitionServiceType, got.Data[0].ServiceType)
	assert.Equal(t, monitorAlertDefinitionDescription, got.Data[0].Description)
	assert.Equal(t, 2, got.Data[0].Severity)
	assert.InEpsilon(t, 90, got.Data[0].Criteria[keyThreshold], 0.001)
}

func TestClientListMonitorAlertDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorAlertDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorAlertDefinitions(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorAlertDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorAlertDefinitionsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorAlertDefinitionID, keyLabel: monitorAlertDefinitionLabel}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorAlertDefinitions(t.Context(), 0, 0)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorAlertDefinitionID, got.Data[0].ID)
}

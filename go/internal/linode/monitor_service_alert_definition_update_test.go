package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func monitorAlertDefinitionUpdateRequest() *linode.UpdateAlertDefinitionRequest {
	description := "Updated alert when CPU usage is high"
	severity := 1
	status := statusEnabledFixture
	label := monitorAlertDefinitionLabel + " Updated"

	return &linode.UpdateAlertDefinitionRequest{
		ChannelIDs:  []int{546, 392},
		Description: &description,
		EntityIDs:   []string{"13116"},
		Label:       &label,
		RuleCriteria: map[string]any{
			"rules": []any{map[string]any{
				"metric":     "cpu_usage",
				"operator":   "gt",
				keyThreshold: float64(80),
			}},
		},
		Severity: &severity,
		Status:   &status,
		TriggerConditions: map[string]any{
			"criteria_condition":        "ALL",
			"evaluation_period_seconds": float64(300),
			"polling_interval_seconds":  float64(300),
			"trigger_occurrences":       float64(3),
		},
	}
}

func TestClientUpdateMonitorServiceAlertDefinitionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !monitorCheckNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		monitorCheckEqual(t, monitorAlertDefinitionLabel+" Updated", body[keyLabel])
		monitorCheckNumericEqual(t, float64(1), body[keySeverity])
		monitorCheckEqual(t, statusEnabledFixture, body[keyStatus])
		monitorCheckEqual(t, []any{float64(546), float64(392)}, body["channel_ids"])
		monitorCheckEqual(t, "Updated alert when CPU usage is high", body[keyDescription])
		monitorCheckEqual(t, []any{"13116"}, body["entity_ids"])
		monitorCheckHasKey(t, body, "rule_criteria")
		monitorCheckHasKey(t, body, "trigger_conditions")

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID,
			keyLabel:       monitorAlertDefinitionLabel + " Updated",
			keyServiceType: monitorServiceTypeDatabase,
			keySeverity:    1,
			keyStatus:      statusEnabledFixture,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
	monitorCheckEqual(t, monitorAlertDefinitionLabel+" Updated", got.Label)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.ServiceType)
	monitorCheckEqual(t, statusEnabledFixture, got.Status)
}

func TestClientUpdateMonitorServiceAlertDefinitionPartialStatusUpdate(t *testing.T) {
	t.Parallel()

	status := statusEnabledFixture

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")

		var body map[string]any
		if !monitorCheckNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		monitorCheckEqual(t, map[string]any{keyStatus: statusEnabledFixture}, body)

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID,
			keyLabel:       monitorAlertDefinitionLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, &linode.UpdateAlertDefinitionRequest{Status: &status})

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
	monitorCheckEmpty(t, got.Status, "status is omitted when the API response omits it")
}

func TestClientUpdateMonitorServiceAlertDefinitionEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceAlertDefinitionEscapedGetPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeWithSlash, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
}

func TestClientUpdateMonitorServiceAlertDefinitionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientUpdateMonitorServiceAlertDefinitionDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		monitorCheckEqual(t, monitorServiceAlertDefinitionGetPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)
	monitorCheckEqual(t, int32(1), calls.Load(), "update route must not retry after transient failure")
}

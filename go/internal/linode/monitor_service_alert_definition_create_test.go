package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func monitorAlertDefinitionCreateRequest() *linode.CreateAlertDefinitionRequest {
	description := "Alert when CPU usage is high"

	return &linode.CreateAlertDefinitionRequest{
		ChannelIDs:  []int{546, 392},
		Description: &description,
		EntityIDs:   []string{"13116"},
		Label:       monitorAlertDefinitionLabel,
		RuleCriteria: map[string]any{
			"rules": []any{map[string]any{
				keyMetric:            "cpu_usage",
				"operator":           "gt",
				keyThreshold:         float64(80),
				"aggregate_function": "avg",
			}},
		},
		Severity: 2,
		TriggerConditions: map[string]any{
			"criteria_condition":        "ALL",
			"evaluation_period_seconds": float64(300),
			"polling_interval_seconds":  float64(300),
			"trigger_occurrences":       float64(3),
		},
	}
}

func TestClientCreateMonitorServiceAlertDefinitionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !monitorCheckNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		monitorCheckEqual(t, monitorAlertDefinitionLabel, body[keyLabel])
		monitorCheckNumericEqual(t, float64(2), body[keySeverity])
		monitorCheckEqual(t, []any{float64(546), float64(392)}, body["channel_ids"])
		monitorCheckHasKey(t, body, "rule_criteria")
		monitorCheckHasKey(t, body, "trigger_conditions")

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID,
			keyLabel:       monitorAlertDefinitionLabel,
			keyServiceType: monitorServiceTypeDatabase,
			keySeverity:    2,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionCreateRequest())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
	monitorCheckEqual(t, monitorAlertDefinitionLabel, got.Label)
	monitorCheckEqual(t, monitorServiceTypeDatabase, got.ServiceType)
}

func TestClientCreateMonitorServiceAlertDefinitionEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, monitorServiceEscapedAlertDefinitionsPath, r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeWithSlash, monitorAlertDefinitionCreateRequest())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, monitorAlertDefinitionID, got.ID)
}

func TestClientCreateMonitorServiceAlertDefinitionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionCreateRequest())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateMonitorServiceAlertDefinitionDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceAlertDefinitionsPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.CreateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionCreateRequest())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)
	monitorCheckEqual(t, int32(1), calls.Load(), "create route must not retry after transient failure")
}

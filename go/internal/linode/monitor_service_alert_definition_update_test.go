package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var gotReq linode.UpdateAlertDefinitionRequest
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if want := *monitorAlertDefinitionUpdateRequest(); !reflect.DeepEqual(gotReq, want) {
			t.Errorf("request body = %+v, want %+v", gotReq, want)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID,
			keyLabel:       monitorAlertDefinitionLabel + " Updated",
			keyServiceType: monitorServiceTypeDatabase,
			keySeverity:    1,
			keyStatus:      statusEnabledFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != monitorAlertDefinitionID {
		t.Errorf("got.ID = %v, want %v", got.ID, monitorAlertDefinitionID)
	}

	if got.Label != monitorAlertDefinitionLabel+" Updated" {
		t.Errorf("got.Label = %v, want %v", got.Label, monitorAlertDefinitionLabel+" Updated")
	}

	if got.ServiceType != monitorServiceTypeDatabase {
		t.Errorf("got.ServiceType = %v, want %v", got.ServiceType, monitorServiceTypeDatabase)
	}

	if got.Status != statusEnabledFixture {
		t.Errorf("got.Status = %v, want %v", got.Status, statusEnabledFixture)
	}
}

func TestClientUpdateMonitorServiceAlertDefinitionPartialStatusUpdate(t *testing.T) {
	t.Parallel()

	status := statusEnabledFixture

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body, map[string]any{keyStatus: statusEnabledFixture}) {
			t.Errorf("body = %v, want %v", body, map[string]any{keyStatus: statusEnabledFixture})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID,
			keyLabel:       monitorAlertDefinitionLabel,
			keyServiceType: monitorServiceTypeDatabase,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, &linode.UpdateAlertDefinitionRequest{Status: &status})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != monitorAlertDefinitionID {
		t.Errorf("got.ID = %v, want %v", got.ID, monitorAlertDefinitionID)
	}

	if got.Status != "" {
		t.Errorf("got.Status = %v, want empty", got.Status)
	}
}

func TestClientUpdateMonitorServiceAlertDefinitionEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != monitorServiceAlertDefinitionEscapedGetPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), monitorServiceAlertDefinitionEscapedGetPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeWithSlash, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != monitorAlertDefinitionID {
		t.Errorf("got.ID = %v, want %v", got.ID, monitorAlertDefinitionID)
	}
}

func TestClientUpdateMonitorServiceAlertDefinitionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUpdateMonitorServiceAlertDefinitionDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	got, err := client.UpdateMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID, monitorAlertDefinitionUpdateRequest())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

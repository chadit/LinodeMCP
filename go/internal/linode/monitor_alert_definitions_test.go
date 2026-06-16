package linode_test

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorAlertDefinitionsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorAlertDefinitionsPath)
		}

		if r.URL.RawQuery != monitorAlertDefinitionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, monitorAlertDefinitionsQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
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
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorAlertDefinitions(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Page != 1 {
		t.Errorf("got.Page = %v, want %v", got.Page, 1)
	}

	if got.Pages != 1 {
		t.Errorf("got.Pages = %v, want %v", got.Pages, 1)
	}

	if got.Results != 1 {
		t.Errorf("got.Results = %v, want %v", got.Results, 1)
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != monitorAlertDefinitionID {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, monitorAlertDefinitionID)
	}

	if got.Data[0].Label != monitorAlertDefinitionLabel {
		t.Errorf("got.Data[0].Label = %v, want %v", got.Data[0].Label, monitorAlertDefinitionLabel)
	}

	if got.Data[0].Type != monitorAlertDefinitionType {
		t.Errorf("got.Data[0].Type = %v, want %v", got.Data[0].Type, monitorAlertDefinitionType)
	}

	if got.Data[0].ServiceType != monitorAlertDefinitionServiceType {
		t.Errorf("got.Data[0].ServiceType = %v, want %v", got.Data[0].ServiceType, monitorAlertDefinitionServiceType)
	}

	if got.Data[0].Description != monitorAlertDefinitionDescription {
		t.Errorf("got.Data[0].Description = %v, want %v", got.Data[0].Description, monitorAlertDefinitionDescription)
	}

	if got.Data[0].Severity != 2 {
		t.Errorf("got.Data[0].Severity = %v, want %v", got.Data[0].Severity, 2)
	}

	if numVal, numOK := got.Data[0].Criteria[keyThreshold].(float64); !numOK || math.Abs(numVal-float64(90)) > math.Abs(float64(90))*0.001 {
		t.Errorf("got.Data[0].Criteria[keyThreshold] = %v, want ~%v", got.Data[0].Criteria[keyThreshold], 90)
	}
}

func TestClientListMonitorAlertDefinitionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorAlertDefinitionsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorAlertDefinitionsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorAlertDefinitions(t.Context(), 0, 0)
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

func TestClientListMonitorAlertDefinitionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorAlertDefinitionsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorAlertDefinitionsPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorAlertDefinitionID, keyLabel: monitorAlertDefinitionLabel}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListMonitorAlertDefinitions(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != monitorAlertDefinitionID {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, monitorAlertDefinitionID)
	}
}

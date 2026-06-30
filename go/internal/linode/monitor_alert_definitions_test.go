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

	got, err := client.ListMonitorAlertDefinitionsProto(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].GetId() != monitorAlertDefinitionID {
		t.Errorf("got[0].GetId() = %v, want %v", got[0].GetId(), monitorAlertDefinitionID)
	}

	if got[0].GetLabel() != monitorAlertDefinitionLabel {
		t.Errorf("got[0].GetLabel() = %v, want %v", got[0].GetLabel(), monitorAlertDefinitionLabel)
	}

	if got[0].GetType() != monitorAlertDefinitionType {
		t.Errorf("got[0].GetType() = %v, want %v", got[0].GetType(), monitorAlertDefinitionType)
	}

	if got[0].GetServiceType() != monitorAlertDefinitionServiceType {
		t.Errorf("got[0].GetServiceType() = %v, want %v", got[0].GetServiceType(), monitorAlertDefinitionServiceType)
	}

	if got[0].GetDescription() != monitorAlertDefinitionDescription {
		t.Errorf("got[0].GetDescription() = %v, want %v", got[0].GetDescription(), monitorAlertDefinitionDescription)
	}

	if got[0].GetSeverity() != 2 {
		t.Errorf("got[0].GetSeverity() = %v, want %v", got[0].GetSeverity(), 2)
	}

	threshold := got[0].GetCriteria().GetFields()[keyThreshold].GetNumberValue()
	if math.Abs(threshold-float64(90)) > math.Abs(float64(90))*0.001 {
		t.Errorf("criteria threshold = %v, want ~%v", threshold, 90)
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

	got, err := client.ListMonitorAlertDefinitionsProto(t.Context(), 0, 0)
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

	got, err := client.ListMonitorAlertDefinitionsProto(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].GetId() != monitorAlertDefinitionID {
		t.Errorf("got[0].GetId() = %v, want %v", got[0].GetId(), monitorAlertDefinitionID)
	}
}

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

const (
	monitorServiceAlertDefinitionGetPath        = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionEscapedGetPath = "/monitor/services/dbaas%2Fpostgres/alert-definitions/20000"
)

func TestClientGetMonitorServiceAlertDefinitionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
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

	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != monitorAlertDefinitionID {
		t.Errorf("got.ID = %v, want %v", got.ID, monitorAlertDefinitionID)
	}

	if got.Label != monitorAlertDefinitionLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, monitorAlertDefinitionLabel)
	}

	if got.ServiceType != monitorServiceTypeDatabase {
		t.Errorf("got.ServiceType = %v, want %v", got.ServiceType, monitorServiceTypeDatabase)
	}
}

func TestClientGetMonitorServiceAlertDefinitionEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != monitorServiceAlertDefinitionEscapedGetPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), monitorServiceAlertDefinitionEscapedGetPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID, keyServiceType: monitorServiceTypeWithSlash}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeWithSlash, monitorAlertDefinitionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != monitorAlertDefinitionID {
		t.Errorf("got.ID = %v, want %v", got.ID, monitorAlertDefinitionID)
	}

	if got.ServiceType != monitorServiceTypeWithSlash {
		t.Errorf("got.ServiceType = %v, want %v", got.ServiceType, monitorServiceTypeWithSlash)
	}
}

func TestClientGetMonitorServiceAlertDefinitionAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
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

	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !reflect.DeepEqual(got, linode.AlertDefinition{}) {
		t.Errorf("got = %v, want empty", got)
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

func TestClientGetMonitorServiceAlertDefinitionRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorServiceAlertDefinitionGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorServiceAlertDefinitionGetPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: monitorAlertDefinitionID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetMonitorServiceAlertDefinition(t.Context(), monitorServiceTypeDatabase, monitorAlertDefinitionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if got.ID != monitorAlertDefinitionID {
		t.Errorf("got.ID = %v, want %v", got.ID, monitorAlertDefinitionID)
	}
}

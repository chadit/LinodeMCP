package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListInstanceInterfaceHistorySuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/interfaces/history" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/interfaces/history")
		}

		if r.URL.RawQuery != tcPage2PageSize50 {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, tcPage2PageSize50)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				"interface_history_id": 3,
				"interface_id":         221,
				"linode_id":            123,
				"version":              1,
				keyCreated:             "2025-08-01T00:01:01",
				"interface_data": map[string]any{
					keyID:         1234,
					"mac_address": macAddressFixture,
				},
			}},
			keyPage:    2,
			keyPages:   4,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	history, err := client.ListInstanceInterfaceHistory(t.Context(), 123, 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if history == nil {
		t.Fatal("history is nil")
	}

	if len(history.Data) != 1 {
		t.Fatalf("len(history.Data) = %d, want 1", len(history.Data))
	}

	if history.Data[0].InterfaceHistoryID != 3 {
		t.Errorf("history.Data[0].InterfaceHistoryID = %v, want %v", history.Data[0].InterfaceHistoryID, 3)
	}

	if history.Data[0].InterfaceID != 221 {
		t.Errorf("history.Data[0].InterfaceID = %v, want %v", history.Data[0].InterfaceID, 221)
	}

	if history.Data[0].LinodeID != 123 {
		t.Errorf("history.Data[0].LinodeID = %v, want %v", history.Data[0].LinodeID, 123)
	}

	if history.Data[0].Version != 1 {
		t.Errorf("history.Data[0].Version = %v, want %v", history.Data[0].Version, 1)
	}

	if history.Page != 2 {
		t.Errorf("history.Page = %v, want %v", history.Page, 2)
	}

	if history.Pages != 4 {
		t.Errorf("history.Pages = %v, want %v", history.Pages, 4)
	}

	if history.Results != 1 {
		t.Errorf("history.Results = %v, want %v", history.Results, 1)
	}
}

func TestClientListInstanceInterfaceHistoryInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "token", nil, linode.WithMaxRetries(0))

	history, err := client.ListInstanceInterfaceHistory(t.Context(), 0, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if history != nil {
		t.Errorf("history = %v, want nil", history)
	}

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}
}

func TestClientListInstanceInterfaceHistoryAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	history, err := client.ListInstanceInterfaceHistory(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if history != nil {
		t.Errorf("history = %v, want nil", history)
	}
}

func TestClientListInstanceInterfaceHistoryRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{"interface_history_id": 3}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1))

	history, err := client.ListInstanceInterfaceHistory(t.Context(), 123, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if history == nil {
		t.Fatal("history is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}

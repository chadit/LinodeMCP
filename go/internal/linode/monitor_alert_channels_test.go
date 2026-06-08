package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	monitorAlertChannelsPath     = "/monitor/alert-channels"
	monitorAlertChannelsQuery    = "page=2&page_size=25"
	monitorAlertChannelID        = 10000
	monitorAlertChannelLabel     = "Read-Write Channel"
	monitorAlertChannelEmail     = "Users-with-read-write-access-to-resources"
	monitorAlertDefinitionURL    = "/monitor/alerts-definitions/10000"
	monitorAlertChannelSystem    = "system"
	monitorAlertChannelEmailType = "email"
	monitorAlertChannelCreated   = "2025-03-20T01:41:09"
)

func TestClientListMonitorAlertChannelsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorAlertChannelsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorAlertChannelsPath)
		}

		if r.URL.RawQuery != monitorAlertChannelsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, monitorAlertChannelsQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID:          monitorAlertChannelID,
				keyLabel:       monitorAlertChannelLabel,
				keyType:        monitorAlertChannelSystem,
				"channel_type": monitorAlertChannelEmailType,
				"content": map[string]any{
					monitorAlertChannelEmailType: map[string]any{
						"email_addresses": []string{monitorAlertChannelEmail},
					},
				},
				"alerts": []map[string]any{{
					keyID:    monitorAlertChannelID,
					keyLabel: "High Memory Usage Plan Dedicated",
					"type":   "alerts-definitions",
					"url":    monitorAlertDefinitionURL,
				}},
				keyCreated:   monitorAlertChannelCreated,
				"created_by": "system",
				keyUpdated:   monitorAlertChannelCreated,
				"updated_by": "system",
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorAlertChannels(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Page != 2 {
		t.Errorf("got.Page = %v, want %v", got.Page, 2)
	}

	if got.Pages != 3 {
		t.Errorf("got.Pages = %v, want %v", got.Pages, 3)
	}

	if got.Results != 75 {
		t.Errorf("got.Results = %v, want %v", got.Results, 75)
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != monitorAlertChannelID {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, monitorAlertChannelID)
	}

	if got.Data[0].Label != monitorAlertChannelLabel {
		t.Errorf("got.Data[0].Label = %v, want %v", got.Data[0].Label, monitorAlertChannelLabel)
	}

	if got.Data[0].ChannelType != monitorAlertChannelEmailType {
		t.Errorf("got.Data[0].ChannelType = %v, want %v", got.Data[0].ChannelType, monitorAlertChannelEmailType)
	}

	if got.Data[0].Content.Email.EmailAddresses[0] != monitorAlertChannelEmail {
		t.Errorf("got.Data[0].Content.Email.EmailAddresses[0] = %v, want %v", got.Data[0].Content.Email.EmailAddresses[0], monitorAlertChannelEmail)
	}

	if len(got.Data[0].Alerts) != 1 {
		t.Fatalf("len(got.Data[0].Alerts) = %d, want 1", len(got.Data[0].Alerts))
	}

	if got.Data[0].Alerts[0].URL != monitorAlertDefinitionURL {
		t.Errorf("got.Data[0].Alerts[0].URL = %v, want %v", got.Data[0].Alerts[0].URL, monitorAlertDefinitionURL)
	}
}

func TestClientListMonitorAlertChannelsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorAlertChannelsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorAlertChannelsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListMonitorAlertChannels(t.Context(), 0, 0)
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

func TestClientListMonitorAlertChannelsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != monitorAlertChannelsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, monitorAlertChannelsPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorAlertChannelID, keyLabel: monitorAlertChannelLabel}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListMonitorAlertChannels(t.Context(), 0, 0)
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

	if got.Data[0].ID != monitorAlertChannelID {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, monitorAlertChannelID)
	}
}

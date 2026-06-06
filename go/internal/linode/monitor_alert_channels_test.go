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
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorAlertChannelsPath, r.URL.Path, "request path should match")
		monitorCheckEqual(t, monitorAlertChannelsQuery, r.URL.RawQuery, "request query should include pagination")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
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
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorAlertChannels(t.Context(), 2, 25)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, 2, got.Page)
	monitorCheckEqual(t, 3, got.Pages)
	monitorCheckEqual(t, 75, got.Results)
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorAlertChannelID, got.Data[0].ID)
	monitorCheckEqual(t, monitorAlertChannelLabel, got.Data[0].Label)
	monitorCheckEqual(t, monitorAlertChannelEmailType, got.Data[0].ChannelType)
	monitorCheckEqual(t, monitorAlertChannelEmail, got.Data[0].Content.Email.EmailAddresses[0])
	monitorRequireLenOne(t, got.Data[0].Alerts)
	monitorCheckEqual(t, monitorAlertDefinitionURL, got.Data[0].Alerts[0].URL)
}

func TestClientListMonitorAlertChannelsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorAlertChannelsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorAlertChannels(t.Context(), 0, 0)

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorAlertChannelsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		monitorCheckEqual(t, monitorAlertChannelsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorAlertChannelID, keyLabel: monitorAlertChannelLabel}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorAlertChannels(t.Context(), 0, 0)

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	monitorRequireLenOne(t, got.Data)
	monitorCheckEqual(t, monitorAlertChannelID, got.Data[0].ID)
}

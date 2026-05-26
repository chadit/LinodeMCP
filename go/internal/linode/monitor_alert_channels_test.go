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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorAlertChannelsPath, r.URL.Path, "request path should match")
		assert.Equal(t, monitorAlertChannelsQuery, r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 2, got.Page)
	assert.Equal(t, 3, got.Pages)
	assert.Equal(t, 75, got.Results)
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorAlertChannelID, got.Data[0].ID)
	assert.Equal(t, monitorAlertChannelLabel, got.Data[0].Label)
	assert.Equal(t, monitorAlertChannelEmailType, got.Data[0].ChannelType)
	assert.Equal(t, monitorAlertChannelEmail, got.Data[0].Content.Email.EmailAddresses[0])
	require.Len(t, got.Data[0].Alerts, 1)
	assert.Equal(t, monitorAlertDefinitionURL, got.Data[0].Alerts[0].URL)
}

func TestClientListMonitorAlertChannelsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorAlertChannelsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListMonitorAlertChannels(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListMonitorAlertChannelsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, monitorAlertChannelsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: monitorAlertChannelID, keyLabel: monitorAlertChannelLabel}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListMonitorAlertChannels(t.Context(), 0, 0)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got.Data, 1)
	assert.Equal(t, monitorAlertChannelID, got.Data[0].ID)
}

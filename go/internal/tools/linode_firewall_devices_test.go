package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeFirewallDevicesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallDevicesListTool(&config.Config{})

		assert.Equal(t, "linode_firewall_devices_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
		assert.Contains(t, tool.InputSchema.Properties, keyPage, "schema should include page property")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size property")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		devices := linode.PaginatedResponse[linode.FirewallDevice]{
			Data: []linode.FirewallDevice{{
				ID: 456,
				Entity: linode.FirewallDeviceEntity{
					ID:    123,
					Label: firewallDeviceLabelFixture,
					Type:  monitorAlertDefinitionToolServiceType,
					URL:   "/v4/linode/instances/123",
				},
			}},
			Page:    2,
			Pages:   3,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/devices", r.URL.Path, "request path should match")
			assert.Equal(t, "2", r.URL.Query().Get(keyPage), "page query should match")
			assert.Equal(t, "50", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(devices))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, firewallDeviceLabelFixture, "response should include device entity label")
		assert.Contains(t, textContent.Text, monitorAlertDefinitionToolServiceType, "response should include entity type")
	})

	t.Run("rejects invalid firewall id before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]any{
			"missing firewall id":   nil,
			"zero firewall id":      float64(0),
			"slash firewall id":     paymentMethodIDSlash,
			"query firewall id":     databaseInvalidInstanceIDQuery,
			"traversal firewall id": pathTraversalValue,
		}

		for name, rawID := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called.Store(true)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

				args := map[string]any{}
				if rawID != nil {
					args[keyFirewallID] = rawID
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid firewall_id should be rejected")
				assertErrorContains(t, result, "firewall_id must be a positive integer")
				assert.False(t, called.Load(), "client should not be called for invalid firewall_id")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/devices", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_devices_list")
	})
}

package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeFirewallDeviceCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("create definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallDeviceCreateTool(&config.Config{})

		expectEqual(t, "linode_firewall_device_create", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectEqual(t, profiles.CapWrite, capability, "tool should be write capability")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		expectContains(t, tool.InputSchema.Properties, keyBetaID, "schema should include id property")
		expectContains(t, tool.InputSchema.Properties, keyType, "schema should include type property")
		expectContains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm property")
		expectContains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
		expectContains(t, tool.InputSchema.Required, keyBetaID, "schema should require id")
		expectContains(t, tool.InputSchema.Required, keyType, "schema should require type")
		expectContains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
	})

	t.Run("create success", func(t *testing.T) {
		t.Parallel()

		device := linode.FirewallDevice{
			ID: 789,
			Entity: linode.FirewallDeviceEntity{
				ID:    456,
				Label: firewallDeviceLabelFixture,
				Type:  monitorAlertDefinitionToolServiceType,
				URL:   "/v4/linode/instances/456",
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/networking/firewalls/123/devices", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request should not include query params")

			var got linode.CreateFirewallDeviceRequest
			checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be JSON")
			checkEqual(t, linode.CreateFirewallDeviceRequest{ID: 456, Type: monitorAlertDefinitionToolServiceType}, got)

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(device))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true})
		result, err := handler(t.Context(), req)

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		expectFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "Firewall device assigned successfully", "response should include message")
		expectContains(t, textContent.Text, firewallDeviceLabelFixture, "response should include device entity label")
	})

	t.Run("create rejects invalid confirm before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]any{
			caseMissingConfirm:         nil,
			caseConfirmFalse:           false,
			caseStringConfirmRejected:  boolStringTrue,
			caseNumericConfirmRejected: float64(1),
		}

		for name, rawConfirm := range cases {
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
				_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

				args := map[string]any{keyFirewallID: float64(123), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType}
				if rawConfirm != nil {
					args[keyConfirm] = rawConfirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err, "handler should not return Go error")
				expectNotNil(t, result, "handler should return a result")
				expectTrue(t, result.IsError, "invalid confirm should be rejected")
				assertErrorContains(t, result, "confirm=true")
				expectFalse(t, called.Load(), "client should not be called without confirm")
			})
		}
	})

	t.Run("create rejects invalid input before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]map[string]any{
			caseMissingFirewallPathID:   {keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			caseZeroFirewallPathID:      {keyFirewallID: float64(0), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			caseSlashFirewallPathID:     {keyFirewallID: paymentMethodIDSlash, keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			caseQueryFirewallPathID:     {keyFirewallID: databaseInvalidInstanceIDQuery, keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			caseTraversalFirewallPathID: {keyFirewallID: pathTraversalValue, keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			"missing device id":         {keyFirewallID: float64(123), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			"zero device id":            {keyFirewallID: float64(123), keyBetaID: float64(0), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
			caseInvalidType:             {keyFirewallID: float64(123), keyBetaID: float64(456), keyType: "linode/123", keyConfirm: true},
			"query type":                {keyFirewallID: float64(123), keyBetaID: float64(456), keyType: "linode?x=1", keyConfirm: true},
			"traversal type":            {keyFirewallID: float64(123), keyBetaID: float64(456), keyType: pathTraversalValue, keyConfirm: true},
		}

		for name, args := range cases {
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
				_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err, "handler should not return Go error")
				expectNotNil(t, result, "handler should return a result")
				expectTrue(t, result.IsError, "invalid input should be rejected")
				expectFalse(t, called.Load(), "client should not be called for invalid input")
			})
		}
	})

	t.Run("create client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/networking/firewalls/123/devices", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			checkNoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectTrue(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to create linode_firewall_device_create")
	})
}

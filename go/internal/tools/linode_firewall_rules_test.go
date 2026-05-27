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

func TestLinodeFirewallRulesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallRulesListTool(&config.Config{})

		assert.Equal(t, "linode_firewall_rules_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		rules := linode.FirewallRules{
			InboundPolicy:  policyDrop,
			OutboundPolicy: policyAccept,
			Inbound: []linode.FirewallRule{{
				Action:   policyAccept,
				Protocol: "TCP",
				Ports:    "443",
				Label:    "allow-https",
			}},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/rules", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(rules))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallRulesListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, firewallRuleLabelAllowHTTPS, "response should include rule label")
		assert.Contains(t, textContent.Text, policyDrop, "response should include inbound policy")
	})

	t.Run("rejects invalid firewall id before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]any{
			caseMissingFirewallPathID:   nil,
			caseZeroFirewallPathID:      float64(0),
			caseFractionalLinodeID:      float64(123.5),
			caseSlashFirewallPathID:     paymentMethodIDSlash,
			caseQueryFirewallPathID:     databaseInvalidInstanceIDQuery,
			caseTraversalFirewallPathID: pathTraversalValue,
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
				_, _, handler := tools.NewLinodeFirewallRulesListTool(cfg)

				args := map[string]any{}
				if rawID != nil {
					args[keyFirewallID] = rawID
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid firewall_id should be rejected")
				assertErrorContains(t, result, errFirewallIDPositive)
				assert.False(t, called.Load(), "client should not be called for invalid firewall_id")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/rules", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallRulesListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_rules_list")
	})
}

func TestLinodeFirewallRulesUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallRulesUpdateTool(&config.Config{})

		assert.Equal(t, "linode_firewall_rules_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		assert.Contains(t, tool.InputSchema.Properties, keyInbound, "schema should include inbound property")
		assert.Contains(t, tool.InputSchema.Properties, keyOutbound, "schema should include outbound property")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/networking/firewalls/123/rules", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var got linode.FirewallRules
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be valid JSON")

			if assert.Len(t, got.Inbound, 1) {
				assert.Equal(t, firewallRuleLabelAllowHTTPS, got.Inbound[0].Label)
			}

			assert.Empty(t, got.Outbound)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.FirewallRules{InboundPolicy: policyDrop, Inbound: got.Inbound}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(123),
			keyInbound:    `[{"action":"ACCEPT","protocol":"TCP","ports":"443","label":"allow-https"}]`,
			keyOutbound:   databaseJSONArray,
			keyConfirm:    true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
		assert.Contains(t, textContent.Text, firewallRuleLabelAllowHTTPS, "response should include rule label")
	})

	t.Run("requires explicit confirm before client call", func(t *testing.T) {
		t.Parallel()

		confirmTests := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseConfirmFalse, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
		}

		for _, tt := range confirmTests {
			t.Run(tt.name, func(t *testing.T) {
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
				_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

				args := map[string]any{keyFirewallID: float64(123), keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray}
				if tt.set {
					args[keyConfirm] = tt.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "missing or invalid confirm should be rejected")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.False(t, called.Load(), "client should not be called without confirm=true")
			})
		}
	})

	t.Run("rejects invalid inputs before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			args map[string]any
			want string
		}{
			caseMissingFirewallPathID:   {args: map[string]any{keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
			caseZeroFirewallPathID:      {args: map[string]any{keyFirewallID: float64(0), keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
			caseFractionalLinodeID:      {args: map[string]any{keyFirewallID: float64(123.5), keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
			caseSlashFirewallPathID:     {args: map[string]any{keyFirewallID: paymentMethodIDSlash, keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
			caseQueryFirewallPathID:     {args: map[string]any{keyFirewallID: databaseInvalidInstanceIDQuery, keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
			caseTraversalFirewallPathID: {args: map[string]any{keyFirewallID: pathTraversalValue, keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
			"missing inbound":           {args: map[string]any{keyFirewallID: float64(123), keyOutbound: databaseJSONArray, keyConfirm: true}, want: "inbound is required"},
			"invalid inbound":           {args: map[string]any{keyFirewallID: float64(123), keyInbound: jsonObjectEmpty, keyOutbound: databaseJSONArray, keyConfirm: true}, want: "inbound must be a JSON array"},
			"null outbound":             {args: map[string]any{keyFirewallID: float64(123), keyInbound: databaseJSONArray, keyOutbound: `null`, keyConfirm: true}, want: "outbound must be a JSON array"},
		}

		for name, tt := range cases {
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
				_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid input should be rejected")
				assertErrorContains(t, result, tt.want)
				assert.False(t, called.Load(), "client should not be called for invalid input")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/networking/firewalls/123/rules", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(123),
			keyInbound:    databaseJSONArray,
			keyOutbound:   databaseJSONArray,
			keyConfirm:    true,
		}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update linode_firewall_rules_update")
	})
}

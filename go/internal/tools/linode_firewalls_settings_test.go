package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeFirewallSettingsListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallSettingsListTool(&config.Config{})

		assert.Equal(t, "linode_firewall_settings_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, "page", "schema should include page property")
		assert.Contains(t, tool.InputSchema.Properties, "page_size", "schema should include page_size property")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
			Linode: 100, NodeBalancer: 101, PublicInterface: 200, VPCInterface: 201,
		}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/settings", r.URL.Path, "request path should match")
			assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
			assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(settings))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallSettingsListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"page": float64(2), "page_size": float64(50)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, keyDefaultFirewallIDs, "response should include default firewall IDs")
		assert.Contains(t, textContent.Text, "nodebalancer", "response should include nodebalancer default")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/settings", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallSettingsListTool(cfg)

		result, err := handler(t.Context(), mcp.CallToolRequest{})

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_settings_list")
	})
}

func TestLinodeFirewallSettingsUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

		assert.Equal(t, "linode_firewall_settings_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyDefaultFirewallIDs, "schema should include default_firewall_ids property")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyDefaultFirewallIDs, "default_firewall_ids must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
			Linode: 100, NodeBalancer: 101, PublicInterface: 102, VPCInterface: 103,
		}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/networking/firewalls/settings", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body map[string]map[string]int
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, map[string]int{keyDefaultFirewallLinode: 100, "nodebalancer": 101, "public_interface": 102, "vpc_interface": 103}, body[keyDefaultFirewallIDs])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(settings))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(100), "nodebalancer": float64(101), "public_interface": float64(102), "vpc_interface": float64(103)},
			keyConfirm:            true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Default firewall settings updated successfully", "response should include success message")
		assert.Contains(t, textContent.Text, keyDefaultFirewallIDs, "response should include default firewall IDs")
	})

	t.Run("confirm required", func(t *testing.T) {
		t.Parallel()

		for _, confirm := range []any{nil, false, boolStringTrue, float64(1)} {
			t.Run("reject", func(t *testing.T) {
				t.Parallel()

				_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

				args := map[string]any{keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(100)}}
				if confirm != nil {
					args[keyConfirm] = confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("invalid default firewall IDs", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			ids  any
			want string
		}{
			{name: caseMissing, ids: nil, want: "default_firewall_ids is required"},
			{name: "empty", ids: map[string]any{}, want: "non-empty object"},
			{name: "unsupported key", ids: map[string]any{keyDefaultFirewallLinode: float64(100), "bad": float64(101)}, want: "unsupported key"},
			{name: caseZero, ids: map[string]any{keyDefaultFirewallLinode: float64(0)}, want: errPositiveInteger},
			{name: caseNegative, ids: map[string]any{keyDefaultFirewallLinode: float64(-1)}, want: errPositiveInteger},
			{name: "fractional", ids: map[string]any{keyDefaultFirewallLinode: float64(1.5)}, want: errPositiveInteger},
			{name: caseString, ids: map[string]any{keyDefaultFirewallLinode: "100"}, want: errPositiveInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

				args := map[string]any{keyConfirm: true}
				if testCase.ids != nil {
					args[keyDefaultFirewallIDs] = testCase.ids
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/networking/firewalls/settings", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(100)},
			keyConfirm:            true,
		}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update linode_firewall_settings_update")
	})
}

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

const networkingIPAddressFixture = "198.51.100.5"

func TestLinodeNetworkingIPsListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeNetworkingIPListTool(&config.Config{})

		assert.Equal(t, "linode_networking_ips_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		assert.Contains(t, tool.InputSchema.Properties, "skip_ipv6_rdns", "tool should declare skip_ipv6_rdns")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ips := linode.PaginatedResponse[linode.IPAddress]{
			Data: []linode.IPAddress{{
				Address: networkingIPAddressFixture,
				Type:    keyIPv4,
				Public:  true,
				Region:  regionUSEast,
			}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/ips", r.URL.Path, "request path should match")
			assert.Equal(t, "true", r.URL.Query().Get("skip_ipv6_rdns"), "skip_ipv6_rdns query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ips))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"skip_ipv6_rdns": true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, networkingIPAddressFixture, "response should include IP address")
		assert.Contains(t, textContent.Text, regionUSEast, "response should include region")
	})

	t.Run("invalid skip_ipv6_rdns", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPListTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"skip_ipv6_rdns": boolStringTrue}))

		require.NoError(t, err, "handler should return MCP error result, not Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "invalid skip_ipv6_rdns should be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "skip_ipv6_rdns must be a boolean")
	})
}

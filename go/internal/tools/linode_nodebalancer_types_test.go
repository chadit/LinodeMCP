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
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeNodeBalancerTypesTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeNodeBalancerTypesTool(cfg)

		assert.Equal(t, "linode_nodebalancer_types", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Empty(t, tool.InputSchema.Required, "type lookup should not require arguments")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/nodebalancers/types", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{
					keyID:      "common",
					keyLabel:   "Common",
					keyPrice:   map[string]float64{keyHourly: 0.015, keyMonthly: 10},
					"transfer": 1000,
				}},
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerTypesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "common", "response should contain type id")
		assert.Contains(t, textContent.Text, "Common", "response should contain type label")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/nodebalancers/types", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerTypesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_nodebalancer_types", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

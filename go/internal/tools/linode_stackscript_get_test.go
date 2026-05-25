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

const toolStackScriptGet = "linode_stackscript_get"

func TestLinodeStackScriptGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeStackScriptGetTool(cfg)

		assert.Equal(t, toolStackScriptGet, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyStackScriptID, "schema should include stackscript_id")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		script := linode.StackScript{ID: 123, Label: "deploy-base", Description: "Base deploy script"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(script))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeStackScriptGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyStackScriptID: 123})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deploy-base", "response should contain StackScript label")
		assert.Contains(t, textContent.Text, "123", "response should contain StackScript ID")
	})

	t.Run("client failure returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/stackscripts/123", r.URL.Path, "request path should include StackScript ID")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeStackScriptGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyStackScriptID: 123})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "client failure should return a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve StackScript")
		assert.Contains(t, textContent.Text, errTemporaryFailure)
	})

	t.Run("rejects invalid stackscript_id before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			caseMissing:     nil,
			caseBlank:       "",
			caseNumeric:     "123",
			caseZero:        0,
			caseNegative:    -1,
			caseSlash:       paymentMethodIDSlash,
			caseQuery:       "123?query",
			caseFragment:    "123#fragment",
			caseDotdot:      pathTraversalValue,
			"prefixed path": "../123",
		}

		for name, value := range invalidValues {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeStackScriptGetTool(cfg)

				args := map[string]any{}
				if name != caseMissing {
					args[keyStackScriptID] = value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "invalid stackscript_id should return a tool error")
				assert.False(t, called.Load(), "invalid stackscript_id should be rejected before client call")
			})
		}
	})
}

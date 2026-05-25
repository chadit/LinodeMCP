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

func TestLinodeKernelGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeKernelGetTool(cfg)

		assert.Equal(t, "linode_kernel_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyKernelID, "schema should include kernel_id")
		assert.Contains(t, tool.InputSchema.Required, keyKernelID, "kernel_id must be marked required")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		kernel := linode.Kernel{ID: kernelLatestFixture, Label: kernelLabelFixture, Version: "6.8.9", Architecture: "x86_64"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/kernels/linode/latest-64bit", r.URL.Path, "request path should include kernel ID")
			assert.Equal(t, "/linode/kernels/linode%2Flatest-64bit", r.URL.EscapedPath(), "request path should encode kernel ID slash")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(kernel))
		}))
		defer srv.Close()

		cfg := kernelTestConfig(srv.URL)
		_, _, handler := tools.NewLinodeKernelGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyKernelID: kernelLatestFixture})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, kernelLatestFixture, "response should contain kernel ID")
		assert.Contains(t, textContent.Text, kernelLabelFixture, "response should contain kernel label")
	})

	t.Run("client failure returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/kernels/linode/latest-64bit", r.URL.Path, "request path should include kernel ID")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := kernelTestConfig(srv.URL)
		_, _, handler := tools.NewLinodeKernelGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyKernelID: kernelLatestFixture})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "client failure should return a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve kernel")
		assert.Contains(t, textContent.Text, errTemporaryFailure)
	})

	t.Run("rejects invalid kernel_id before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			caseMissing:        nil,
			caseBlank:          "",
			caseNumeric:        123,
			"missing prefix":   "latest-64bit",
			"empty prefix":     "/latest-64bit",
			"empty name":       "linode/",
			caseExtraSeparator: "linode/latest/64bit",
			caseQuery:          "linode/latest-64bit?arch=x64",
			caseFragment:       "linode/latest-64bit#x64",
			caseDotdot:         pathTraversalValue,
			"prefixed dotdot":  "linode/..",
		}

		for name, value := range invalidValues {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}))
				defer srv.Close()

				cfg := kernelTestConfig(srv.URL)
				_, _, handler := tools.NewLinodeKernelGetTool(cfg)

				args := map[string]any{}
				if name != caseMissing {
					args[keyKernelID] = value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "invalid kernel_id should return a tool error")
				assert.False(t, called.Load(), "invalid kernel_id should be rejected before client call")
			})
		}
	})
}

func kernelTestConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}

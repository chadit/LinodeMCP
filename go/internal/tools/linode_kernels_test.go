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
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeKernelListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeKernelListTool(cfg)

		checkEqual(t, "linode_kernel_list", tool.Name, "tool name should match")
		checkNotEmpty(t, tool.Description, "tool should have a description")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success with pagination", func(t *testing.T) {
		t.Parallel()

		kernels := []linode.Kernel{{ID: "linode/latest-64bit", Label: "Latest 64 bit", Version: "6.15.7", KVM: true, Architecture: "x86_64", PVOPS: true}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/linode/kernels", r.URL.Path, "request path should match")
			checkEqual(t, "3", r.URL.Query().Get("page"), "page query should match")
			checkEqual(t, "25", r.URL.Query().Get("page_size"), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    kernels,
				keyPage:    3,
				keyPages:   14,
				keyResults: 338,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeKernelListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: float64(3), keyPageSize: float64(25)})
		result, err := handler(t.Context(), req)

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		checkFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		checkContains(t, textContent.Text, "linode/latest-64bit", "response should contain kernel ID")
		checkContains(t, textContent.Text, `"count": 1`, "response should contain count")
	})

	t.Run("invalid pagination rejected before client call", func(t *testing.T) {
		t.Parallel()

		testCases := map[string]map[string]any{
			"kernel page below minimum": {keyPage: float64(0)},
			"kernel page malformed":     {keyPage: "two"},
			"page size below minimum":   {keyPageSize: float64(1)},
			"page size above maximum":   {keyPageSize: float64(501)},
			"page size malformed":       {keyPageSize: "many"},
		}

		for name, args := range testCases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}))
				defer srv.Close()

				cfg := &config.Config{
					Environments: map[string]config.EnvironmentConfig{
						envKeyDefault: {
							Label:  envLabelDefault,
							Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
						},
					},
				}
				_, _, handler := tools.NewLinodeKernelListTool(cfg)

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				requireNoError(t, err)
				requireNotNil(t, result)
				checkTrue(t, result.IsError, "invalid pagination should return tool error")
				checkFalse(t, called.Load(), "validation should reject before client call")
			})
		}
	})
}

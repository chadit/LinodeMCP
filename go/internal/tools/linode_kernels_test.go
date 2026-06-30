package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeKernelListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeKernelListTool(cfg)

	if tool.Name != "linode_kernel_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_kernel_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeKernelListToolSuccessWithPagination(t *testing.T) {
	t.Parallel()

	kernels := []linode.Kernel{{ID: "linode/latest-64bit", Label: "Latest 64 bit", Version: "6.15.7", KVM: true, Architecture: "x86_64", PVOPS: true}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/kernels" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/kernels")
		}

		if r.URL.Query().Get("page") != "3" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "3")
		}

		if r.URL.Query().Get("page_size") != "25" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "25")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    kernels,
			keyPage:    3,
			keyPages:   14,
			keyResults: 338,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode/latest-64bit") {
		t.Errorf("textContent.Text does not contain %v", "linode/latest-64bit")
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want %d", got, 1)
	}
}

func TestLinodeKernelListToolInvalidPaginationRejectedBeforeClientCall(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

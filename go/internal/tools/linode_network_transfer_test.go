package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeNetworkTransferPricesToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkTransferPricesTool(&config.Config{})

	if tool.Name != "linode_network_transfer_price_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_network_transfer_price_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkTransferPricesToolSuccess(t *testing.T) {
	t.Parallel()

	prices := linode.PaginatedResponse[linode.NetworkTransferPrice]{
		Data: []linode.NetworkTransferPrice{{
			ID:       "network_transfer",
			Label:    "Network Transfer",
			Price:    linode.Price{Hourly: 0.005},
			Transfer: 0,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/network-transfer/prices" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/network-transfer/prices")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(prices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkTransferPricesTool(cfg)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
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

	if !strings.Contains(textContent.Text, "network_transfer") {
		t.Errorf("textContent.Text does not contain %v", "network_transfer")
	}

	if !strings.Contains(textContent.Text, "Network Transfer") {
		t.Errorf("textContent.Text does not contain %v", "Network Transfer")
	}
}

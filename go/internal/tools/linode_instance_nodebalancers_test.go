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

func TestLinodeInstanceNodeBalancersListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceNodeBalancerListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_nodebalancer_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_nodebalancer_list")
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

	if _, ok := tool.InputSchema.Properties[keyLinodeID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyLinodeID)
	}
}

func TestLinodeInstanceNodeBalancersListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceNodeBalancerListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing instance nodebalancers linode id", args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: "separator instance nodebalancers linode id", args: map[string]any{keyLinodeID: "123/.."}, wantContains: errLinodeIDInteger},
		{name: "query instance nodebalancers linode id", args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: "traversal instance nodebalancers linode id", args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: "negative instance nodebalancers linode id", args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceNodeBalancersListToolSuccess(t *testing.T) {
	t.Parallel()

	nodeBalancers := []linode.NodeBalancer{{ID: 456, Label: "app-lb", Region: regionUSEast}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/nodebalancers" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/nodebalancers")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: nodeBalancers, keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceNodeBalancerListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

	result, err := srvHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "app-lb") {
		t.Errorf("textContent.Text does not contain %v", "app-lb")
	}

	if !strings.Contains(textContent.Text, "nodebalancers") {
		t.Errorf("textContent.Text does not contain %v", "nodebalancers")
	}
}

func TestLinodeInstanceNodeBalancersListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceNodeBalancerListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list NodeBalancers for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list NodeBalancers for instance 123")
	}
}

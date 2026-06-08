package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	errNodeBalancerIDRequired   = "nodebalancer_id is required"
	errNodeBalancerIDInteger    = "nodebalancer_id must be an integer"
	errNodeBalancerIDMin        = "nodebalancer_id must be an integer greater than or equal to 1"
	nodeBalancerNodeAddress     = "192.0.2.10:80"
	nodeBalancerFirewallLabel   = "nb-firewall"
	nodeBalancerNodeKeyMode     = "mode"
	nodeBalancerNodeStatusUP    = "UP"
	nodeBalancerNodeModeAccept  = "accept"
	caseSeparatorNodeID         = "separator node id"
	caseQueryNodeID             = "query node id"
	caseTraversalNodeID         = "traversal node id"
	keyWeight                   = "weight"
	invalidNodeBalancerNodeMode = "invalid"
)

// expect* helpers are fatal package-local checks from linode_assertions_test.go; check* helpers are nonfatal.

func TestLinodeNodeBalancerFirewallListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerFirewallListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_firewall_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_firewall_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[keyNodeBalancerID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyNodeBalancerID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyNodeBalancerID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyNodeBalancerID)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerFirewallListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerFirewallListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1)}, wantContains: errNodeBalancerIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerFirewallListToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Firewalls {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Firewalls)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyLabel: nodeBalancerFirewallLabel, keyStatus: statusEnabled}},
			keyPage: 1, keyPages: 1, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallListTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "firewalls") {
		t.Errorf("textContent.Text does not contain %v", "firewalls")
	}

	if !strings.Contains(textContent.Text, nodeBalancerFirewallLabel) {
		t.Errorf("textContent.Text does not contain %v", nodeBalancerFirewallLabel)
	}
}

func TestLinodeNodeBalancerFirewallListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallListTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list firewalls for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list firewalls for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerConfigListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[keyNodeBalancerID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyNodeBalancerID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyNodeBalancerID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyNodeBalancerID)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1)}, wantContains: errNodeBalancerIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigListToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyNodeBalancerID: 123}},
			keyPage: 1, keyPages: 1, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80)})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "configs") {
		t.Errorf("textContent.Text does not contain %v", "configs")
	}

	if !strings.Contains(textContent.Text, protocolHTTPS) {
		t.Errorf("textContent.Text does not contain %v", protocolHTTPS)
	}

	if !strings.Contains(textContent.Text, "443") {
		t.Errorf("textContent.Text does not contain %v", "443")
	}
}

func TestLinodeNodeBalancerConfigListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigListTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list configs for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list configs for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerConfigNodesListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigNodesListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_nodes_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_nodes_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyPage, keyPageSize} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigNodesListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigNodesListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456)}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456)}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456)}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456)}, wantContains: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123)}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: configIDQueryValue}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue}, wantContains: errConfigIDInteger},
		{name: caseZeroConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(0)}, wantContains: errConfigIDMin},
		{name: paginationCasePageZero, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPage: float64(0)}, wantContains: paginationMessagePageMustBe},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPageSize: float64(24)}, wantContains: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPageSize: float64(501)}, wantContains: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPageSize: "25"}, wantContains: errPageSizeInteger},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigNodesListToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		if r.URL.Query().Get(keyPage) != "2" {
			t.Errorf("r.URL.Query().Get(keyPage) = %v, want %v", r.URL.Query().Get(keyPage), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "25" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "25")
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeBalancerNodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP, keyNodeBalancerID: 123, keyConfigID: 456}},
			keyPage: 2, keyPages: 3, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodesListTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPage: float64(2), keyPageSize: float64(25)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, nodeBalancerNodeLabelWeb1) {
		t.Errorf("textContent.Text does not contain %v", nodeBalancerNodeLabelWeb1)
	}

	if !strings.Contains(textContent.Text, "192.0.2.10:80") {
		t.Errorf("textContent.Text does not contain %v", "192.0.2.10:80")
	}
}

func TestLinodeNodeBalancerConfigNodesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodesListTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list nodes for NodeBalancer 123 config 456") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list nodes for NodeBalancer 123 config 456")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerConfigNodeGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigNodeGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_node_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_node_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyNodeID} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyNodeID} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigNodeGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigNodeGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyNodeID: float64(789)}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456), keyNodeID: float64(789)}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyNodeID: float64(789)}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyNodeID: float64(789)}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1), keyConfigID: float64(456), keyNodeID: float64(789)}, wantContains: errNodeBalancerIDMin},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyNodeID: float64(789)}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorLinodeID, keyNodeID: float64(789)}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyNodeID: float64(789)}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyNodeID: float64(789)}, wantContains: errConfigIDInteger},
		{name: caseZeroConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(0), keyNodeID: float64(789)}, wantContains: errConfigIDMin},
		{name: caseMissingNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}, wantContains: errNodeIDRequired},
		{name: caseSeparatorNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: pathSeparatorLinodeID}, wantContains: errNodeIDInteger},
		{name: caseQueryNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: shareGroupIDQueryValue}, wantContains: errNodeIDInteger},
		{name: caseTraversalNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: pathTraversalValue}, wantContains: errNodeIDInteger},
		{name: "negative node id", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(-1)}, wantContains: "node_id must be an integer greater than or equal to 1"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigNodeGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeBalancerNodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP,
			keyWeight: 100, nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456,
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
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodeGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "192.0.2.10:80") {
		t.Errorf("textContent.Text does not contain %v", "192.0.2.10:80")
	}

	if !strings.Contains(textContent.Text, "web-1") {
		t.Errorf("textContent.Text does not contain %v", "web-1")
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeNodeBalancerConfigNodeGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodeGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve node 789 for NodeBalancer 123 config 456") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve node 789 for NodeBalancer 123 config 456")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerConfigCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_create")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyPort, keySSLCert, keySSLKey, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyPort, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigCreateToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigCreateTool(cfg)

	confirmTests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80)}},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyConfirm: false}},
		{name: caseStringConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyConfirm: boolStringTrue}},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyConfirm: float64(1)}},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeNodeBalancerConfigCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfirm: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1), keyPort: float64(80), keyConfirm: true}, wantContains: errNodeBalancerIDMin},
		{name: "missing port", args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true}, wantContains: "port is required"},
		{name: "invalid port", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(0), keyConfirm: true}, wantContains: "port must be an integer from 1 through 65535"},
		{name: "invalid protocol", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyProtocol: protocolUDP, keyConfirm: true}, wantContains: "protocol must be one of"},
		{name: "invalid algorithm", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyAlgorithm: "random", keyConfirm: true}, wantContains: "algorithm must be one of"},
		{name: "invalid stickiness", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyStickiness: "cookie", keyConfirm: true}, wantContains: "stickiness must be one of"},
		{name: "invalid check", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyCheck: "ping", keyConfirm: true}, wantContains: "check must be one of"},
		{name: "invalid cipher suite", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyCipherSuite: "custom", keyConfirm: true}, wantContains: "cipher_suite must be one of"},
		{name: "missing https tls", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(443), keyProtocol: protocolHTTPS, keyConfirm: true}, wantContains: "ssl_cert and ssl_key are required"},
		{name: "invalid check interval", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyCheckInterval: "ten", keyConfirm: true}, wantContains: "check_interval must be an integer"},
		{name: "negative check timeout", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyCheckTimeout: float64(-1), keyConfirm: true}, wantContains: "check_timeout must be an integer greater than or equal to 1"},
		{name: "invalid check attempts", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyCheckAttempts: "three", keyConfirm: true}, wantContains: "check_attempts must be an integer"},
		{name: "invalid check passive", args: map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyCheckPassive: boolStringTrue, keyConfirm: true}, wantContains: "check_passive must be a boolean"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNodebalancers123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyProtocol:      protocolHTTP,
			keyAlgorithm:     valueRoundRobin,
			keyStickiness:    valueNone,
			keyCheck:         protocolHTTP,
			keyPort:          float64(80),
			keyCheckInterval: float64(10),
			keyCheckPath:     tcHealth,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigCreateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyProtocol: protocolHTTP, keyAlgorithm: valueRoundRobin, keyStickiness: valueNone, keyCheck: protocolHTTP, keyCheckInterval: float64(10), keyCheckTimeout: float64(5), keyCheckAttempts: float64(3), keyCheckPath: tcHealth, keyCheckBody: statusOK, keyCheckPassive: true, keyCipherSuite: valueRecommended, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "config") {
		t.Errorf("textContent.Text does not contain %v", "config")
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}

	if !strings.Contains(textContent.Text, "NodeBalancer 123") {
		t.Errorf("textContent.Text does not contain %v", "NodeBalancer 123")
	}
}

func TestLinodeNodeBalancerConfigCreateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigCreateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyProtocol: protocolHTTP, keyAlgorithm: valueRoundRobin, keyStickiness: valueNone, keyCheck: protocolHTTP, keyCheckInterval: float64(10), keyCheckTimeout: float64(5), keyCheckAttempts: float64(3), keyCheckPath: tcHealth, keyCheckBody: statusOK, keyCheckPassive: true, keyCipherSuite: valueRecommended, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create config for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create config for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerConfigGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456)}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456)}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456)}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456)}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1), keyConfigID: float64(456)}, wantContains: errNodeBalancerIDMin},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123)}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorLinodeID}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue}, wantContains: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(-1)}, wantContains: errConfigIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyNodeBalancerID: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, protocolHTTPS) {
		t.Errorf("textContent.Text does not contain %v", protocolHTTPS)
	}

	if !strings.Contains(textContent.Text, "443") {
		t.Errorf("textContent.Text does not contain %v", "443")
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}
}

func TestLinodeNodeBalancerConfigGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve config 456 for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve config 456 for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerNodeCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerNodeCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_node_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_node_create")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyLabel, keyAddress, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyLabel, keyAddress, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerNodeCreateToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerNodeCreateTool(cfg)

	confirmTests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress}},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: false}},
		{name: caseStringConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: boolStringTrue}},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: float64(1)}},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeNodeBalancerNodeCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerNodeCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(-1), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseMissingLabel, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyAddress: nodeBalancerNodeAddress, keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseMissingAddress, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: "address is required"},
		{name: "invalid weight", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyWeight: float64(0), keyConfirm: true}, wantContains: "weight must be an integer greater than or equal to 1"},
		{name: "invalid mode", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, nodeBalancerNodeKeyMode: invalidNodeBalancerNodeMode, keyConfirm: true}, wantContains: "mode must be one of"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerNodeCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:                nodeBalancerNodeLabelWeb1,
			keyAddress:              nodeBalancerNodeAddress,
			nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyStatus: nodeBalancerNodeStatusUP, nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeCreateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyWeight: float64(50), nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "node") {
		t.Errorf("textContent.Text does not contain %v", "node")
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}

	if !strings.Contains(textContent.Text, "NodeBalancer 123") {
		t.Errorf("textContent.Text does not contain %v", "NodeBalancer 123")
	}
}

func TestLinodeNodeBalancerNodeCreateToolDryRunPreviewDoesNotCallClient(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("dry_run should not call the Linode API")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	dryRunCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, dryRunHandler := tools.NewLinodeNodeBalancerNodeCreateTool(dryRunCfg)

	result, err := dryRunHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_nodebalancer_node_create") {
		t.Errorf("textContent.Text does not contain %v", "linode_nodebalancer_node_create")
	}

	if !strings.Contains(textContent.Text, "POST") {
		t.Errorf("textContent.Text does not contain %v", "POST")
	}

	if !strings.Contains(textContent.Text, tcNodebalancers123Configs456Nodes) {
		t.Errorf("textContent.Text does not contain %v", tcNodebalancers123Configs456Nodes)
	}
}

func TestLinodeNodeBalancerNodeCreateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeCreateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create node for NodeBalancer 123 config 456") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create node for NodeBalancer 123 config 456")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerNodeDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerNodeDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_node_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_node_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyNodeID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyNodeID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerNodeDeleteToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerNodeDeleteTool(cfg)

	confirmTests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789)}},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: false}},
		{name: caseStringConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: boolStringTrue}},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: float64(1)}},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeNodeBalancerNodeDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerNodeDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDInteger},
		{name: caseZeroConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(0), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDMin},
		{name: caseMissingNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeIDRequired},
		{name: caseSeparatorNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: pathSeparatorLinodeID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeIDInteger},
		{name: caseQueryNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: shareGroupIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeIDInteger},
		{name: caseTraversalNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeIDInteger},
		{name: "negative node id", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(-1), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "node_id must be an integer greater than or equal to 1"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerNodeDeleteToolDryRunReturnsPreviewWithoutDeleting(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, `"dry_run": true`) {
		t.Errorf("textContent.Text does not contain %v", `"dry_run": true`)
	}

	if !strings.Contains(textContent.Text, `"method": "DELETE"`) {
		t.Errorf("textContent.Text does not contain %v", `"method": "DELETE"`)
	}

	if !strings.Contains(textContent.Text, `"path": "/nodebalancers/123/configs/456/nodes/789"`) {
		t.Errorf("textContent.Text does not contain %v", `"path": "/nodebalancers/123/configs/456/nodes/789"`)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestLinodeNodeBalancerNodeDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeNodeBalancerNodeDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete node 789 from NodeBalancer 123 config 456") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete node 789 from NodeBalancer 123 config 456")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerNodeDeleteToolTransientErrorIsNotReplayed(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}},
		Resilience:   config.ResilienceConfig{MaxRetries: 2},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestLinodeNodeBalancerNodeUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerNodeUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_node_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_node_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyNodeID, keyLabel, keyAddress, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyNodeID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerNodeUpdateToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerNodeUpdateTool(cfg)

	confirmTests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1}},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: false}},
		{name: caseStringConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: boolStringTrue}},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: float64(1)}},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeNodeBalancerNodeUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerNodeUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseMissingNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeIDRequired},
		{name: caseSeparatorNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: pathSeparatorValue, keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeIDInteger},
		{name: caseQueryNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: shareGroupIDQueryValue, keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeIDInteger},
		{name: caseTraversalNodeID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: pathTraversalValue, keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}, wantContains: errNodeIDInteger},
		{name: managedContactUpdateEmptyCase, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true}, wantContains: "at least one update field is required"},
		{name: caseMissingLabel, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: " ", keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseMissingAddress, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyAddress: " ", keyConfirm: true}, wantContains: "address is required"},
		{name: "invalid weight", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyWeight: float64(0), keyConfirm: true}, wantContains: "weight must be an integer greater than or equal to 1"},
		{name: "invalid mode", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), nodeBalancerNodeKeyMode: invalidNodeBalancerNodeMode, keyConfirm: true}, wantContains: "mode must be one of"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerNodeUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:                nodeBalancerNodeLabelWeb1,
			keyAddress:              nodeBalancerNodeAddress,
			keyWeight:               float64(50),
			nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyStatus: nodeBalancerNodeStatusUP, nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyWeight: float64(50), nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeNodeBalancerNodeUpdateToolDryRunPreviewDoesNotCallClient(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("dry_run should not call the Linode API")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	dryRunCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, dryRunHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(dryRunCfg)

	result, err := dryRunHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_nodebalancer_node_update") {
		t.Errorf("textContent.Text does not contain %v", "linode_nodebalancer_node_update")
	}

	if !strings.Contains(textContent.Text, "PUT") {
		t.Errorf("textContent.Text does not contain %v", "PUT")
	}

	if !strings.Contains(textContent.Text, tcNodebalancers123Configs456Nodes789) {
		t.Errorf("textContent.Text does not contain %v", tcNodebalancers123Configs456Nodes789)
	}
}

func TestLinodeNodeBalancerNodeUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update node 789 for NodeBalancer 123 config 456") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update node 789 for NodeBalancer 123 config 456")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerNodeUpdateToolEmptyResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "empty response") {
		t.Errorf("error text %q does not contain %q", text.Text, "empty response")
	}
}

func TestLinodeNodeBalancerConfigRebuildToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigRebuildTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_rebuild" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_rebuild")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigRebuildToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigRebuildTool(cfg)

	confirmTests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: false}},
		{name: caseStringConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: boolStringTrue}},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: float64(1)}},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeNodeBalancerConfigRebuildToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigRebuildTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errNodeBalancerIDInteger},
		{name: "missing config id", args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: configIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDInteger},
		{name: caseZeroConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(0), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigRebuildToolDryRun(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigRebuildTool(cfg)

	t.Parallel()

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_nodebalancer_config_rebuild") {
		t.Errorf("textContent.Text does not contain %v", "linode_nodebalancer_config_rebuild")
	}

	if !strings.Contains(textContent.Text, "POST") {
		t.Errorf("textContent.Text does not contain %v", "POST")
	}

	if !strings.Contains(textContent.Text, "/nodebalancers/123/configs/456/rebuild") {
		t.Errorf("textContent.Text does not contain %v", "/nodebalancers/123/configs/456/rebuild")
	}
}

func TestLinodeNodeBalancerConfigRebuildToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/nodebalancers/123/configs/456/rebuild" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/configs/456/rebuild")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigRebuildTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Rebuilt config 456") {
		t.Errorf("textContent.Text does not contain %v", "Rebuilt config 456")
	}

	if !strings.Contains(textContent.Text, "NodeBalancer 123") {
		t.Errorf("textContent.Text does not contain %v", "NodeBalancer 123")
	}
}

func TestLinodeNodeBalancerConfigRebuildToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigRebuildTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to rebuild config 456 for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to rebuild config 456 for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerConfigUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyPort, keySSLCert, keySSLKey, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigUpdateToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigUpdateTool(cfg)

	confirmTests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443)}},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyConfirm: false}},
		{name: caseStringConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyConfirm: boolStringTrue}},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyConfirm: float64(1)}},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeNodeBalancerConfigUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: managedContactUpdateEmptyCase, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true}, wantContains: "at least one update field is required"},
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyConfirm: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyConfigID: float64(456), keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: "missing config id", args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true}, wantContains: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: configIDQueryValue, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseZeroConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(0), keyConfirm: true}, wantContains: errConfigIDMin},
		{name: "invalid port", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(0), keyConfirm: true}, wantContains: "port must be an integer from 1 through 65535"},
		{name: "invalid protocol", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyProtocol: protocolUDP, keyConfirm: true}, wantContains: "protocol must be one of"},
		{name: "invalid algorithm", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyAlgorithm: "random", keyConfirm: true}, wantContains: "algorithm must be one of"},
		{name: "invalid stickiness", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyStickiness: "cookie", keyConfirm: true}, wantContains: "stickiness must be one of"},
		{name: "invalid check", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyCheck: "ping", keyConfirm: true}, wantContains: "check must be one of"},
		{name: "invalid cipher suite", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyCipherSuite: "custom", keyConfirm: true}, wantContains: "cipher_suite must be one of"},
		{name: "invalid check interval", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyCheckInterval: "ten", keyConfirm: true}, wantContains: "check_interval must be an integer"},
		{name: "negative check timeout", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyCheckTimeout: float64(-1), keyConfirm: true}, wantContains: "check_timeout must be an integer greater than or equal to 1"},
		{name: "invalid check attempts", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyCheckAttempts: "three", keyConfirm: true}, wantContains: "check_attempts must be an integer"},
		{name: "missing https tls", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyProtocol: protocolHTTPS, keyConfirm: true}, wantContains: "ssl_cert and ssl_key are required"},
		{name: "invalid check passive", args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyCheckPassive: boolStringTrue, keyConfirm: true}, wantContains: "check_passive must be a boolean"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}},
			keyPage: 1, keyPages: 1, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_nodebalancer_config_update") {
		t.Errorf("textContent.Text does not contain %v", "linode_nodebalancer_config_update")
	}

	if !strings.Contains(textContent.Text, "PUT") {
		t.Errorf("textContent.Text does not contain %v", "PUT")
	}

	if !strings.Contains(textContent.Text, tcNodebalancers123Configs456) {
		t.Errorf("textContent.Text does not contain %v", tcNodebalancers123Configs456)
	}
}

func TestLinodeNodeBalancerConfigUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		port, portOK := body[keyPort].(float64)
		if !portOK {
			t.Error("portOK = false, want true")
		}

		if int(port) != 443 {
			t.Errorf("int(port) = %v, want %v", int(port), 443)
		}

		if !reflect.DeepEqual(body[keyProtocol], protocolHTTPS) {
			t.Errorf("body[keyProtocol] = %v, want %v", body[keyProtocol], protocolHTTPS)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyNodeBalancerID: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyProtocol: protocolHTTPS, keySSLCert: testCertPEM, keySSLKey: testKeyPEM, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "config") {
		t.Errorf("textContent.Text does not contain %v", "config")
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}

	if !strings.Contains(textContent.Text, "NodeBalancer 123") {
		t.Errorf("textContent.Text does not contain %v", "NodeBalancer 123")
	}
}

func TestLinodeNodeBalancerConfigUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update config 456 for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update config 456 for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNodeBalancerFirewallUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerFirewallUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_firewall_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_firewall_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyNodeBalancerID, keyFirewallIDs, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyNodeBalancerID, keyFirewallIDs, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerFirewallUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerFirewallUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1), keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, wantContains: errNodeBalancerIDMin},
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyFirewallIDs: []any{float64(456)}}, wantContains: errConfirmEqualsTrue},
		{name: "false confirm", args: map[string]any{keyNodeBalancerID: float64(123), keyFirewallIDs: []any{float64(456)}, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseString, args: map[string]any{keyNodeBalancerID: float64(123), keyFirewallIDs: []any{float64(456)}, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumeric, args: map[string]any{keyNodeBalancerID: float64(123), keyFirewallIDs: []any{float64(456)}, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: "missing firewall ids", args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true}, wantContains: "firewall_ids is required"},
		{name: "bad firewall ids", args: map[string]any{keyNodeBalancerID: float64(123), keyFirewallIDs: []any{"456"}, keyConfirm: true}, wantContains: "firewall_ids entries must be positive integers"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerFirewallUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Firewalls {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Firewalls)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string][]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyFirewallIDs], []int{456, 789}) {
			t.Errorf("body[keyFirewallIDs] = %v, want %v", body[keyFirewallIDs], []int{456, 789})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyLabel: nodeBalancerFirewallLabel, keyStatus: statusEnabled}},
			keyPage: 2, keyPages: 3, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(123), keyFirewallIDs: []any{float64(456), float64(789)}, "page": float64(2), "page_size": float64(25), keyConfirm: true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "firewalls") {
		t.Errorf("textContent.Text does not contain %v", "firewalls")
	}

	if !strings.Contains(textContent.Text, nodeBalancerFirewallLabel) {
		t.Errorf("textContent.Text does not contain %v", nodeBalancerFirewallLabel)
	}
}

func TestLinodeNodeBalancerFirewallUpdateToolEmptyAssignments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Firewalls {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Firewalls)
		}

		var body map[string][]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body[keyFirewallIDs]) != 0 {
			t.Errorf("body[keyFirewallIDs] = %v, want empty", body[keyFirewallIDs])
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{}, keyPage: 1, keyPages: 1, keyResults: 0}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(123), keyFirewallIDs: []any{}, keyConfirm: true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeNodeBalancerFirewallUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallUpdateTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(123), keyFirewallIDs: []any{float64(456)}, keyConfirm: true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update firewall assignments for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update firewall assignments for NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

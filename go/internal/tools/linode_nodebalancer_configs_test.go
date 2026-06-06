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

func TestLinodeNodeBalancerFirewallListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerFirewallListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_firewall_list", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/123/firewalls", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{keyID: 456, keyLabel: nodeBalancerFirewallLabel, keyStatus: statusEnabled}},
				keyPage: 1, keyPages: 1, keyResults: 1,
			}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallListTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "firewalls", "response should contain firewall list")
		expectContainsWithMode(t, false, textContent.Text, nodeBalancerFirewallLabel, "response should contain firewall label")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerFirewallListTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list firewalls for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerConfigListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_config_list", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/123/configs", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyNodeBalancerID: 123}},
				keyPage: 1, keyPages: 1, keyResults: 1,
			}))
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "configs", "response should contain config list")
		expectContainsWithMode(t, false, textContent.Text, protocolHTTPS, "response should contain protocol")
		expectContainsWithMode(t, false, textContent.Text, "443", "response should contain port")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigListTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list configs for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerConfigNodesListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigNodesListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_config_nodes_list", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyPage, "schema should include page")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
			checkEqual(t, "2", r.URL.Query().Get(keyPage), "page query should match")
			checkEqual(t, "25", r.URL.Query().Get(keyPageSize), "page_size query should match")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeBalancerNodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP, keyNodeBalancerID: 123, keyConfigID: 456}},
				keyPage: 2, keyPages: 3, keyResults: 1,
			}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodesListTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPage: float64(2), keyPageSize: float64(25)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, nodeBalancerNodeLabelWeb1, "response should contain node label")
		expectContainsWithMode(t, false, textContent.Text, "192.0.2.10:80", "response should contain node address")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodesListTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list nodes for NodeBalancer 123 config 456")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerConfigNodeGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigNodeGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_config_node_get", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeID, "schema should include node_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeID, "schema should require node_id")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeBalancerNodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP,
				keyWeight: 100, nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456,
			}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodeGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "192.0.2.10:80", "response should contain node address")
		expectContainsWithMode(t, false, textContent.Text, "web-1", "response should contain node label")
		expectContainsWithMode(t, false, textContent.Text, "789", "response should contain node ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigNodeGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve node 789 for NodeBalancer 123 config 456")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerConfigCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_config_create", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapWrite, capability, "tool should require write capability")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyPort, "schema should include port")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keySSLCert, "schema should include ssl_cert")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keySSLKey, "schema should include ssl_key")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyPort, "schema should require port")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, "confirm=true")
		})
	}

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/nodebalancers/123/configs", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
			port, portOK := body[keyPort].(float64)
			checkTrueWithMode(t, false, portOK, "request body port should be numeric")
			checkEqual(t, 80, int(port), "request body should include port")
			checkEqual(t, protocolHTTP, body[keyProtocol], "request body should include protocol")
			checkEqual(t, valueRoundRobin, body[keyAlgorithm], "request body should include algorithm")
			checkEqual(t, valueNone, body[keyStickiness], "request body should include stickiness")
			checkEqual(t, protocolHTTP, body[keyCheck], "request body should include check")
			checkInterval, ok := body[keyCheckInterval].(float64)
			checkTrueWithMode(t, false, ok, "request body check_interval should be numeric")
			checkEqual(t, 10, int(checkInterval), "request body should include check_interval")
			checkEqual(t, "/health", body[keyCheckPath], "request body should include check_path")

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigCreateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyProtocol: protocolHTTP, keyAlgorithm: valueRoundRobin, keyStickiness: valueNone, keyCheck: protocolHTTP, keyCheckInterval: float64(10), keyCheckTimeout: float64(5), keyCheckAttempts: float64(3), keyCheckPath: "/health", keyCheckBody: statusOK, keyCheckPassive: true, keyCipherSuite: valueRecommended, keyConfirm: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "config", "response should contain created config")
		expectContainsWithMode(t, false, textContent.Text, "456", "response should contain config ID")
		expectContainsWithMode(t, false, textContent.Text, "NodeBalancer 123", "response should include parent NodeBalancer")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigCreateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyPort: float64(80), keyProtocol: protocolHTTP, keyAlgorithm: valueRoundRobin, keyStickiness: valueNone, keyCheck: protocolHTTP, keyCheckInterval: float64(10), keyCheckTimeout: float64(5), keyCheckAttempts: float64(3), keyCheckPath: "/health", keyCheckBody: statusOK, keyCheckPassive: true, keyCipherSuite: valueRecommended, keyConfirm: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to create config for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerConfigGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_config_get", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyNodeBalancerID: 123}))
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, protocolHTTPS, "response should contain protocol")
		expectContainsWithMode(t, false, textContent.Text, "443", "response should contain port")
		expectContainsWithMode(t, false, textContent.Text, "456", "response should contain config id")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve config 456 for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerNodeCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerNodeCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_nodebalancer_node_create", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapWrite, capability, "tool should require write capability")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyLabel, "schema should include label")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyAddress, "schema should include address")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyLabel, "schema should require label")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyAddress, "schema should require address")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, "confirm=true")
		})
	}

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
			checkEqual(t, nodeBalancerNodeLabelWeb1, body[keyLabel], "request body should include label")
			checkEqual(t, nodeBalancerNodeAddress, body[keyAddress], "request body should include address")
			checkEqual(t, nodeBalancerNodeModeAccept, body[nodeBalancerNodeKeyMode], "request body should include mode")

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyStatus: nodeBalancerNodeStatusUP, nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeCreateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyWeight: float64(50), nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyConfirm: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "node", "response should contain created node")
		expectContainsWithMode(t, false, textContent.Text, "789", "response should contain node ID")
		expectContainsWithMode(t, false, textContent.Text, "NodeBalancer 123", "response should include parent NodeBalancer")
	})

	t.Run("dry_run preview does not call client", func(t *testing.T) {
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "dry_run should return a preview")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "linode_nodebalancer_node_create", "preview should include tool name")
		expectContainsWithMode(t, false, textContent.Text, "POST", "preview should include method")
		expectContainsWithMode(t, false, textContent.Text, "/nodebalancers/123/configs/456/nodes", "preview should include path")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeCreateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyConfirm: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to create node for NodeBalancer 123 config 456")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerNodeDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerNodeDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_node_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should require destroy capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeID, "schema should include node_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		assert.Contains(t, tool.InputSchema.Required, keyNodeID, "schema should require node_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, "confirm=true")
		})
	}

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("dry_run returns preview without deleting", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			assert.Equal(t, http.MethodGet, r.Method, "dry_run must only issue GET")
			assert.Equal(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "dry_run should not require confirm")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"dry_run": true`)
		assert.Contains(t, textContent.Text, `"method": "DELETE"`)
		assert.Contains(t, textContent.Text, `"path": "/nodebalancers/123/configs/456/nodes/789"`)
		assert.Equal(t, int32(1), calls.Load(), "dry_run must issue exactly one GET and no DELETE")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
		assert.Contains(t, textContent.Text, "789", "response should contain node ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to delete node 789 from NodeBalancer 123 config 456")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("transient error is not replayed", func(t *testing.T) {
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

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assert.Equal(t, int32(1), calls.Load(), "destructive delete should not be retried")
	})
}

func TestLinodeNodeBalancerNodeUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerNodeUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_node_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeID, "schema should include node_id")
		assert.Contains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		assert.Contains(t, tool.InputSchema.Properties, keyAddress, "schema should include address")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		assert.Contains(t, tool.InputSchema.Required, keyNodeID, "schema should require node_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, "confirm=true")
		})
	}

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, nodeBalancerNodeLabelWeb1, body[keyLabel], "request body should include label")
			assert.Equal(t, nodeBalancerNodeAddress, body[keyAddress], "request body should include address")
			assert.InDelta(t, float64(50), body[keyWeight], 0.001, "request body should include weight")
			assert.Equal(t, nodeBalancerNodeModeAccept, body[nodeBalancerNodeKeyMode], "request body should include mode")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyStatus: nodeBalancerNodeStatusUP, nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyAddress: nodeBalancerNodeAddress, keyWeight: float64(50), nodeBalancerNodeKeyMode: nodeBalancerNodeModeAccept, keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
		assert.Contains(t, textContent.Text, "789", "response should contain node ID")
	})

	t.Run("dry_run preview does not call client", func(t *testing.T) {
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

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "dry_run should return a preview")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode_nodebalancer_node_update", "preview should include tool name")
		assert.Contains(t, textContent.Text, "PUT", "preview should include method")
		assert.Contains(t, textContent.Text, "/nodebalancers/123/configs/456/nodes/789", "preview should include path")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update node 789 for NodeBalancer 123 config 456")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("empty response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerNodeUpdateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyNodeID: float64(789), keyLabel: nodeBalancerNodeLabelWeb1, keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "empty response")
	})
}

func TestLinodeNodeBalancerConfigRebuildTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigRebuildTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_config_rebuild", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, "confirm=true")
		})
	}

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode_nodebalancer_config_rebuild", "dry-run response should name the tool")
		assert.Contains(t, textContent.Text, "POST", "dry-run response should include method")
		assert.Contains(t, textContent.Text, "/nodebalancers/123/configs/456/rebuild", "dry-run response should include rebuild path")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/nodebalancers/123/configs/456/rebuild", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigRebuildTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Rebuilt config 456", "response should confirm rebuild")
		assert.Contains(t, textContent.Text, "NodeBalancer 123", "response should include parent NodeBalancer")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigRebuildTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to rebuild config 456 for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerConfigUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_config_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		assert.Contains(t, tool.InputSchema.Properties, keyPort, "schema should include port")
		assert.Contains(t, tool.InputSchema.Properties, keySSLCert, "schema should include ssl_cert")
		assert.Contains(t, tool.InputSchema.Properties, keySSLKey, "schema should include ssl_key")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfigID, "schema should require config_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, "confirm=true")
		})
	}

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "dry-run should use GET for preview")
			assert.Equal(t, "/nodebalancers/123/configs", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}},
				keyPage: 1, keyPages: 1, keyResults: 1,
			}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigUpdateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode_nodebalancer_config_update", "dry-run response should name the tool")
		assert.Contains(t, textContent.Text, "PUT", "dry-run response should include method")
		assert.Contains(t, textContent.Text, "/nodebalancers/123/configs/456", "dry-run response should include update path")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			port, portOK := body[keyPort].(float64)
			assert.True(t, portOK, "request body port should be numeric")
			assert.Equal(t, 443, int(port), "request body should include port")
			assert.Equal(t, protocolHTTPS, body[keyProtocol], "request body should include protocol")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyNodeBalancerID: 123}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigUpdateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyProtocol: protocolHTTPS, keySSLCert: testCertPEM, keySSLKey: testKeyPEM, keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "config", "response should contain updated config")
		assert.Contains(t, textContent.Text, "456", "response should contain config ID")
		assert.Contains(t, textContent.Text, "NodeBalancer 123", "response should include parent NodeBalancer")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigUpdateTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyPort: float64(443), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update config 456 for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeNodeBalancerFirewallUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerFirewallUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_firewall_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallIDs, "schema should include firewall_ids")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyNodeBalancerID, "schema should require nodebalancer_id")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallIDs, "schema should require firewall_ids")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

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
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/nodebalancers/123/firewalls", r.URL.Path, "request path should match")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string][]int
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Equal(t, []int{456, 789}, body[keyFirewallIDs], "request body should include firewall IDs")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{keyID: 456, keyLabel: nodeBalancerFirewallLabel, keyStatus: statusEnabled}},
				keyPage: 2, keyPages: 3, keyResults: 1,
			}))
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

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "firewalls", "response should contain firewall list")
		assert.Contains(t, textContent.Text, nodeBalancerFirewallLabel, "response should contain firewall label")
	})

	t.Run("empty assignments", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/nodebalancers/123/firewalls", r.URL.Path, "request path should match")

			var body map[string][]int
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Empty(t, body[keyFirewallIDs], "empty firewall_ids should be forwarded")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{}, keyPage: 1, keyPages: 1, keyResults: 0}))
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

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
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

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update firewall assignments for NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}

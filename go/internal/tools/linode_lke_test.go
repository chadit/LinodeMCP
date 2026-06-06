package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// End-to-end verification of LKE cluster listing and filtering.
func TestLinodeLKEClustersListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEClusterListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		clusters := []linode.LKECluster{
			{ID: 1, Label: labelProdCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady},
			{ID: 2, Label: "dev-cluster", Region: regionEUWest, K8sVersion: lkeVersion128, Status: statusReady},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    clusters,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, labelProdCluster, "response should contain prod-cluster")
		expectContainsWithMode(t, false, textContent.Text, "dev-cluster", "response should contain dev-cluster")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		clusters := []linode.LKECluster{
			{ID: 1, Label: labelProdCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady},
			{ID: 2, Label: "dev-cluster", Region: regionEUWest, K8sVersion: lkeVersion128, Status: statusReady},
			{ID: 3, Label: "staging-prod", Region: regionUSWest, K8sVersion: lkeVersion129, Status: statusReady},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    clusters,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLabel: "prod"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, labelProdCluster, "response should contain prod-cluster")
		expectContainsWithMode(t, false, textContent.Text, "staging-prod", "response should contain staging-prod")
		expectNotContains(t, textContent.Text, "dev-cluster", "response should not contain dev-cluster")
	})
}

// End-to-end verification of LKE cluster get workflow.
func TestLinodeLKEClusterGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{}, wantContains: errClusterIDRequired},
		{name: "invalid cluster id", args: map[string]any{keyClusterID: notANumber}, wantContains: "cluster_id must be a valid integer"},
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

		cluster := linode.LKECluster{
			ID: 123, Label: labelProdCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(cluster), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, labelProdCluster, "response should contain cluster label")
		expectContainsWithMode(t, false, textContent.Text, lkeVersion129, "response should contain k8s version")
	})
}

// TestLinodeLKEPoolsListTool verifies the LKE pools list tool
// registers correctly, validates cluster_id, and returns pool data.
func TestLinodeLKEPoolsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_pool_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingClusterID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errClusterIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		pools := []linode.LKENodePool{
			{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 3},
			{ID: 11, ClusterID: 123, Type: "g6-standard-4", Count: 2},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/pools", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: pools, keyPage: 1, keyPages: 1, keyResults: 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEPoolListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, typeG6Standard2, "response should contain pool type")
		expectContainsWithMode(t, false, textContent.Text, "g6-standard-4", "response should contain pool type")
		expectContainsWithMode(t, false, textContent.Text, `"count": 2`, "response should contain pool count")
	})
}

// TestLinodeLKEPoolGetTool verifies the LKE pool get tool
// registers correctly, validates required fields, and retrieves pool details.
func TestLinodeLKEPoolGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_pool_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{keyPoolID: "10"}, wantContains: errClusterIDRequired},
		{name: "missing pool id", args: map[string]any{keyClusterID: "123"}, wantContains: "pool_id is required"},
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

		pool := linode.LKENodePool{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 3}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/pools/10", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(pool), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEPoolGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123", keyPoolID: "10"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, typeG6Standard2, "response should contain pool type")
	})
}

// TestLinodeLKENodeGetTool verifies the LKE node get tool
// registers correctly, validates required fields, and retrieves node details.
func TestLinodeLKENodeGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKENodeGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_node_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{keyNodeID: idAbc123}, wantContains: errClusterIDRequired},
		{name: "missing node id", args: map[string]any{keyClusterID: "123"}, wantContains: "node_id is required"},
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

		node := linode.LKENode{ID: idAbc123, InstanceID: 456, Status: statusReady}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/nodes/abc-123", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(node), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKENodeGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123", keyNodeID: idAbc123})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, idAbc123, "response should contain node ID")
		expectContainsWithMode(t, false, textContent.Text, statusReady, "response should contain node status")
	})
}

// TestLinodeLKEKubeconfigGetTool verifies the LKE kubeconfig get tool
// registers correctly, validates cluster_id, and returns kubeconfig data.
func TestLinodeLKEKubeconfigGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_kubeconfig_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingClusterID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errClusterIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		kubeconfig := linode.LKEKubeconfig{
			Kubeconfig: "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/kubeconfig", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(kubeconfig), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEKubeconfigGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==", "response should contain kubeconfig data")
	})
}

// TestLinodeLKEDashboardGetTool verifies the LKE dashboard get tool
// registers correctly and returns the dashboard URL.
func TestLinodeLKEDashboardGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEDashboardGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_dashboard_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		dashboard := linode.LKEDashboard{URL: "https://dashboard.lke.example.com"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/dashboard", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(dashboard), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEDashboardGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "https://dashboard.lke.example.com", "response should contain dashboard URL")
	})
}

// TestLinodeLKEAPIEndpointsListTool verifies the LKE API endpoints list tool
// registers correctly and returns available API endpoints.
func TestLinodeLKEAPIEndpointsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEAPIEndpointListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_api_endpoint_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		endpoints := []linode.LKEAPIEndpoint{
			{Endpoint: "https://abc123.us-east.lke.example.com:443"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/api-endpoints", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: endpoints, keyPage: 1, keyPages: 1, keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEAPIEndpointListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "abc123.us-east.lke.example.com", "response should contain API endpoint")
	})
}

// TestLinodeLKEACLGetTool verifies the LKE ACL get tool
// registers correctly and returns control plane ACL data.
func TestLinodeLKEACLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEACLGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_acl_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		acl := linode.LKEControlPlaneACL{
			Enabled: true,
			Addresses: linode.LKEControlPlaneACLAddresses{
				IPv4: []string{"10.0.0.1/32"},
				IPv6: []string{cidrV6},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/control_plane_acl", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(acl), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEACLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "10.0.0.1/32", "response should contain IPv4 address")
		expectContainsWithMode(t, false, textContent.Text, cidrV6, "response should contain IPv6 address")
	})
}

// TestLinodeLKEVersionsListTool verifies the LKE versions list tool
// registers correctly and returns available Kubernetes versions.
func TestLinodeLKEVersionsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEVersionListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_version_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		versions := []linode.LKEVersion{{ID: lkeVersion129}, {ID: lkeVersion128}, {ID: "1.27"}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/versions", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: versions, keyPage: 1, keyPages: 1, keyResults: 3,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEVersionListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, lkeVersion129, "response should contain version 1.29")
		expectContainsWithMode(t, false, textContent.Text, lkeVersion128, "response should contain version 1.28")
	})
}

// TestLinodeLKEVersionGetTool verifies the LKE version get tool
// registers correctly, validates the version parameter, and returns version details.
func TestLinodeLKEVersionGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_version_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing version", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "version is required")
	})

	t.Run("invalid version path parameter", func(t *testing.T) {
		t.Parallel()

		for _, versionCase := range []struct {
			name  string
			value string
		}{
			{name: "separator version", value: lkeVersionWithSlash},
			{name: "query version", value: lkeVersionWithQuery},
			{name: "traversal version", value: lkeVersionTraversal},
		} {
			t.Run(versionCase.name, func(t *testing.T) {
				t.Parallel()
				req := createRequestWithArgs(t, map[string]any{keyVersion: versionCase.value})
				result, err := handler(t.Context(), req)
				expectNoError(t, err, "handler should not return Go error")
				expectNotNil(t, result, "handler should return a result")
				checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, "version must be a Kubernetes version ID")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		version := linode.LKEVersion{ID: lkeVersion129}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/versions/1.29", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(version), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEVersionGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyVersion: lkeVersion129})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, lkeVersion129, "response should contain version")
	})
}

// TestLinodeLKETypesListTool verifies the LKE types list tool
// registers correctly and returns available LKE node types.
func TestLinodeLKETypesListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKETypeListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_type_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.LKEType{
			{
				ID: typeG6Standard2, Label: typeLinode4GB, Transfer: 4000,
				Price: linode.LKETypePrice{Hourly: 0.036, Monthly: 24.0},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/types", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: types, keyPage: 1, keyPages: 1, keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKETypeListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, typeG6Standard2, "response should contain type ID")
	})
}

// TestLinodeLKETierVersionsListTool verifies the LKE tier versions list tool
// registers correctly and returns tier version data.
const (
	keyLKETier              = "tier"
	errLKETierInvalidChoice = "tier must be 'standard' or 'enterprise'"
)

func TestLinodeLKETierVersionsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKETierVersionListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_tier_version_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyLKETier, "tier must be marked required")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tierVersions := []linode.LKETierVersion{
			{ID: lkeVersion129, Tier: classStandard},
			{ID: lkeVersion128, Tier: classStandard},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/tiers/standard/versions", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: tierVersions, keyPage: 1, keyPages: 1, keyResults: 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKETierVersionListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLKETier: classStandard})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, classStandard, "response should contain standard tier")
	})

	invalidCases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing tier", args: map[string]any{}, want: "tier is required"},
		{name: "slash tier", args: map[string]any{keyLKETier: "standard/enterprise"}, want: errLKETierInvalidChoice},
		{name: "query tier", args: map[string]any{keyLKETier: "standard?x=1"}, want: errLKETierInvalidChoice},
		{name: "traversal tier", args: map[string]any{keyLKETier: pathTraversalValue}, want: errLKETierInvalidChoice},
	}

	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "invalid tier should return a tool error")
			assertErrorContains(t, result, testCase.want)
		})
	}
}

// End-to-end verification of LKE cluster creation workflow.
func TestLinodeLKEClusterCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_create", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
		expectContainsWithMode(t, false, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, "label", "schema should include label")
		expectContainsWithMode(t, false, props, "region", "schema should include region")
		expectContainsWithMode(t, false, props, "k8s_version", "schema should include k8s_version")
		expectContainsWithMode(t, false, props, "node_pools", "schema should include node_pools")
		expectContainsWithMode(t, false, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingConfirm,
			args:         map[string]any{keyLabel: labelTestCluster, keyRegion: regionUSEast, keyK8sVersion: lkeVersion129, keyNodePools: lkePoolSnapshot},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingLabel,
			args:         map[string]any{keyRegion: regionUSEast, keyK8sVersion: lkeVersion129, keyNodePools: lkePoolSnapshot, keyConfirm: true},
			wantContains: errLabelRequired,
		},
		{
			name:         caseMissingRegion,
			args:         map[string]any{keyLabel: labelTestCluster, keyK8sVersion: lkeVersion129, keyNodePools: lkePoolSnapshot, keyConfirm: true},
			wantContains: errRegionRequired,
		},
		{
			name:         "invalid node pools JSON",
			args:         map[string]any{keyLabel: labelTestCluster, keyRegion: regionUSEast, keyK8sVersion: lkeVersion129, keyNodePools: "not-valid-json", keyConfirm: true},
			wantContains: "invalid node_pools JSON",
		},
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

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		cluster := linode.LKECluster{
			ID: 999, Label: labelTestCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(cluster), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel: labelTestCluster, keyRegion: regionUSEast, keyK8sVersion: lkeVersion129,
			keyNodePools: lkePoolSnapshot, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, labelTestCluster, "response should contain cluster label")
		expectContainsWithMode(t, false, textContent.Text, "999", "response should contain cluster ID")
	})
}

// TestLinodeLKEClusterUpdateTool verifies the LKE cluster update tool
// registers correctly, validates required fields, and updates clusters.
func TestLinodeLKEClusterUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_update", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, "cluster_id", "schema should include cluster_id")
		expectContainsWithMode(t, false, props, "label", "schema should include label")
		expectContainsWithMode(t, false, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123), keyLabel: labelNew}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingClusterID, args: map[string]any{keyLabel: labelNew, keyConfirm: true}, wantContains: errClusterIDRequired},
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

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		cluster := linode.LKECluster{
			ID: 123, Label: "updated-cluster", Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(cluster), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyLabel: "updated-cluster", keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeLKEClusterDeleteTool verifies the LKE cluster delete tool
// registers correctly, validates required fields, and deletes clusters.
func TestLinodeLKEClusterDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
		expectContainsWithMode(t, false, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingClusterID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errClusterIDRequired},
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

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// TestLinodeLKEClusterRecycleTool verifies the LKE cluster recycle tool
// registers correctly, validates confirm, and recycles cluster nodes.
func TestLinodeLKEClusterRecycleTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_recycle", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful recycle", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/recycle", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterRecycleTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "recycle initiated successfully", "response should confirm recycle")
	})
}

// TestLinodeLKEClusterRegenerateTool verifies the LKE cluster regenerate tool
// registers correctly, validates confirm, and regenerates cluster credentials.
func TestLinodeLKEClusterRegenerateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_cluster_regenerate", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful regenerate", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/regenerate", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEClusterRegenerateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "regenerated successfully", "response should confirm regeneration")
	})
}

// End-to-end verification of LKE pool creation workflow.
func TestLinodeLKEPoolCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_pool_create", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, "cluster_id", "schema should include cluster_id")
		expectContainsWithMode(t, false, props, "type", "schema should include type")
		expectContainsWithMode(t, false, props, "count", "schema should include count")
		expectContainsWithMode(t, false, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123), keyType: typeG6Standard2, keyCount: float64(3)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingType, args: map[string]any{keyClusterID: float64(123), keyCount: float64(3), keyConfirm: true}, wantContains: errTypeRequired},
		{name: "missing count", args: map[string]any{keyClusterID: float64(123), keyType: typeG6Standard2, keyConfirm: true}, wantContains: "count is required"},
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

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		pool := linode.LKENodePool{ID: 50, ClusterID: 123, Type: typeG6Standard2, Count: 3}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/pools", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(pool), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEPoolCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123), keyType: typeG6Standard2, keyCount: float64(3), keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, typeG6Standard2, "response should contain pool type")
	})
}

// TestLinodeLKEPoolUpdateTool verifies the LKE pool update tool
// registers correctly, validates confirm, and updates node pools.
func TestLinodeLKEPoolUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_pool_update", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10), keyCount: float64(5)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		pool := linode.LKENodePool{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 5}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/pools/10", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(pool), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEPoolUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123), keyPoolID: float64(10), keyCount: float64(5), keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeLKEPoolDeleteTool verifies the LKE pool delete tool
// registers correctly, validates confirm, and deletes node pools.
func TestLinodeLKEPoolDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_pool_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/pools/10", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEPoolDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "deleted", "response should confirm deletion")
	})
}

// TestLinodeLKEPoolRecycleTool verifies the LKE pool recycle tool
// registers correctly, validates confirm, and recycles pool nodes.
func TestLinodeLKEPoolRecycleTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_pool_recycle", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful recycle", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/pools/10/recycle", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEPoolRecycleTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10), keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "recycle initiated successfully", "response should confirm recycle")
	})
}

// TestLinodeLKENodeDeleteTool verifies the LKE node delete tool
// registers correctly, validates required fields, and deletes nodes.
func TestLinodeLKENodeDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_node_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123}, wantContains: errConfirmEqualsTrue},
		{name: "missing node id", args: map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "node_id is required"},
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

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/nodes/abc-123", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKENodeDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123, keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "deleted", "response should confirm deletion")
		expectContainsWithMode(t, false, textContent.Text, idAbc123, "response should contain node ID")
	})
}

// TestLinodeLKENodeRecycleTool verifies the LKE node recycle tool
// registers correctly, validates confirm, and recycles individual nodes.
func TestLinodeLKENodeRecycleTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_node_recycle", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful recycle", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/nodes/abc-123/recycle", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKENodeRecycleTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123, keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "recycle initiated successfully", "response should confirm recycle")
	})
}

// TestLinodeLKEKubeconfigDeleteTool verifies the LKE kubeconfig delete tool
// registers correctly, validates confirm, and regenerates the kubeconfig.
func TestLinodeLKEKubeconfigDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_kubeconfig_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/kubeconfig", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEKubeconfigDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "regenerated successfully", "response should confirm regeneration")
	})
}

// TestLinodeLKEServiceTokenDeleteTool verifies the LKE service token delete tool
// registers correctly, validates confirm, and regenerates the service token.
func TestLinodeLKEServiceTokenDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_service_token_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	confirmCases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirmRejected, value: false},
		{name: caseStringConfirmRejected, value: boolStringTrue},
		{name: caseNumericConfirmRejected, value: float64(1)},
	}

	for i := range confirmCases {
		confirmCase := confirmCases[i]
		t.Run(confirmCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyClusterID: float64(123)}
			if confirmCase.value != nil {
				args[keyConfirm] = confirmCase.value
			}

			req := createRequestWithArgs(t, args)
			result, err := handler(t.Context(), req)

			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/servicetoken", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEServiceTokenDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "regenerated successfully", "response should confirm regeneration")
	})
}

// TestLinodeLKEACLUpdateTool verifies the LKE ACL update tool
// registers correctly, validates confirm, and updates control plane ACLs.
func TestLinodeLKEACLUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_acl_update", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, "cluster_id", "schema should include cluster_id")
		expectContainsWithMode(t, false, props, statusEnabled, "schema should include enabled")
		expectContainsWithMode(t, false, props, keyIPv4, "schema should include ipv4")
		expectContainsWithMode(t, false, props, "ipv6", "schema should include ipv6")
		expectContainsWithMode(t, false, props, "confirm", "schema should include confirm")
	})

	t.Run("rejects non-true confirm before client call", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissingConfirm},
			{name: caseFalseConfirm, confirm: false, include: true},
			{name: caseStringConfirm, confirm: boolStringTrue, include: true},
			{name: caseNumericConfirm, confirm: 1, include: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				called := make(chan struct{}, 1)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called <- struct{}{}

					w.WriteHeader(http.StatusTeapot)
				}))
				t.Cleanup(srv.Close)

				srvCfg := &config.Config{
					Environments: map[string]config.EnvironmentConfig{
						envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
					},
				}
				_, _, srvHandler := tools.NewLinodeLKEACLUpdateTool(srvCfg)

				args := map[string]any{keyClusterID: float64(123), statusEnabled: true}
				if tt.include {
					args[keyConfirm] = tt.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := srvHandler(t.Context(), req)
				expectNoError(t, err, "handler should not return Go error")
				expectNotNil(t, result, "handler should return a result")
				checkTrueWithMode(t, false, result.IsError, "result should be a tool error")

				select {
				case <-called:
					t.Error("handler should reject confirm before client call")
				default:
				}

				assertErrorContains(t, result, errConfirmEqualsTrue)
			})
		}
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		acl := linode.LKEControlPlaneACL{
			Enabled: true,
			Addresses: linode.LKEControlPlaneACLAddresses{
				IPv4: []string{"10.0.0.1/32", "192.168.1.0/24"},
				IPv6: []string{cidrV6},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/control_plane_acl", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")

			var got linode.UpdateLKEControlPlaneACLRequest
			checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			checkEqual(t, acl, got.ACL, "request body should match ACL payload")

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(acl), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEACLUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123), statusEnabled: true,
			keyIPv4: "10.0.0.1/32, 192.168.1.0/24", "ipv6": cidrV6, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeLKEACLDeleteTool verifies the LKE ACL delete tool
// registers correctly, validates confirm, and deletes control plane ACLs.
func TestLinodeLKEACLDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_acl_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/lke/clusters/123/control_plane_acl", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEACLDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// Dry-run coverage for LKE cluster delete. Sibling function keeps the
// main test's subtest count below maintidx's threshold.
func TestLinodeLKEClusterDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterDeleteTool(&config.Config{})
		expectContainsWithMode(t, false, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		clusterBody := `{"id":123,"label":"prod-cluster","region":"us-east","k8s_version":"1.29","status":"ready"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)

			if r.Method != http.MethodGet {
				t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == lkeClusterGetPath {
				_, _ = w.Write([]byte(clusterBody))

				return
			}

			// The Tier A walk also lists node pools; an empty page keeps
			// this subtest on the no-mutation and preview-shape contract.
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, true, body[keyDryRun])
		checkEqual(t, "linode_lke_cluster_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		expectTrue(t, isWouldObject)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, "/lke/clusters/123", would["path"])

		expectNotEmpty(t, methodsSeen, "dry_run must read state")
		expectNotContains(t, methodsSeen, http.MethodDelete,
			"dry_run must never issue a DELETE")
	})

	t.Run("still validates cluster_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEClusterDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{keyDryRun: true})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		checkTrueWithMode(t, false, result.IsError)
		assertErrorContains(t, result, errClusterIDRequired)
	})
}

// Dry-run coverage for LKE pool delete via the ByTwoIDs helper.
func TestLinodeLKEPoolDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolDeleteTool(&config.Config{})
		expectContainsWithMode(t, false, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		poolBody := `{"id":10,"count":3,"type":"g6-standard-2","nodes":[]}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			checkEqual(t, "/lke/clusters/123/pools/10", r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(poolBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyPoolID:    float64(10),
			keyDryRun:    true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, "linode_lke_pool_delete", body["tool"])
		would, wouldOK := body["would_execute"].(map[string]any)
		expectTrue(t, wouldOK)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, "/lke/clusters/123/pools/10", would["path"])

		checkEqual(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never DELETE")
	})

	t.Run("still validates cluster_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEPoolDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyPoolID: float64(10),
			keyDryRun: true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		checkTrueWithMode(t, false, result.IsError)
		assertErrorContains(t, result, errClusterIDRequired)
	})
}

// Dry-run coverage for LKE node delete (mixed int + string IDs).
func TestLinodeLKENodeDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKENodeDeleteTool(&config.Config{})
		expectContainsWithMode(t, false, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		nodeBody := `{"id":"123-abc","instance_id":456,"status":"ready"}`
		expectedPath := "/lke/clusters/123/nodes/123-abc"

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			checkEqual(t, expectedPath, r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(nodeBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyNodeID:    "123-abc",
			keyDryRun:    true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, "linode_lke_node_delete", body["tool"])
		would, wouldOK := body["would_execute"].(map[string]any)
		expectTrue(t, wouldOK)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, expectedPath, would["path"])

		checkEqual(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never DELETE")
	})

	t.Run("still validates node_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKENodeDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		checkTrueWithMode(t, false, result.IsError)
		assertErrorContains(t, result, "node_id is required")
	})
}

// Dry-run coverage for LKE kubeconfig delete. The fetch returns the
// CLUSTER state (not the kubeconfig contents) so dry-run never surfaces
// credential material to the model. Locks that design choice.
func TestLinodeLKEKubeconfigDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEKubeconfigDeleteTool(&config.Config{})
		expectContainsWithMode(t, false, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview fetches cluster, never kubeconfig", func(t *testing.T) {
		t.Parallel()

		var pathsSeen []string

		clusterBody := `{"id":123,"label":"prod-cluster","region":"us-east"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

			if r.Method == http.MethodGet && r.URL.Path == "/lke/clusters/123" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(clusterBody))

				return
			}

			t.Errorf("dry_run must only GET cluster metadata, never touch kubeconfig; got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, "linode_lke_kubeconfig_delete", body["tool"])
		would, wouldOK := body["would_execute"].(map[string]any)
		expectTrue(t, wouldOK)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, "/lke/clusters/123/kubeconfig", would["path"],
			"would_execute.path must point at kubeconfig sub-resource")

		checkEqual(t, []string{"GET /lke/clusters/123"}, pathsSeen,
			"dry_run must only fetch cluster metadata, never the kubeconfig itself")

		state, stateOK := body["current_state"].(map[string]any)
		expectTrue(t, stateOK)
		expectNotContains(t, state, "kubeconfig",
			"current_state must NOT include kubeconfig credential material")
	})
}

// Dry-run coverage for LKE service token delete. Same safety design as
// kubeconfig delete: fetch the cluster, not the token.
func TestLinodeLKEServiceTokenDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEServiceTokenDeleteTool(&config.Config{})
		expectContainsWithMode(t, false, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview fetches cluster, never service token", func(t *testing.T) {
		t.Parallel()

		var pathsSeen []string

		clusterBody := `{"id":123,"label":"prod-cluster"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

			if r.Method == http.MethodGet && r.URL.Path == "/lke/clusters/123" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(clusterBody))

				return
			}

			t.Errorf("dry_run must only GET cluster metadata; got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, "linode_lke_service_token_delete", body["tool"])
		would, wouldOK := body["would_execute"].(map[string]any)
		expectTrue(t, wouldOK)
		checkEqual(t, "/lke/clusters/123/servicetoken", would["path"])

		checkEqual(t, []string{"GET /lke/clusters/123"}, pathsSeen,
			"dry_run must only fetch cluster metadata, never the service token itself")
	})
}

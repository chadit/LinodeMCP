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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestLinodeLKEClustersListTool verifies the LKE clusters list tool
// registers correctly, returns cluster data, and supports label filtering.
//
// Workflow:
//  1. Definition: Verify tool name, description, and handler
//  2. Success: List clusters through mock API and verify response
//  3. FilterByLabel: Filter clusters by label substring
//
// Expected Behavior:
//   - Tool registers as "linode_lke_clusters_list" with a valid handler
//   - Successful list returns all cluster names in the response
//   - Label filter returns only matching clusters
//
// Purpose: End-to-end verification of LKE cluster listing and filtering.
func TestLinodeLKEClustersListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClustersListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_clusters_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		clusters := []linode.LKECluster{
			{ID: 1, Label: "prod-cluster", Region: "us-east", K8sVersion: "1.29", Status: "ready"},
			{ID: 2, Label: "dev-cluster", Region: "eu-west", K8sVersion: "1.28", Status: "ready"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    clusters,
				"page":    1,
				"pages":   1,
				"results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClustersListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "prod-cluster", "response should contain prod-cluster")
		assert.Contains(t, textContent.Text, "dev-cluster", "response should contain dev-cluster")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		clusters := []linode.LKECluster{
			{ID: 1, Label: "prod-cluster", Region: "us-east", K8sVersion: "1.29", Status: "ready"},
			{ID: 2, Label: "dev-cluster", Region: "eu-west", K8sVersion: "1.28", Status: "ready"},
			{ID: 3, Label: "staging-prod", Region: "us-west", K8sVersion: "1.29", Status: "ready"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    clusters,
				"page":    1,
				"pages":   1,
				"results": 3,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClustersListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"label": "prod"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "prod-cluster", "response should contain prod-cluster")
		assert.Contains(t, textContent.Text, "staging-prod", "response should contain staging-prod")
		assert.NotContains(t, textContent.Text, "dev-cluster", "response should not contain dev-cluster")
	})
}

// TestLinodeLKEClusterGetTool verifies the LKE cluster get tool
// registers correctly, validates required fields, and retrieves cluster details.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing or invalid cluster_id produces clear errors
//  3. Success: Get cluster through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_lke_cluster_get" with required params
//   - Missing cluster_id returns descriptive error
//   - Invalid cluster_id returns descriptive error
//   - Successful get returns cluster details from API
//
// Purpose: End-to-end verification of LKE cluster get workflow.
func TestLinodeLKEClusterGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_cluster_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing cluster id", args: map[string]any{}, wantContains: "cluster_id is required"},
		{name: "invalid cluster id", args: map[string]any{"cluster_id": "not-a-number"}, wantContains: "cluster_id must be a valid integer"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cluster := linode.LKECluster{
			ID: 123, Label: "prod-cluster", Region: "us-east", K8sVersion: "1.29", Status: "ready",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(cluster), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClusterGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "prod-cluster", "response should contain cluster label")
		assert.Contains(t, textContent.Text, "1.29", "response should contain k8s version")
	})
}

// TestLinodeLKEPoolsListTool verifies the LKE pools list tool
// registers correctly, validates cluster_id, and returns pool data.
func TestLinodeLKEPoolsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEPoolsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_pools_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing cluster id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "cluster_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		pools := []linode.LKENodePool{
			{ID: 10, ClusterID: 123, Type: "g6-standard-2", Count: 3},
			{ID: 11, ClusterID: 123, Type: "g6-standard-4", Count: 2},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/pools", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": pools, "page": 1, "pages": 1, "results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEPoolsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "g6-standard-2", "response should contain pool type")
		assert.Contains(t, textContent.Text, "g6-standard-4", "response should contain pool type")
		assert.Contains(t, textContent.Text, `"count": 2`, "response should contain pool count")
	})
}

// TestLinodeLKEPoolGetTool verifies the LKE pool get tool
// registers correctly, validates required fields, and retrieves pool details.
func TestLinodeLKEPoolGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_pool_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing cluster id", args: map[string]any{"pool_id": "10"}, wantContains: "cluster_id is required"},
		{name: "missing pool id", args: map[string]any{"cluster_id": "123"}, wantContains: "pool_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		pool := linode.LKENodePool{ID: 10, ClusterID: 123, Type: "g6-standard-2", Count: 3}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/pools/10", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(pool), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEPoolGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123", "pool_id": "10"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "g6-standard-2", "response should contain pool type")
	})
}

// TestLinodeLKENodeGetTool verifies the LKE node get tool
// registers correctly, validates required fields, and retrieves node details.
func TestLinodeLKENodeGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKENodeGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_node_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing cluster id", args: map[string]any{"node_id": "abc-123"}, wantContains: "cluster_id is required"},
		{name: "missing node id", args: map[string]any{"cluster_id": "123"}, wantContains: "node_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		node := linode.LKENode{ID: "abc-123", InstanceID: 456, Status: "ready"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/nodes/abc-123", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(node), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKENodeGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123", "node_id": "abc-123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "abc-123", "response should contain node ID")
		assert.Contains(t, textContent.Text, "ready", "response should contain node status")
	})
}

// TestLinodeLKEKubeconfigGetTool verifies the LKE kubeconfig get tool
// registers correctly, validates cluster_id, and returns kubeconfig data.
func TestLinodeLKEKubeconfigGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_kubeconfig_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing cluster id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "cluster_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		kubeconfig := linode.LKEKubeconfig{
			Kubeconfig: "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/kubeconfig", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(kubeconfig), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEKubeconfigGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==", "response should contain kubeconfig data")
	})
}

// TestLinodeLKEDashboardGetTool verifies the LKE dashboard get tool
// registers correctly and returns the dashboard URL.
func TestLinodeLKEDashboardGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEDashboardGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_dashboard_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		dashboard := linode.LKEDashboard{URL: "https://dashboard.lke.example.com"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/dashboard", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(dashboard), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEDashboardGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "https://dashboard.lke.example.com", "response should contain dashboard URL")
	})
}

// TestLinodeLKEAPIEndpointsListTool verifies the LKE API endpoints list tool
// registers correctly and returns available API endpoints.
func TestLinodeLKEAPIEndpointsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEAPIEndpointsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_api_endpoints_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		endpoints := []linode.LKEAPIEndpoint{
			{Endpoint: "https://abc123.us-east.lke.example.com:443"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/api-endpoints", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": endpoints, "page": 1, "pages": 1, "results": 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEAPIEndpointsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "abc123.us-east.lke.example.com", "response should contain API endpoint")
	})
}

// TestLinodeLKEACLGetTool verifies the LKE ACL get tool
// registers correctly and returns control plane ACL data.
func TestLinodeLKEACLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEACLGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_acl_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		acl := linode.LKEControlPlaneACL{
			Enabled: true,
			Addresses: linode.LKEControlPlaneACLAddresses{
				IPv4: []string{"10.0.0.1/32"},
				IPv6: []string{"2001:db8::1/128"},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/control-plane-acl", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(acl), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEACLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "10.0.0.1/32", "response should contain IPv4 address")
		assert.Contains(t, textContent.Text, "2001:db8::1/128", "response should contain IPv6 address")
	})
}

// TestLinodeLKEVersionsListTool verifies the LKE versions list tool
// registers correctly and returns available Kubernetes versions.
func TestLinodeLKEVersionsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEVersionsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_versions_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		versions := []linode.LKEVersion{{ID: "1.29"}, {ID: "1.28"}, {ID: "1.27"}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/versions", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": versions, "page": 1, "pages": 1, "results": 3,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEVersionsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "1.29", "response should contain version 1.29")
		assert.Contains(t, textContent.Text, "1.28", "response should contain version 1.28")
	})
}

// TestLinodeLKEVersionGetTool verifies the LKE version get tool
// registers correctly, validates the version parameter, and returns version details.
func TestLinodeLKEVersionGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_version_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing version", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "version is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		version := linode.LKEVersion{ID: "1.29"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/versions/1.29", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(version), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEVersionGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"version": "1.29"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "1.29", "response should contain version")
	})
}

// TestLinodeLKETypesListTool verifies the LKE types list tool
// registers correctly and returns available LKE node types.
func TestLinodeLKETypesListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKETypesListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_types_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.LKEType{
			{
				ID: "g6-standard-2", Label: "Linode 4GB", Transfer: 4000,
				Price: linode.LKETypePrice{Hourly: 0.036, Monthly: 24.0},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/types", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": types, "page": 1, "pages": 1, "results": 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKETypesListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "g6-standard-2", "response should contain type ID")
	})
}

// TestLinodeLKETierVersionsListTool verifies the LKE tier versions list tool
// registers correctly and returns tier version data.
func TestLinodeLKETierVersionsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKETierVersionsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_tier_versions_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tierVersions := []linode.LKETierVersion{
			{ID: "1.29", Tier: "standard"},
			{ID: "1.28", Tier: "enterprise"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/tiers/versions", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": tierVersions, "page": 1, "pages": 1, "results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKETierVersionsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "standard", "response should contain standard tier")
		assert.Contains(t, textContent.Text, "enterprise", "response should contain enterprise tier")
	})
}

// TestLinodeLKEClusterCreateTool verifies the LKE cluster creation tool
// registers correctly, validates required fields, and creates clusters.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing required fields produce clear errors
//  3. Success: Create cluster through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_lke_cluster_create" with required params
//   - Missing confirm, label, region, or invalid node_pools returns descriptive error
//   - Successful creation returns cluster details from API
//
// Purpose: End-to-end verification of LKE cluster creation workflow.
func TestLinodeLKEClusterCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_cluster_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "region", "schema should include region")
		assert.Contains(t, props, "k8s_version", "schema should include k8s_version")
		assert.Contains(t, props, "node_pools", "schema should include node_pools")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         "missing confirm",
			args:         map[string]any{"label": "test-cluster", "region": "us-east", "k8s_version": "1.29", "node_pools": `[{"type":"g6-standard-2","count":3}]`},
			wantContains: "confirm=true",
		},
		{
			name:         "missing label",
			args:         map[string]any{"region": "us-east", "k8s_version": "1.29", "node_pools": `[{"type":"g6-standard-2","count":3}]`, "confirm": true},
			wantContains: "label is required",
		},
		{
			name:         "missing region",
			args:         map[string]any{"label": "test-cluster", "k8s_version": "1.29", "node_pools": `[{"type":"g6-standard-2","count":3}]`, "confirm": true},
			wantContains: "region is required",
		},
		{
			name:         "invalid node pools JSON",
			args:         map[string]any{"label": "test-cluster", "region": "us-east", "k8s_version": "1.29", "node_pools": "not-valid-json", "confirm": true},
			wantContains: "invalid node_pools JSON",
		},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		cluster := linode.LKECluster{
			ID: 999, Label: "test-cluster", Region: "us-east", K8sVersion: "1.29", Status: "ready",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(cluster), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClusterCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label": "test-cluster", "region": "us-east", "k8s_version": "1.29",
			"node_pools": `[{"type":"g6-standard-2","count":3}]`, "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "test-cluster", "response should contain cluster label")
		assert.Contains(t, textContent.Text, "999", "response should contain cluster ID")
	})
}

// TestLinodeLKEClusterUpdateTool verifies the LKE cluster update tool
// registers correctly, validates required fields, and updates clusters.
func TestLinodeLKEClusterUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_cluster_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "cluster_id", "schema should include cluster_id")
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"cluster_id": float64(123), "label": "new-label"}, wantContains: "confirm=true"},
		{name: "missing cluster id", args: map[string]any{"label": "new-label", "confirm": true}, wantContains: "cluster_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		cluster := linode.LKECluster{
			ID: 123, Label: "updated-cluster", Region: "us-east", K8sVersion: "1.29", Status: "ready",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(cluster), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClusterUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "label": "updated-cluster", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeLKEClusterDeleteTool verifies the LKE cluster delete tool
// registers correctly, validates required fields, and deletes clusters.
func TestLinodeLKEClusterDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_cluster_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"cluster_id": float64(123)}, wantContains: "confirm=true"},
		{name: "missing cluster id", args: map[string]any{"confirm": true}, wantContains: "cluster_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClusterDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// TestLinodeLKEClusterRecycleTool verifies the LKE cluster recycle tool
// registers correctly, validates confirm, and recycles cluster nodes.
func TestLinodeLKEClusterRecycleTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_cluster_recycle", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful recycle", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/recycle", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClusterRecycleTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "recycle initiated successfully", "response should confirm recycle")
	})
}

// TestLinodeLKEClusterRegenerateTool verifies the LKE cluster regenerate tool
// registers correctly, validates confirm, and regenerates cluster credentials.
func TestLinodeLKEClusterRegenerateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_cluster_regenerate", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful regenerate", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/regenerate", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEClusterRegenerateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "regenerated successfully", "response should confirm regeneration")
	})
}

// TestLinodeLKEPoolCreateTool verifies the LKE pool creation tool
// registers correctly, validates required fields, and creates node pools.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, type, or count returns descriptive error
//  3. Success: Create pool through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_lke_pool_create" with required params
//   - Missing required fields return descriptive errors
//   - Successful creation returns pool details from API
//
// Purpose: End-to-end verification of LKE pool creation workflow.
func TestLinodeLKEPoolCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_pool_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "cluster_id", "schema should include cluster_id")
		assert.Contains(t, props, "type", "schema should include type")
		assert.Contains(t, props, "count", "schema should include count")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"cluster_id": float64(123), "type": "g6-standard-2", "count": float64(3)}, wantContains: "confirm=true"},
		{name: "missing type", args: map[string]any{"cluster_id": float64(123), "count": float64(3), "confirm": true}, wantContains: "type is required"},
		{name: "missing count", args: map[string]any{"cluster_id": float64(123), "type": "g6-standard-2", "confirm": true}, wantContains: "count is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		pool := linode.LKENodePool{ID: 50, ClusterID: 123, Type: "g6-standard-2", Count: 3}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/pools", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(pool), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEPoolCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"cluster_id": float64(123), "type": "g6-standard-2", "count": float64(3), "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "g6-standard-2", "response should contain pool type")
	})
}

// TestLinodeLKEPoolUpdateTool verifies the LKE pool update tool
// registers correctly, validates confirm, and updates node pools.
func TestLinodeLKEPoolUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_pool_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "pool_id": float64(10), "count": float64(5)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		pool := linode.LKENodePool{ID: 10, ClusterID: 123, Type: "g6-standard-2", Count: 5}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/pools/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(pool), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEPoolUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"cluster_id": float64(123), "pool_id": float64(10), "count": float64(5), "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeLKEPoolDeleteTool verifies the LKE pool delete tool
// registers correctly, validates confirm, and deletes node pools.
func TestLinodeLKEPoolDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_pool_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "pool_id": float64(10)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/pools/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEPoolDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "pool_id": float64(10), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted", "response should confirm deletion")
	})
}

// TestLinodeLKEPoolRecycleTool verifies the LKE pool recycle tool
// registers correctly, validates confirm, and recycles pool nodes.
func TestLinodeLKEPoolRecycleTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_pool_recycle", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "pool_id": float64(10)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful recycle", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/pools/10/recycle", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEPoolRecycleTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "pool_id": float64(10), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "recycle initiated successfully", "response should confirm recycle")
	})
}

// TestLinodeLKENodeDeleteTool verifies the LKE node delete tool
// registers correctly, validates required fields, and deletes nodes.
func TestLinodeLKENodeDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_node_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"cluster_id": float64(123), "node_id": "abc-123"}, wantContains: "confirm=true"},
		{name: "missing node id", args: map[string]any{"cluster_id": float64(123), "confirm": true}, wantContains: "node_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/nodes/abc-123", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKENodeDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "node_id": "abc-123", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted", "response should confirm deletion")
		assert.Contains(t, textContent.Text, "abc-123", "response should contain node ID")
	})
}

// TestLinodeLKENodeRecycleTool verifies the LKE node recycle tool
// registers correctly, validates confirm, and recycles individual nodes.
func TestLinodeLKENodeRecycleTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_node_recycle", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "node_id": "abc-123"})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful recycle", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/nodes/abc-123/recycle", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKENodeRecycleTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "node_id": "abc-123", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "recycle initiated successfully", "response should confirm recycle")
	})
}

// TestLinodeLKEKubeconfigDeleteTool verifies the LKE kubeconfig delete tool
// registers correctly, validates confirm, and regenerates the kubeconfig.
func TestLinodeLKEKubeconfigDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_kubeconfig_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/kubeconfig", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEKubeconfigDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "regenerated successfully", "response should confirm regeneration")
	})
}

// TestLinodeLKEServiceTokenDeleteTool verifies the LKE service token delete tool
// registers correctly, validates confirm, and regenerates the service token.
func TestLinodeLKEServiceTokenDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_service_token_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/service-token", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEServiceTokenDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "regenerated successfully", "response should confirm regeneration")
	})
}

// TestLinodeLKEACLUpdateTool verifies the LKE ACL update tool
// registers correctly, validates confirm, and updates control plane ACLs.
func TestLinodeLKEACLUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_acl_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "cluster_id", "schema should include cluster_id")
		assert.Contains(t, props, "enabled", "schema should include enabled")
		assert.Contains(t, props, "ipv4", "schema should include ipv4")
		assert.Contains(t, props, "ipv6", "schema should include ipv6")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "enabled": true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		acl := linode.LKEControlPlaneACL{
			Enabled: true,
			Addresses: linode.LKEControlPlaneACLAddresses{
				IPv4: []string{"10.0.0.1/32", "192.168.1.0/24"},
				IPv6: []string{"2001:db8::1/128"},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/control-plane-acl", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(acl), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEACLUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"cluster_id": float64(123), "enabled": true,
			"ipv4": "10.0.0.1/32, 192.168.1.0/24", "ipv6": "2001:db8::1/128", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeLKEACLDeleteTool verifies the LKE ACL delete tool
// registers correctly, validates confirm, and deletes control plane ACLs.
func TestLinodeLKEACLDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_lke_acl_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/lke/clusters/123/control-plane-acl", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeLKEACLDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

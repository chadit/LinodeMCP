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

// =============================================================================
// LKE Clusters List Tool Tests
// =============================================================================

func TestNewLinodeLKEClustersListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClustersListTool(cfg)

	assert.Equal(t, "linode_lke_clusters_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEClustersListTool_Success(t *testing.T) {
	t.Parallel()

	clusters := []linode.LKECluster{
		{ID: 1, Label: "prod-cluster", Region: "us-east", K8sVersion: "1.29", Status: "ready"},
		{ID: 2, Label: "dev-cluster", Region: "eu-west", K8sVersion: "1.28", Status: "ready"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    clusters,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClustersListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "prod-cluster")
	assert.Contains(t, textContent.Text, "dev-cluster")
}

func TestLinodeLKEClustersListTool_FilterByLabel(t *testing.T) {
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
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClustersListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "prod"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "prod-cluster")
	assert.Contains(t, textContent.Text, "staging-prod")
	assert.NotContains(t, textContent.Text, "dev-cluster")
}

// =============================================================================
// LKE Cluster Get Tool Tests
// =============================================================================

func TestNewLinodeLKEClusterGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	assert.Equal(t, "linode_lke_cluster_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEClusterGetTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKEClusterGetTool_InvalidClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "not-a-number"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id must be a valid integer")
}

func TestLinodeLKEClusterGetTool_Success(t *testing.T) {
	t.Parallel()

	cluster := linode.LKECluster{
		ID:         123,
		Label:      "prod-cluster",
		Region:     "us-east",
		K8sVersion: "1.29",
		Status:     "ready",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(cluster))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "prod-cluster")
	assert.Contains(t, textContent.Text, "1.29")
}

// =============================================================================
// LKE Pools List Tool Tests
// =============================================================================

func TestNewLinodeLKEPoolsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEPoolsListTool(cfg)

	assert.Equal(t, "linode_lke_pools_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEPoolsListTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKEPoolsListTool_Success(t *testing.T) {
	t.Parallel()

	pools := []linode.LKENodePool{
		{ID: 10, ClusterID: 123, Type: "g6-standard-2", Count: 3},
		{ID: 11, ClusterID: 123, Type: "g6-standard-4", Count: 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/pools", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    pools,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "g6-standard-2")
	assert.Contains(t, textContent.Text, "g6-standard-4")
	assert.Contains(t, textContent.Text, `"count": 2`)
}

// =============================================================================
// LKE Pool Get Tool Tests
// =============================================================================

func TestNewLinodeLKEPoolGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	assert.Equal(t, "linode_lke_pool_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEPoolGetTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"pool_id": "10"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKEPoolGetTool_MissingPoolID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "pool_id is required")
}

func TestLinodeLKEPoolGetTool_Success(t *testing.T) {
	t.Parallel()

	pool := linode.LKENodePool{
		ID:        10,
		ClusterID: 123,
		Type:      "g6-standard-2",
		Count:     3,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/pools/10", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(pool))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123", "pool_id": "10"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "g6-standard-2")
}

// =============================================================================
// LKE Node Get Tool Tests
// =============================================================================

func TestNewLinodeLKENodeGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKENodeGetTool(cfg)

	assert.Equal(t, "linode_lke_node_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKENodeGetTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"node_id": "abc-123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKENodeGetTool_MissingNodeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "node_id is required")
}

func TestLinodeLKENodeGetTool_Success(t *testing.T) {
	t.Parallel()

	node := linode.LKENode{
		ID:         "abc-123",
		InstanceID: 456,
		Status:     "ready",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/nodes/abc-123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(node))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123", "node_id": "abc-123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "abc-123")
	assert.Contains(t, textContent.Text, "ready")
}

// =============================================================================
// LKE Kubeconfig Get Tool Tests
// =============================================================================

func TestNewLinodeLKEKubeconfigGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	assert.Equal(t, "linode_lke_kubeconfig_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEKubeconfigGetTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKEKubeconfigGetTool_Success(t *testing.T) {
	t.Parallel()

	kubeconfig := linode.LKEKubeconfig{
		Kubeconfig: "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/kubeconfig", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(kubeconfig))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==")
}

// =============================================================================
// LKE Dashboard Get Tool Tests
// =============================================================================

func TestNewLinodeLKEDashboardGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEDashboardGetTool(cfg)

	assert.Equal(t, "linode_lke_dashboard_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEDashboardGetTool_Success(t *testing.T) {
	t.Parallel()

	dashboard := linode.LKEDashboard{
		URL: "https://dashboard.lke.example.com",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/dashboard", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(dashboard))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEDashboardGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "https://dashboard.lke.example.com")
}

// =============================================================================
// LKE API Endpoints List Tool Tests
// =============================================================================

func TestNewLinodeLKEAPIEndpointsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEAPIEndpointsListTool(cfg)

	assert.Equal(t, "linode_lke_api_endpoints_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEAPIEndpointsListTool_Success(t *testing.T) {
	t.Parallel()

	endpoints := []linode.LKEAPIEndpoint{
		{Endpoint: "https://abc123.us-east.lke.example.com:443"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/api-endpoints", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    endpoints,
			"page":    1,
			"pages":   1,
			"results": 1,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEAPIEndpointsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "abc123.us-east.lke.example.com")
}

// =============================================================================
// LKE ACL Get Tool Tests
// =============================================================================

func TestNewLinodeLKEACLGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEACLGetTool(cfg)

	assert.Equal(t, "linode_lke_acl_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEACLGetTool_Success(t *testing.T) {
	t.Parallel()

	acl := linode.LKEControlPlaneACL{
		Enabled: true,
		Addresses: linode.LKEControlPlaneACLAddresses{
			IPv4: []string{"10.0.0.1/32"},
			IPv6: []string{"2001:db8::1/128"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/control-plane-acl", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(acl))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEACLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "10.0.0.1/32")
	assert.Contains(t, textContent.Text, "2001:db8::1/128")
}

// =============================================================================
// LKE Versions List Tool Tests
// =============================================================================

func TestNewLinodeLKEVersionsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEVersionsListTool(cfg)

	assert.Equal(t, "linode_lke_versions_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEVersionsListTool_Success(t *testing.T) {
	t.Parallel()

	versions := []linode.LKEVersion{
		{ID: "1.29"},
		{ID: "1.28"},
		{ID: "1.27"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/versions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    versions,
			"page":    1,
			"pages":   1,
			"results": 3,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEVersionsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "1.29")
	assert.Contains(t, textContent.Text, "1.28")
}

// =============================================================================
// LKE Version Get Tool Tests
// =============================================================================

func TestNewLinodeLKEVersionGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	assert.Equal(t, "linode_lke_version_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEVersionGetTool_MissingVersion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "version is required")
}

func TestLinodeLKEVersionGetTool_Success(t *testing.T) {
	t.Parallel()

	version := linode.LKEVersion{ID: "1.29"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/versions/1.29", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(version))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"version": "1.29"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "1.29")
}

// =============================================================================
// LKE Types List Tool Tests
// =============================================================================

func TestNewLinodeLKETypesListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKETypesListTool(cfg)

	assert.Equal(t, "linode_lke_types_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKETypesListTool_Success(t *testing.T) {
	t.Parallel()

	types := []linode.LKEType{
		{
			ID: "g6-standard-2", Label: "Linode 4GB", Transfer: 4000,
			Price: linode.LKETypePrice{Hourly: 0.036, Monthly: 24.0},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/types", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    types,
			"page":    1,
			"pages":   1,
			"results": 1,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKETypesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "g6-standard-2")
}

// =============================================================================
// LKE Tier Versions List Tool Tests
// =============================================================================

func TestNewLinodeLKETierVersionsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKETierVersionsListTool(cfg)

	assert.Equal(t, "linode_lke_tier_versions_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKETierVersionsListTool_Success(t *testing.T) {
	t.Parallel()

	tierVersions := []linode.LKETierVersion{
		{ID: "1.29", Tier: "standard"},
		{ID: "1.28", Tier: "enterprise"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/tiers/versions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    tierVersions,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKETierVersionsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "standard")
	assert.Contains(t, textContent.Text, "enterprise")
}

// =============================================================================
// LKE Cluster Create Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEClusterCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	assert.Equal(t, "linode_lke_cluster_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "k8s_version")
	assert.Contains(t, props, "node_pools")
	assert.Contains(t, props, "confirm")
}

func TestLinodeLKEClusterCreateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":       "test-cluster",
		"region":      "us-east",
		"k8s_version": "1.29",
		"node_pools":  `[{"type":"g6-standard-2","count":3}]`,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should require confirm=true")
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEClusterCreateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":      "us-east",
		"k8s_version": "1.29",
		"node_pools":  `[{"type":"g6-standard-2","count":3}]`,
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeLKEClusterCreateTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":       "test-cluster",
		"k8s_version": "1.29",
		"node_pools":  `[{"type":"g6-standard-2","count":3}]`,
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "region is required")
}

func TestLinodeLKEClusterCreateTool_InvalidNodePoolsJSON(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":       "test-cluster",
		"region":      "us-east",
		"k8s_version": "1.29",
		"node_pools":  "not-valid-json",
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "invalid node_pools JSON")
}

func TestLinodeLKEClusterCreateTool_Success(t *testing.T) {
	t.Parallel()

	cluster := linode.LKECluster{
		ID:         999,
		Label:      "test-cluster",
		Region:     "us-east",
		K8sVersion: "1.29",
		Status:     "ready",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(cluster))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":       "test-cluster",
		"region":      "us-east",
		"k8s_version": "1.29",
		"node_pools":  `[{"type":"g6-standard-2","count":3}]`,
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "test-cluster")
	assert.Contains(t, textContent.Text, "999")
}

// =============================================================================
// LKE Cluster Update Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEClusterUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	assert.Equal(t, "linode_lke_cluster_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "cluster_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "confirm")
}

func TestLinodeLKEClusterUpdateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"label":      "new-label",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEClusterUpdateTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "new-label",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKEClusterUpdateTool_Success(t *testing.T) {
	t.Parallel()

	cluster := linode.LKECluster{
		ID:         123,
		Label:      "updated-cluster",
		Region:     "us-east",
		K8sVersion: "1.29",
		Status:     "ready",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(cluster))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"label":      "updated-cluster",
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

// =============================================================================
// LKE Cluster Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEClusterDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	assert.Equal(t, "linode_lke_cluster_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")
}

func TestLinodeLKEClusterDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEClusterDeleteTool_MissingClusterID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"confirm": true})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "cluster_id is required")
}

func TestLinodeLKEClusterDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// LKE Cluster Recycle Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEClusterRecycleTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	assert.Equal(t, "linode_lke_cluster_recycle", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEClusterRecycleTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEClusterRecycleTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/recycle", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "recycle initiated successfully")
}

// =============================================================================
// LKE Cluster Regenerate Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEClusterRegenerateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	assert.Equal(t, "linode_lke_cluster_regenerate", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEClusterRegenerateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEClusterRegenerateTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/regenerate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "regenerated successfully")
}

// =============================================================================
// LKE Pool Create Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEPoolCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	assert.Equal(t, "linode_lke_pool_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "cluster_id")
	assert.Contains(t, props, "type")
	assert.Contains(t, props, "count")
	assert.Contains(t, props, "confirm")
}

func TestLinodeLKEPoolCreateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"type":       "g6-standard-2",
		"count":      float64(3),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEPoolCreateTool_MissingType(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"count":      float64(3),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "type is required")
}

func TestLinodeLKEPoolCreateTool_MissingCount(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"type":       "g6-standard-2",
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "count is required")
}

func TestLinodeLKEPoolCreateTool_Success(t *testing.T) {
	t.Parallel()

	pool := linode.LKENodePool{
		ID:        50,
		ClusterID: 123,
		Type:      "g6-standard-2",
		Count:     3,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/pools", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(pool))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"type":       "g6-standard-2",
		"count":      float64(3),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "g6-standard-2")
}

// =============================================================================
// LKE Pool Update Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEPoolUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	assert.Equal(t, "linode_lke_pool_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEPoolUpdateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"pool_id":    float64(10),
		"count":      float64(5),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEPoolUpdateTool_Success(t *testing.T) {
	t.Parallel()

	pool := linode.LKENodePool{
		ID:        10,
		ClusterID: 123,
		Type:      "g6-standard-2",
		Count:     5,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/pools/10", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(pool))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"pool_id":    float64(10),
		"count":      float64(5),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

// =============================================================================
// LKE Pool Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEPoolDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	assert.Equal(t, "linode_lke_pool_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEPoolDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"pool_id":    float64(10),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEPoolDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/pools/10", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"pool_id":    float64(10),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted")
}

// =============================================================================
// LKE Pool Recycle Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEPoolRecycleTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	assert.Equal(t, "linode_lke_pool_recycle", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEPoolRecycleTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"pool_id":    float64(10),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEPoolRecycleTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/pools/10/recycle", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"pool_id":    float64(10),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "recycle initiated successfully")
}

// =============================================================================
// LKE Node Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKENodeDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	assert.Equal(t, "linode_lke_node_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKENodeDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"node_id":    "abc-123",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKENodeDeleteTool_MissingNodeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "node_id is required")
}

func TestLinodeLKENodeDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/nodes/abc-123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"node_id":    "abc-123",
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted")
	assert.Contains(t, textContent.Text, "abc-123")
}

// =============================================================================
// LKE Node Recycle Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKENodeRecycleTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	assert.Equal(t, "linode_lke_node_recycle", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKENodeRecycleTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"node_id":    "abc-123",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKENodeRecycleTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/nodes/abc-123/recycle", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"node_id":    "abc-123",
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "recycle initiated successfully")
}

// =============================================================================
// LKE Kubeconfig Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEKubeconfigDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	assert.Equal(t, "linode_lke_kubeconfig_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEKubeconfigDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEKubeconfigDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/kubeconfig", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "regenerated successfully")
}

// =============================================================================
// LKE Service Token Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEServiceTokenDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	assert.Equal(t, "linode_lke_service_token_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEServiceTokenDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEServiceTokenDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/service-token", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "regenerated successfully")
}

// =============================================================================
// LKE ACL Update Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEACLUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

	assert.Equal(t, "linode_lke_acl_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "cluster_id")
	assert.Contains(t, props, "enabled")
	assert.Contains(t, props, "ipv4")
	assert.Contains(t, props, "ipv6")
	assert.Contains(t, props, "confirm")
}

func TestLinodeLKEACLUpdateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"enabled":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEACLUpdateTool_Success(t *testing.T) {
	t.Parallel()

	acl := linode.LKEControlPlaneACL{
		Enabled: true,
		Addresses: linode.LKEControlPlaneACLAddresses{
			IPv4: []string{"10.0.0.1/32", "192.168.1.0/24"},
			IPv6: []string{"2001:db8::1/128"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/control-plane-acl", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(acl))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"enabled":    true,
		"ipv4":       "10.0.0.1/32, 192.168.1.0/24",
		"ipv6":       "2001:db8::1/128",
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

// =============================================================================
// LKE ACL Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeLKEACLDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	assert.Equal(t, "linode_lke_acl_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeLKEACLDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"cluster_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeLKEACLDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/control-plane-acl", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"cluster_id": float64(123),
		"confirm":    true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

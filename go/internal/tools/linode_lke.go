package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeLKEClustersListTool creates a tool for listing all LKE clusters.
func NewLinodeLKEClustersListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_lke_clusters_list",
		"Lists all Linode Kubernetes Engine (LKE) clusters. Can filter by label.",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.LKECluster, error) {
			return client.ListLKEClusters(ctx)
		},
		[]listFilterParam[linode.LKECluster]{
			containsFilter("label", "Filter clusters by label containing this string (case-insensitive)",
				func(c linode.LKECluster) string { return c.Label }),
		},
		"clusters",
	)
}

// NewLinodeLKEClusterGetTool creates a tool for getting a single LKE cluster by ID.
func NewLinodeLKEClusterGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_cluster_get",
		mcp.WithDescription("Retrieves details of a single LKE cluster by its ID"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEClusterGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cluster, err := client.GetLKECluster(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve LKE cluster: %v", err)), nil
	}

	return marshalToolResponse(cluster)
}

// NewLinodeLKEPoolsListTool creates a tool for listing node pools in an LKE cluster.
func NewLinodeLKEPoolsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_pools_list",
		mcp.WithDescription("Lists all node pools for a specific LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolsListRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEPoolsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pools, err := client.ListLKENodePools(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list node pools for cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Count int                  `json:"count"`
		Pools []linode.LKENodePool `json:"pools"`
	}{
		Count: len(pools),
		Pools: pools,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEPoolGetTool creates a tool for getting a specific node pool.
func NewLinodeLKEPoolGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_pool_get",
		mcp.WithDescription("Retrieves details of a specific node pool within an LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
		mcp.WithString("pool_id",
			mcp.Required(),
			mcp.Description("The ID of the node pool to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEPoolGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	poolID, err := parseLKEPoolID(request.GetString("pool_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pool, err := client.GetLKENodePool(ctx, clusterID, poolID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve node pool %d for cluster %d: %v", poolID, clusterID, err)), nil
	}

	return marshalToolResponse(pool)
}

// NewLinodeLKENodeGetTool creates a tool for getting a specific node in an LKE cluster.
func NewLinodeLKENodeGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_node_get",
		mcp.WithDescription("Retrieves details of a specific node within an LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
		mcp.WithString("node_id",
			mcp.Required(),
			mcp.Description("The ID of the node to retrieve (string format)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKENodeGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKENodeGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeID := request.GetString("node_id", "")
	if nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	node, err := client.GetLKENode(ctx, clusterID, nodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve node '%s' for cluster %d: %v", nodeID, clusterID, err)), nil
	}

	return marshalToolResponse(node)
}

// NewLinodeLKEKubeconfigGetTool creates a tool for retrieving the kubeconfig of an LKE cluster.
func NewLinodeLKEKubeconfigGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_kubeconfig_get",
		mcp.WithDescription("Retrieves the kubeconfig file for an LKE cluster (base64-encoded)"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEKubeconfigGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEKubeconfigGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	kubeconfig, err := client.GetLKEKubeconfig(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve kubeconfig for cluster %d: %v", clusterID, err)), nil
	}

	return marshalToolResponse(kubeconfig)
}

// NewLinodeLKEDashboardGetTool creates a tool for retrieving the dashboard URL of an LKE cluster.
func NewLinodeLKEDashboardGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_dashboard_get",
		mcp.WithDescription("Retrieves the Kubernetes dashboard URL for an LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEDashboardGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEDashboardGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dashboard, err := client.GetLKEDashboard(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve dashboard URL for cluster %d: %v", clusterID, err)), nil
	}

	return marshalToolResponse(dashboard)
}

// NewLinodeLKEAPIEndpointsListTool creates a tool for listing API endpoints of an LKE cluster.
func NewLinodeLKEAPIEndpointsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_api_endpoints_list",
		mcp.WithDescription("Lists the API endpoints for an LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEAPIEndpointsListRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEAPIEndpointsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	endpoints, err := client.ListLKEAPIEndpoints(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list API endpoints for cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Count     int                     `json:"count"`
		Endpoints []linode.LKEAPIEndpoint `json:"endpoints"`
	}{
		Count:     len(endpoints),
		Endpoints: endpoints,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEACLGetTool creates a tool for getting the control plane ACL of an LKE cluster.
func NewLinodeLKEACLGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_acl_get",
		mcp.WithDescription("Retrieves the control plane ACL configuration for an LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEACLGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEACLGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	acl, err := client.GetLKEControlPlaneACL(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve control plane ACL for cluster %d: %v", clusterID, err)), nil
	}

	return marshalToolResponse(acl)
}

// NewLinodeLKEVersionsListTool creates a tool for listing available Kubernetes versions.
func NewLinodeLKEVersionsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_lke_versions_list",
		"Lists available Kubernetes versions for LKE clusters",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.LKEVersion, error) {
			return client.ListLKEVersions(ctx)
		},
		nil,
		"versions",
	)
}

// NewLinodeLKEVersionGetTool creates a tool for getting a specific Kubernetes version.
func NewLinodeLKEVersionGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_lke_version_get",
		mcp.WithDescription("Retrieves details of a specific Kubernetes version available for LKE"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("version",
			mcp.Required(),
			mcp.Description("The Kubernetes version ID (e.g., '1.31')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEVersionGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLKEVersionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	versionID := request.GetString("version", "")
	if versionID == "" {
		return mcp.NewToolResultError("version is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	version, err := client.GetLKEVersion(ctx, versionID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve LKE version '%s': %v", versionID, err)), nil
	}

	return marshalToolResponse(version)
}

// NewLinodeLKETypesListTool creates a tool for listing available LKE node types.
func NewLinodeLKETypesListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_lke_types_list",
		"Lists available node types for LKE clusters with pricing information",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.LKEType, error) {
			return client.ListLKETypes(ctx)
		},
		nil,
		"types",
	)
}

// NewLinodeLKETierVersionsListTool creates a tool for listing available LKE tier versions.
func NewLinodeLKETierVersionsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_lke_tier_versions_list",
		"Lists available LKE tier versions",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.LKETierVersion, error) {
			return client.ListLKETierVersions(ctx)
		},
		nil,
		"tier_versions",
	)
}

// parseLKEClusterID validates and converts the cluster ID string to an integer.
func parseLKEClusterID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrLKEClusterIDRequired
	}

	clusterID, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrLKEClusterIDInvalid, raw)
	}

	return clusterID, nil
}

// parseLKEPoolID validates and converts the pool ID string to an integer.
func parseLKEPoolID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrLKEPoolIDRequired
	}

	poolID, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrLKEPoolIDInvalid, raw)
	}

	return poolID, nil
}

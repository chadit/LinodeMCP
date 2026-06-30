package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeLKEClusterListTool creates a tool for listing all LKE clusters.
func NewLinodeLKEClusterListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_lke_cluster_list",
		"Lists all Linode Kubernetes Engine (LKE) clusters. Can filter by label.",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LKECluster, error) {
			return client.ListLKEClustersProto(ctx)
		},
		[]listFilterParam[*linodev1.LKECluster]{
			containsFilter("label", "Filter clusters by label containing this string (case-insensitive)",
				func(c *linodev1.LKECluster) string { return c.GetLabel() }),
		},
		lkeClusterListResponse,
	)

	return tool, profiles.CapRead, handler
}

func lkeClusterListResponse(items []*linodev1.LKECluster, count int32, filter *string) *linodev1.LKEClusterListResponse {
	return &linodev1.LKEClusterListResponse{Count: count, Filter: filter, Clusters: items}
}

// NewLinodeLKEClusterGetTool creates a tool for getting a single LKE cluster by ID.
func NewLinodeLKEClusterGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_cluster_get",
		"Retrieves details of a single LKE cluster by its ID",
		toolschemas.Schema("linode.mcp.v1.LKEClusterGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	cluster, err := client.GetLKEClusterProto(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve LKE cluster: %v", err)), nil
	}

	return MarshalProtoToolResponse(cluster)
}

// NewLinodeLKEPoolListTool creates a tool for listing node pools in an LKE cluster.
func NewLinodeLKEPoolListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresource(
		cfg,
		"linode_lke_pool_list",
		"Lists all node pools for a specific LKE cluster",
		protoListPathID{
			option: mcp.WithString("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			parse: lkePoolListClusterIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, clusterID int) ([]*linodev1.LKENodePool, error) {
			return client.ListLKENodePoolsProto(ctx, clusterID)
		},
		nil,
		lkePoolListResponse,
	)

	return tool, profiles.CapRead, handler
}

// lkePoolListClusterIDFromTool validates the cluster_id path param exactly like
// the non-proto handler did (via parseLKEClusterID), returning the same error
// text. cluster_id is a string param, so this preserves the family's schema.
func lkePoolListClusterIDFromTool(request *mcp.CallToolRequest) (int, string) {
	clusterID, err := parseLKEClusterID(request.GetString("cluster_id", ""))
	if err != nil {
		return 0, err.Error()
	}

	return clusterID, ""
}

func lkePoolListResponse(items []*linodev1.LKENodePool, count int32, filter *string) *linodev1.LKENodePoolListResponse {
	return &linodev1.LKENodePoolListResponse{Count: count, Filter: filter, Pools: items}
}

// NewLinodeLKEPoolGetTool creates a tool for getting a specific node pool.
func NewLinodeLKEPoolGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_pool_get",
		"Retrieves details of a specific node pool within an LKE cluster",
		toolschemas.Schema("linode.mcp.v1.LKENodePoolGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	pool, err := client.GetLKENodePoolProto(ctx, clusterID, poolID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve node pool %d for cluster %d: %v", poolID, clusterID, err)), nil
	}

	return MarshalProtoToolResponse(pool)
}

// NewLinodeLKENodeGetTool creates a tool for getting a specific node in an LKE cluster.
func NewLinodeLKENodeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_node_get",
		"Retrieves details of a specific node within an LKE cluster",
		toolschemas.Schema("linode.mcp.v1.LKENodeGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKENodeGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	node, err := client.GetLKENodeProto(ctx, clusterID, nodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve node '%s' for cluster %d: %v", nodeID, clusterID, err)), nil
	}

	return MarshalProtoToolResponse(node)
}

// NewLinodeLKEKubeconfigGetTool creates a tool for retrieving the kubeconfig of an LKE cluster.
func NewLinodeLKEKubeconfigGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_kubeconfig_get",
		"Retrieves the kubeconfig file for an LKE cluster (base64-encoded)",
		toolschemas.Schema("linode.mcp.v1.LKEKubeconfigGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEKubeconfigGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	kubeconfig, err := client.GetLKEKubeconfigProto(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve kubeconfig for cluster %d: %v", clusterID, err)), nil
	}

	return MarshalProtoToolResponse(kubeconfig)
}

// NewLinodeLKEDashboardGetTool creates a tool for retrieving the dashboard URL of an LKE cluster.
func NewLinodeLKEDashboardGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_dashboard_get",
		"Retrieves the Kubernetes dashboard URL for an LKE cluster",
		toolschemas.Schema("linode.mcp.v1.LKEDashboardGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEDashboardGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	dashboard, err := client.GetLKEDashboardProto(ctx, clusterID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve dashboard URL for cluster %d: %v", clusterID, err)), nil
	}

	return MarshalProtoToolResponse(dashboard)
}

// NewLinodeLKEAPIEndpointListTool creates a tool for listing API endpoints of an LKE cluster.
func NewLinodeLKEAPIEndpointListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresource(
		cfg,
		"linode_lke_api_endpoint_list",
		"Lists the API endpoints for an LKE cluster",
		protoListPathID{
			option: mcp.WithString("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			parse: lkePoolListClusterIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, clusterID int) ([]*linodev1.LKEAPIEndpoint, error) {
			return client.ListLKEAPIEndpointsProto(ctx, clusterID)
		},
		nil,
		lkeAPIEndpointListResponse,
	)

	return tool, profiles.CapRead, handler
}

func lkeAPIEndpointListResponse(items []*linodev1.LKEAPIEndpoint, count int32, filter *string) *linodev1.LKEAPIEndpointListResponse {
	return &linodev1.LKEAPIEndpointListResponse{Count: count, Filter: filter, Endpoints: items}
}

// NewLinodeLKEACLGetTool creates a tool for getting the control plane ACL of an LKE cluster.
func NewLinodeLKEACLGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_lke_acl_get",
		mcp.WithDescription("Retrieves the control plane ACL configuration for an LKE cluster"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString(
			"cluster_id",
			mcp.Required(),
			mcp.Description("The ID of the LKE cluster"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEACLGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	return MarshalToolResponse(acl)
}

// NewLinodeLKEVersionListTool creates a tool for listing available Kubernetes versions.
func NewLinodeLKEVersionListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_lke_version_list",
		"Lists available Kubernetes versions for LKE clusters",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LKEVersion, error) {
			return client.ListLKEVersionsProto(ctx)
		},
		nil,
		lkeVersionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func lkeVersionListResponse(items []*linodev1.LKEVersion, count int32, filter *string) *linodev1.LKEVersionListResponse {
	return &linodev1.LKEVersionListResponse{Count: count, Filter: filter, Versions: items}
}

// NewLinodeLKEVersionGetTool creates a tool for getting a specific Kubernetes version.
func NewLinodeLKEVersionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_version_get",
		"Retrieves details of a specific Kubernetes version available for LKE",
		toolschemas.Schema("linode.mcp.v1.LKEVersionGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEVersionGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLKEVersionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	versionID := request.GetString("version", "")
	if errMsg := validateLKEVersionID(versionID); errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	version, err := client.GetLKEVersionProto(ctx, versionID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve LKE version '%s': %v", versionID, err)), nil
	}

	return MarshalProtoToolResponse(version)
}

func validateLKEVersionID(versionID string) string {
	if versionID == "" {
		return "version is required"
	}

	if strings.Contains(versionID, "/") || strings.Contains(versionID, "?") || strings.Contains(versionID, "..") {
		return "version must be a Kubernetes version ID"
	}

	return ""
}

// NewLinodeLKETypeListTool creates a tool for listing available LKE node types.
func NewLinodeLKETypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_lke_type_list",
		"Lists available node types for LKE clusters with pricing information",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LinodeType, error) {
			return client.ListLKETypesProto(ctx)
		},
		nil,
		lkeTypeListResponse,
	)

	return tool, profiles.CapRead, handler
}

func lkeTypeListResponse(items []*linodev1.LinodeType, count int32, filter *string) *linodev1.LKETypeListResponse {
	return &linodev1.LKETypeListResponse{Count: count, Filter: filter, LkeTypes: items}
}

// NewLinodeLKETierVersionListTool creates a tool for listing available LKE tier versions.
func NewLinodeLKETierVersionListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourceString(
		cfg,
		"linode_lke_tier_version_list",
		"Lists available LKE tier versions for the requested tier",
		protoListPathIDString{
			option: mcp.WithString("tier", mcp.Required(), mcp.Description("LKE tier: standard or enterprise.")),
			parse:  lkeTierVersionListTierFromTool,
		},
		func(ctx context.Context, client *linode.Client, tier string) ([]*linodev1.LKETierVersion, error) {
			return client.ListLKETierVersionsProto(ctx, tier)
		},
		nil,
		lkeTierVersionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func lkeTierVersionListResponse(items []*linodev1.LKETierVersion, count int32, filter *string) *linodev1.LKETierVersionListResponse {
	return &linodev1.LKETierVersionListResponse{Count: count, Filter: filter, TierVersions: items}
}

// lkeTierVersionListTierFromTool validates the required tier path-id the same way
// the pre-proto handler did (parseLKETier accepts only standard or enterprise),
// returning the validated tier and any error message.
func lkeTierVersionListTierFromTool(request *mcp.CallToolRequest) (string, string) {
	tier, err := parseLKETier(request.GetString("tier", ""))
	if err != nil {
		return "", err.Error()
	}

	return tier, ""
}

// NewLinodeLKETierVersionGetTool creates a tool for getting a specific Kubernetes version for an LKE tier.
func NewLinodeLKETierVersionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_tier_version_get",
		"Retrieves details of a specific Kubernetes version for an LKE tier",
		toolschemas.Schema("linode.mcp.v1.LKETierVersionGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKETierVersionGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLKETierVersionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tier, errMessage := validateLKETierParam(request.GetString("tier", ""))
	if errMessage != "" {
		return mcp.NewToolResultError(errMessage), nil
	}

	versionID, errMessage := validateLKETierVersionID(request.GetString("version", ""))
	if errMessage != "" {
		return mcp.NewToolResultError(errMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	version, err := client.GetLKETierVersionProto(ctx, tier, versionID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve LKE tier version '%s' for tier '%s': %v", versionID, tier, err)), nil
	}

	return MarshalProtoToolResponse(version)
}

func validateLKETierVersionID(raw string) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "version is required"
	}

	if value != raw || strings.ContainsAny(value, "/?#") || strings.Contains(value, "..") {
		return "", "version must not contain path separators, query separators, fragments, or traversal segments"
	}

	return value, ""
}

func validateLKETierParam(raw string) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ErrLKETierRequired.Error()
	}

	if value != raw || strings.ContainsAny(value, "/?#") || strings.Contains(value, "..") {
		return "", "tier must not contain path separators, query separators, fragments, or traversal segments"
	}

	tier, err := parseLKETier(value)
	if err != nil {
		return "", err.Error()
	}

	return tier, ""
}

// parseLKEClusterID validates and converts the cluster ID string to an integer.
func parseLKETier(raw string) (string, error) {
	if raw == "" {
		return "", ErrLKETierRequired
	}

	switch raw {
	case "standard", "enterprise":
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrLKETierInvalid, raw)
	}
}

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

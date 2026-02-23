package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// handleLKESubResourceAction handles confirmed actions on LKE sub-resources
// (node pools with int IDs, individual nodes with string IDs) using generics
// to avoid code duplication across the delete/recycle handler variants.
func handleLKESubResourceAction[ID int | string](
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	confirmMsg string,
	idParam string,
	extractID func(*mcp.CallToolRequest) (ID, bool),
	action func(context.Context, *linode.RetryableClient, int, ID) error,
	errFmt, successFmt string,
) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, confirmMsg); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	subID, ok := extractID(request)
	if !ok {
		return mcp.NewToolResultError(idParam + " is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := action(ctx, client, clusterID, subID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(errFmt, subID, clusterID, err)), nil
	}

	return marshalToolResponse(map[string]any{
		"message":    fmt.Sprintf(successFmt, subID, clusterID),
		"cluster_id": clusterID,
		idParam:      subID,
	})
}

// NewLinodeLKEClusterCreateTool creates a tool for creating a new LKE cluster.
func NewLinodeLKEClusterCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_cluster_create",
		"Creates a new LKE Kubernetes cluster. WARNING: This creates billable resources. "+
			"Use linode_lke_versions_list to find valid k8s_version values, linode_regions_list for regions, "+
			"and linode_lke_types_list for node types.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the cluster (3-32 characters)")),
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region for the cluster (e.g. us-east)")),
			mcp.WithString("k8s_version", mcp.Required(),
				mcp.Description("Kubernetes version (e.g. 1.29). Use linode_lke_versions_list to find valid values.")),
			mcp.WithString("node_pools", mcp.Required(),
				mcp.Description("JSON array of node pools: [{\"type\": \"g6-standard-2\", \"count\": 3}]. "+
					"Optional per-pool fields: autoscaler ({\"enabled\": true, \"min\": 1, \"max\": 5}), tags.")),
			mcp.WithString("tags",
				mcp.Description("Comma-separated tags for the cluster (optional)")),
			mcp.WithBoolean("high_availability",
				mcp.Description("Enable high availability control plane (optional, incurs additional cost)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm cluster creation. This creates billable resources.")),
		},
		handleLKEClusterCreateRequest,
	)
}

func handleLKEClusterCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This creates billable Kubernetes resources. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	label := request.GetString("label", "")
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	region := request.GetString("region", "")
	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	k8sVersion := request.GetString("k8s_version", "")
	if k8sVersion == "" {
		return mcp.NewToolResultError("k8s_version is required"), nil
	}

	nodePoolsJSON := request.GetString("node_pools", "")
	if nodePoolsJSON == "" {
		return mcp.NewToolResultError("node_pools is required (JSON array of node pool definitions)"), nil
	}

	var nodePools []linode.CreateLKEClusterNodePool
	if err := json.Unmarshal([]byte(nodePoolsJSON), &nodePools); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid node_pools JSON: %v", err)), nil
	}

	if len(nodePools) == 0 {
		return mcp.NewToolResultError("at least one node pool is required"), nil
	}

	req := linode.CreateLKEClusterRequest{
		Label:      label,
		Region:     region,
		K8sVersion: k8sVersion,
		NodePools:  nodePools,
	}

	if tagsStr := request.GetString("tags", ""); tagsStr != "" {
		req.Tags = splitTags(tagsStr)
	}

	if _, ok := request.GetArguments()["high_availability"]; ok {
		ha := request.GetBool("high_availability", false)
		req.ControlPlane = &linode.LKEControlPlane{HighAvailability: ha}
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cluster, err := client.CreateLKECluster(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create LKE cluster: %v", err)), nil
	}

	response := struct {
		Message string             `json:"message"`
		Cluster *linode.LKECluster `json:"cluster"`
	}{
		Message: fmt.Sprintf("LKE cluster '%s' (ID: %d) created in %s with Kubernetes %s", cluster.Label, cluster.ID, cluster.Region, cluster.K8sVersion),
		Cluster: cluster,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEClusterUpdateTool creates a tool for updating an LKE cluster.
func NewLinodeLKEClusterUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_cluster_update",
		"Updates an existing LKE cluster's label, Kubernetes version, tags, or high availability setting.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster to update")),
			mcp.WithString("label",
				mcp.Description("New label for the cluster (optional)")),
			mcp.WithString("k8s_version",
				mcp.Description("New Kubernetes version (optional). Use linode_lke_versions_list to find valid values.")),
			mcp.WithString("tags",
				mcp.Description("Comma-separated tags for the cluster (optional, replaces existing tags)")),
			mcp.WithBoolean("high_availability",
				mcp.Description("Enable or disable high availability control plane (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm cluster update.")),
		},
		handleLKEClusterUpdateRequest,
	)
}

func handleLKEClusterUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This modifies the LKE cluster configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	req := linode.UpdateLKEClusterRequest{}

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
	}

	if k8sVersion := request.GetString("k8s_version", ""); k8sVersion != "" {
		req.K8sVersion = k8sVersion
	}

	if tagsStr := request.GetString("tags", ""); tagsStr != "" {
		req.Tags = splitTags(tagsStr)
	}

	if _, ok := request.GetArguments()["high_availability"]; ok {
		ha := request.GetBool("high_availability", false)
		req.ControlPlane = &linode.LKEControlPlane{HighAvailability: ha}
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cluster, err := client.UpdateLKECluster(ctx, clusterID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update LKE cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message string             `json:"message"`
		Cluster *linode.LKECluster `json:"cluster"`
	}{
		Message: fmt.Sprintf("LKE cluster %d updated successfully", clusterID),
		Cluster: cluster,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEClusterDeleteTool creates a tool for deleting an LKE cluster.
func NewLinodeLKEClusterDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_cluster_delete",
		"Deletes an LKE cluster. WARNING: This is irreversible. All node pools, nodes, and associated resources will be deleted.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deletion. This action is irreversible.")),
		},
		handleLKEClusterDeleteRequest,
	)
}

func handleLKEClusterDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This is irreversible. All node pools, nodes, and associated resources will be deleted. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteLKECluster(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete LKE cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message   string `json:"message"`
		ClusterID int    `json:"cluster_id"`
	}{
		Message:   fmt.Sprintf("LKE cluster %d deleted successfully", clusterID),
		ClusterID: clusterID,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEClusterRecycleTool creates a tool for recycling all nodes in an LKE cluster.
func NewLinodeLKEClusterRecycleTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_cluster_recycle",
		"Recycles all nodes in an LKE cluster. WARNING: This causes temporary disruption as all nodes are replaced.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster to recycle")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm recycling. This causes temporary disruption.")),
		},
		handleLKEClusterRecycleRequest,
	)
}

func handleLKEClusterRecycleRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "Recycles all nodes in the cluster. This causes temporary disruption. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.RecycleLKECluster(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to recycle LKE cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message   string `json:"message"`
		ClusterID int    `json:"cluster_id"`
	}{
		Message:   fmt.Sprintf("LKE cluster %d recycle initiated successfully", clusterID),
		ClusterID: clusterID,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEClusterRegenerateTool creates a tool for regenerating an LKE cluster's service token.
func NewLinodeLKEClusterRegenerateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_cluster_regenerate",
		"Regenerates the service token for an LKE cluster. Existing tokens will stop working.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm token regeneration.")),
		},
		handleLKEClusterRegenerateRequest,
	)
}

func handleLKEClusterRegenerateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This regenerates the cluster service token. Existing tokens will stop working. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.RegenerateLKECluster(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to regenerate service token for LKE cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message   string `json:"message"`
		ClusterID int    `json:"cluster_id"`
	}{
		Message:   fmt.Sprintf("Service token for LKE cluster %d regenerated successfully", clusterID),
		ClusterID: clusterID,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEPoolCreateTool creates a tool for adding a node pool to an LKE cluster.
func NewLinodeLKEPoolCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_pool_create",
		"Creates a new node pool in an LKE cluster. WARNING: This creates billable compute resources.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("Linode type for pool nodes (e.g. g6-standard-2). Use linode_lke_types_list to find valid types.")),
			mcp.WithNumber("count", mcp.Required(),
				mcp.Description("Number of nodes in the pool (minimum 1)")),
			mcp.WithBoolean("autoscaler_enabled",
				mcp.Description("Enable the node pool autoscaler (optional)")),
			mcp.WithNumber("autoscaler_min",
				mcp.Description("Minimum number of nodes for the autoscaler (optional)")),
			mcp.WithNumber("autoscaler_max",
				mcp.Description("Maximum number of nodes for the autoscaler (optional)")),
			mcp.WithString("tags",
				mcp.Description("Comma-separated tags for the node pool (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm pool creation. This creates billable resources.")),
		},
		handleLKEPoolCreateRequest,
	)
}

func handleLKEPoolCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This creates billable compute resources. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	nodeType := request.GetString("type", "")
	if nodeType == "" {
		return mcp.NewToolResultError("type is required"), nil
	}

	count := request.GetInt("count", 0)
	if count <= 0 {
		return mcp.NewToolResultError("count is required and must be at least 1"), nil
	}

	req := linode.CreateLKENodePoolRequest{
		Type:  nodeType,
		Count: count,
	}

	if _, ok := request.GetArguments()["autoscaler_enabled"]; ok {
		autoscaler := &linode.LKENodePoolAutoscaler{
			Enabled: request.GetBool("autoscaler_enabled", false),
			Min:     request.GetInt("autoscaler_min", 0),
			Max:     request.GetInt("autoscaler_max", 0),
		}
		req.Autoscaler = autoscaler
	}

	if tagsStr := request.GetString("tags", ""); tagsStr != "" {
		req.Tags = splitTags(tagsStr)
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pool, err := client.CreateLKENodePool(ctx, clusterID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create node pool in cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message string              `json:"message"`
		Pool    *linode.LKENodePool `json:"pool"`
	}{
		Message: fmt.Sprintf("Node pool (ID: %d) created in cluster %d with %d %s node(s)", pool.ID, clusterID, pool.Count, pool.Type),
		Pool:    pool,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEPoolUpdateTool creates a tool for updating an LKE node pool.
func NewLinodeLKEPoolUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_pool_update",
		"Updates a node pool in an LKE cluster. Can change node count, autoscaler settings, or tags.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithNumber("pool_id", mcp.Required(),
				mcp.Description("The ID of the node pool to update")),
			mcp.WithNumber("count",
				mcp.Description("New number of nodes in the pool (optional)")),
			mcp.WithBoolean("autoscaler_enabled",
				mcp.Description("Enable or disable the node pool autoscaler (optional)")),
			mcp.WithNumber("autoscaler_min",
				mcp.Description("Minimum number of nodes for the autoscaler (optional)")),
			mcp.WithNumber("autoscaler_max",
				mcp.Description("Maximum number of nodes for the autoscaler (optional)")),
			mcp.WithString("tags",
				mcp.Description("Comma-separated tags for the node pool (optional, replaces existing tags)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm pool update.")),
		},
		handleLKEPoolUpdateRequest,
	)
}

func handleLKEPoolUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This modifies the node pool configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	poolID := request.GetInt("pool_id", 0)
	if poolID == 0 {
		return mcp.NewToolResultError("pool_id is required"), nil
	}

	req := linode.UpdateLKENodePoolRequest{}

	if _, ok := request.GetArguments()["count"]; ok {
		count := request.GetInt("count", 0)
		req.Count = &count
	}

	if _, ok := request.GetArguments()["autoscaler_enabled"]; ok {
		autoscaler := &linode.LKENodePoolAutoscaler{
			Enabled: request.GetBool("autoscaler_enabled", false),
			Min:     request.GetInt("autoscaler_min", 0),
			Max:     request.GetInt("autoscaler_max", 0),
		}
		req.Autoscaler = autoscaler
	}

	if tagsStr := request.GetString("tags", ""); tagsStr != "" {
		req.Tags = splitTags(tagsStr)
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pool, err := client.UpdateLKENodePool(ctx, clusterID, poolID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update node pool %d in cluster %d: %v", poolID, clusterID, err)), nil
	}

	response := struct {
		Message string              `json:"message"`
		Pool    *linode.LKENodePool `json:"pool"`
	}{
		Message: fmt.Sprintf("Node pool %d in cluster %d updated successfully", poolID, clusterID),
		Pool:    pool,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEPoolDeleteTool creates a tool for deleting a node pool from an LKE cluster.
func NewLinodeLKEPoolDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_pool_delete",
		"Deletes a node pool from an LKE cluster. All nodes in the pool will be removed.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithNumber("pool_id", mcp.Required(),
				mcp.Description("The ID of the node pool to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm pool deletion.")),
		},
		handleLKEPoolDeleteRequest,
	)
}

func handleLKEPoolDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleLKESubResourceAction(ctx, request, cfg,
		"This deletes the node pool and all its nodes. Set confirm=true to proceed.",
		"pool_id",
		func(r *mcp.CallToolRequest) (int, bool) {
			id := r.GetInt("pool_id", 0)

			return id, id != 0
		},
		func(ctx context.Context, c *linode.RetryableClient, clusterID, poolID int) error {
			return c.DeleteLKENodePool(ctx, clusterID, poolID)
		},
		"Failed to delete node pool %d from cluster %d: %v",
		"Node pool %d deleted from cluster %d successfully",
	)
}

// NewLinodeLKEPoolRecycleTool creates a tool for recycling all nodes in an LKE node pool.
func NewLinodeLKEPoolRecycleTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_pool_recycle",
		"Recycles all nodes in a specific LKE node pool. Nodes will be replaced with new ones.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithNumber("pool_id", mcp.Required(),
				mcp.Description("The ID of the node pool to recycle")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm pool recycling.")),
		},
		handleLKEPoolRecycleRequest,
	)
}

func handleLKEPoolRecycleRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleLKESubResourceAction(ctx, request, cfg,
		"This recycles all nodes in the pool, causing temporary disruption. Set confirm=true to proceed.",
		"pool_id",
		func(r *mcp.CallToolRequest) (int, bool) {
			id := r.GetInt("pool_id", 0)

			return id, id != 0
		},
		func(ctx context.Context, c *linode.RetryableClient, clusterID, poolID int) error {
			return c.RecycleLKENodePool(ctx, clusterID, poolID)
		},
		"Failed to recycle node pool %d in cluster %d: %v",
		"Node pool %d in cluster %d recycle initiated successfully",
	)
}

// NewLinodeLKENodeDeleteTool creates a tool for deleting a specific node from an LKE cluster.
func NewLinodeLKENodeDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_node_delete",
		"Deletes a specific node from an LKE cluster. The node will be removed and may be replaced depending on pool settings.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithString("node_id", mcp.Required(),
				mcp.Description("The ID of the node to delete (string format)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm node deletion.")),
		},
		handleLKENodeDeleteRequest,
	)
}

func handleLKENodeDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleLKESubResourceAction(ctx, request, cfg,
		"This deletes the specified node. Set confirm=true to proceed.",
		"node_id",
		func(r *mcp.CallToolRequest) (string, bool) {
			id := r.GetString("node_id", "")

			return id, id != ""
		},
		func(ctx context.Context, c *linode.RetryableClient, clusterID int, nodeID string) error {
			return c.DeleteLKENode(ctx, clusterID, nodeID)
		},
		"Failed to delete node %s from cluster %d: %v",
		"Node %s deleted from cluster %d successfully",
	)
}

// NewLinodeLKENodeRecycleTool creates a tool for recycling a specific node in an LKE cluster.
func NewLinodeLKENodeRecycleTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_node_recycle",
		"Recycles a specific node in an LKE cluster. The node will be drained and replaced with a new one.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithString("node_id", mcp.Required(),
				mcp.Description("The ID of the node to recycle (string format)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm node recycling.")),
		},
		handleLKENodeRecycleRequest,
	)
}

func handleLKENodeRecycleRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleLKESubResourceAction(ctx, request, cfg,
		"This recycles the specified node, replacing it with a new one. Set confirm=true to proceed.",
		"node_id",
		func(r *mcp.CallToolRequest) (string, bool) {
			id := r.GetString("node_id", "")

			return id, id != ""
		},
		func(ctx context.Context, c *linode.RetryableClient, clusterID int, nodeID string) error {
			return c.RecycleLKENode(ctx, clusterID, nodeID)
		},
		"Failed to recycle node %s in cluster %d: %v",
		"Node %s in cluster %d recycle initiated successfully",
	)
}

// NewLinodeLKEKubeconfigDeleteTool creates a tool for deleting and regenerating an LKE cluster's kubeconfig.
func NewLinodeLKEKubeconfigDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_kubeconfig_delete",
		"Deletes and regenerates the kubeconfig for an LKE cluster. Existing kubeconfig files will stop working.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm kubeconfig deletion.")),
		},
		handleLKEKubeconfigDeleteRequest,
	)
}

func handleLKEKubeconfigDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This deletes the kubeconfig. Existing kubeconfig files will stop working. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteLKEKubeconfig(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete kubeconfig for LKE cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message   string `json:"message"`
		ClusterID int    `json:"cluster_id"`
	}{
		Message:   fmt.Sprintf("Kubeconfig for LKE cluster %d deleted and regenerated successfully", clusterID),
		ClusterID: clusterID,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEServiceTokenDeleteTool creates a tool for deleting and regenerating an LKE cluster's service token.
func NewLinodeLKEServiceTokenDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_service_token_delete",
		"Deletes and regenerates the service token for an LKE cluster. Existing tokens will stop working.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm service token deletion.")),
		},
		handleLKEServiceTokenDeleteRequest,
	)
}

func handleLKEServiceTokenDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This deletes the service token. Existing tokens will stop working. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteLKEServiceToken(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete service token for LKE cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message   string `json:"message"`
		ClusterID int    `json:"cluster_id"`
	}{
		Message:   fmt.Sprintf("Service token for LKE cluster %d deleted and regenerated successfully", clusterID),
		ClusterID: clusterID,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEACLUpdateTool creates a tool for updating the control plane ACL of an LKE cluster.
func NewLinodeLKEACLUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_acl_update",
		"Updates the control plane ACL for an LKE cluster. Controls which IP addresses can access the cluster's API server.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithBoolean("enabled", mcp.Required(),
				mcp.Description("Whether to enable the control plane ACL")),
			mcp.WithString("ipv4",
				mcp.Description("Comma-separated list of IPv4 addresses/CIDRs to allow (optional)")),
			mcp.WithString("ipv6",
				mcp.Description("Comma-separated list of IPv6 addresses/CIDRs to allow (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm ACL update.")),
		},
		handleLKEACLUpdateRequest,
	)
}

func handleLKEACLUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This modifies the control plane ACL, which controls API server access. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	enabled := request.GetBool("enabled", false)

	acl := linode.LKEControlPlaneACL{
		Enabled: enabled,
	}

	if ipv4Str := request.GetString("ipv4", ""); ipv4Str != "" {
		acl.Addresses.IPv4 = splitCommaSeparated(ipv4Str)
	}

	if ipv6Str := request.GetString("ipv6", ""); ipv6Str != "" {
		acl.Addresses.IPv6 = splitCommaSeparated(ipv6Str)
	}

	req := linode.UpdateLKEControlPlaneACLRequest{
		ACL: acl,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := client.UpdateLKEControlPlaneACL(ctx, clusterID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update control plane ACL for cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message string                     `json:"message"`
		ACL     *linode.LKEControlPlaneACL `json:"acl"`
	}{
		Message: fmt.Sprintf("Control plane ACL for cluster %d updated successfully", clusterID),
		ACL:     result,
	}

	return marshalToolResponse(response)
}

// NewLinodeLKEACLDeleteTool creates a tool for deleting the control plane ACL of an LKE cluster.
func NewLinodeLKEACLDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_lke_acl_delete",
		"Deletes the control plane ACL for an LKE cluster. This removes all IP restrictions from the API server.",
		[]mcp.ToolOption{
			mcp.WithNumber("cluster_id", mcp.Required(),
				mcp.Description("The ID of the LKE cluster")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm ACL deletion.")),
		},
		handleLKEACLDeleteRequest,
	)
}

func handleLKEACLDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This removes all IP restrictions from the API server. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	clusterID := request.GetInt("cluster_id", 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteLKEControlPlaneACL(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete control plane ACL for cluster %d: %v", clusterID, err)), nil
	}

	response := struct {
		Message   string `json:"message"`
		ClusterID int    `json:"cluster_id"`
	}{
		Message:   fmt.Sprintf("Control plane ACL for cluster %d deleted successfully", clusterID),
		ClusterID: clusterID,
	}

	return marshalToolResponse(response)
}

// splitTags splits a comma-separated tags string into a trimmed slice.
func splitTags(s string) []string {
	return splitCommaSeparated(s)
}

// splitCommaSeparated splits a comma-separated string into trimmed, non-empty parts.
func splitCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

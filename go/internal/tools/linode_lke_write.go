package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const (
	paramClusterID  = "cluster_id"
	lkeClustersPath = "/lke/clusters"
)

// handleLKESubResourceAction handles confirmed actions on LKE sub-resources
// (node pools with int IDs, individual nodes with string IDs) using generics
// to avoid code duplication across the delete/recycle handler variants.
func handleLKESubResourceAction[ID int | string](
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName string,
	confirmMsg string,
	idParam string,
	extractID func(*mcp.CallToolRequest) (ID, bool),
	action func(context.Context, *linode.Client, int, ID) error,
	errFmt string,
	successProto func(clusterID int, subID ID) proto.Message,
) (*mcp.CallToolResult, error) {
	// Recycles destroy the running nodes, so they carry the full destroy
	// gate, not just a confirm check.
	if result := requireDestroyConfirmation(ctx, request, toolName, confirmMsg); result != nil {
		return result, nil
	}

	clusterID := request.GetInt(paramClusterID, 0)
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

	return MarshalProtoToolResponse(successProto(clusterID, subID))
}

// NewLinodeLKEClusterCreateTool creates a tool for creating a new LKE cluster.
func NewLinodeLKEClusterCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_cluster_create",
		"Creates a new LKE Kubernetes cluster. WARNING: This creates billable resources. "+
			"Use linode_lke_version_list to find valid k8s_version values, linode_region_list for regions, "+
			"and linode_lke_type_list for node types. Pass dry_run=true to preview without creating.",
		toolschemas.Schema("linode.mcp.v1.LKEClusterCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateLKEClusterCreateArgs validates required args and builds the
// create request, shared by the dry-run and real-execution paths.
func validateLKEClusterCreateArgs(request *mcp.CallToolRequest) (*linode.CreateLKEClusterRequest, *mcp.CallToolResult) {
	label := request.GetString("label", "")
	if label == "" {
		return nil, mcp.NewToolResultError("label is required")
	}

	region := request.GetString("region", "")
	if region == "" {
		return nil, mcp.NewToolResultError(errRegionRequired)
	}

	k8sVersion := request.GetString("k8s_version", "")
	if k8sVersion == "" {
		return nil, mcp.NewToolResultError("k8s_version is required")
	}

	rawNodePools, present := request.GetArguments()["node_pools"]
	if !present {
		return nil, mcp.NewToolResultError("node_pools is required (array of node pool definitions)")
	}

	nodePools, validationMessage := objectSliceFromToolArg[linode.CreateLKEClusterNodePool](rawNodePools, "node_pools")
	if validationMessage != "" {
		return nil, mcp.NewToolResultError(validationMessage)
	}

	if len(nodePools) == 0 {
		return nil, mcp.NewToolResultError("at least one node pool is required")
	}

	req := &linode.CreateLKEClusterRequest{
		Label:      label,
		Region:     region,
		K8sVersion: k8sVersion,
		NodePools:  nodePools,
	}

	if result := applyOptionalTags(request, &req.Tags); result != nil {
		return nil, result
	}

	if raw, ok := request.GetArguments()["control_plane"]; ok {
		controlPlane, isObject := raw.(map[string]any)
		if !isObject {
			return nil, mcp.NewToolResultError("control_plane must be an object")
		}

		highAvailability, _ := controlPlane["high_availability"].(bool)
		req.ControlPlane = &linode.LKEControlPlane{HighAvailability: highAvailability}
	}

	return req, nil
}

func handleLKEClusterCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, errResult := validateLKEClusterCreateArgs(request)
	if errResult != nil {
		return errResult, nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_lke_cluster_create", httpMethodPost, lkeClustersPath, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return lkeClusterCreateSideEffects(ctx,
					request.GetString("label", ""),
					request.GetString("region", ""),
					request.GetString("k8s_version", ""))
			})
	}

	if result := RequireConfirm(request, "This creates billable Kubernetes resources. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cluster, err := client.CreateLKEClusterProto(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create LKE cluster: %v", err)), nil
	}

	response := &linodev1.LKEClusterWriteResponse{
		Message: fmt.Sprintf("LKE cluster '%s' (ID: %d) created in %s with Kubernetes %s", cluster.GetLabel(), cluster.GetId(), cluster.GetRegion(), cluster.GetK8SVersion()),
		Cluster: cluster,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeLKEClusterUpdateTool creates a tool for updating an LKE cluster.
func NewLinodeLKEClusterUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_cluster_update",
		"Updates an existing LKE cluster's label, Kubernetes version, tags, or high availability setting."+
			" Pass dry_run=true to preview without modifying.",
		toolschemas.Schema("linode.mcp.v1.LKEClusterUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLKEClusterUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_lke_cluster_update", "PUT",
			fmt.Sprintf(lkeClustersPath+"/%d", clusterID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKECluster(ctx, clusterID)
			},
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return lkeClusterUpdateSideEffects(ctx, state,
					request.GetString("label", ""), request.GetString("k8s_version", ""))
			})
	}

	if result := RequireConfirm(request, "This modifies the LKE cluster configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req := linode.UpdateLKEClusterRequest{}

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
	}

	if k8sVersion := request.GetString("k8s_version", ""); k8sVersion != "" {
		req.K8sVersion = k8sVersion
	}

	if result := applyOptionalTags(request, &req.Tags); result != nil {
		return result, nil
	}

	if raw, ok := request.GetArguments()["control_plane"]; ok {
		controlPlane, isObject := raw.(map[string]any)
		if !isObject {
			return mcp.NewToolResultError("control_plane must be an object"), nil
		}

		highAvailability, _ := controlPlane["high_availability"].(bool)
		req.ControlPlane = &linode.LKEControlPlane{HighAvailability: highAvailability}
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cluster, err := client.UpdateLKEClusterProto(ctx, clusterID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify LKE cluster %d: %v", clusterID, err)), nil
	}

	response := &linodev1.LKEClusterWriteResponse{
		Message: fmt.Sprintf("LKE cluster %d modified successfully", clusterID),
		Cluster: cluster,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeLKEClusterDeleteTool creates a tool for deleting an LKE cluster.
func NewLinodeLKEClusterDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_cluster_delete",
		"Deletes an LKE cluster. WARNING: This is irreversible. All node pools, nodes, and associated resources will be deleted."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.LKEClusterDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// lkeClusterDeleteProto builds the proto-canonical id-echo body for a
// successful LKE cluster delete, keeping the proto literal off the handler's
// struct literal so the delete handlers stay below the dupl threshold.
func lkeClusterDeleteProto(id int) proto.Message {
	return &linodev1.LKEClusterDeleteResponse{
		Message:   fmt.Sprintf("LKE cluster %d removed successfully", id),
		ClusterId: linodeIDToInt32(id),
	}
}

func handleLKEClusterDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_lke_cluster_delete",
		IDParam:        paramClusterID,
		Method:         httpMethodDelete,
		PathPattern:    "/lke/clusters/%d",
		ConfirmMessage: "This is irreversible. All node pools, nodes, and associated resources will be deleted. Set confirm=true to proceed.",
		SuccessProto:   lkeClusterDeleteProto,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetLKECluster(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteLKECluster(ctx, id)
		},
		DependencyWalk: lkeClusterDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("LKECluster"),
	})
}

// NewLinodeLKEClusterRecycleTool creates a tool for recycling all nodes in an LKE cluster.
func NewLinodeLKEClusterRecycleTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_cluster_recycle",
		"Recycles all nodes in an LKE cluster. WARNING: This causes temporary disruption as all nodes are replaced."+
			" Pass dry_run=true to preview without recycling.",
		toolschemas.Schema("linode.mcp.v1.LKEClusterRecycleInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterRecycleRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// lkeClusterActionSpec describes a single-cluster POST action (recycle,
// regenerate) so the dry-run + confirm + execute flow stays in one place.
type lkeClusterActionSpec struct {
	ToolName       string
	Verb           string
	ConfirmMessage string
	FailureFormat  string
	Execute        func(ctx context.Context, c *linode.Client, clusterID int) error

	// SuccessProto builds the proto-canonical success body from the cluster ID,
	// routing output through MarshalProtoToolResponse so it matches the Python
	// side byte for byte. Every cluster action sets it.
	SuccessProto func(clusterID int) proto.Message
}

// runLKEClusterAction wires dry-run preview, confirm gating, and execution
// for cluster-scoped POST actions. dry_run fetches the cluster (never any
// credential the action might rotate) and previews the POST without firing.
func runLKEClusterAction(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, spec *lkeClusterActionSpec) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, spec.ToolName, httpMethodPost,
			fmt.Sprintf(lkeClustersPath+"/%d/"+spec.Verb, clusterID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKECluster(ctx, clusterID)
			})
	}

	// Recycle and regenerate destroy running nodes/credentials, so they
	// carry the full destroy gate, not just a confirm check.
	if result := requireDestroyConfirmation(ctx, request, spec.ToolName, spec.ConfirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := spec.Execute(ctx, client, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(spec.FailureFormat, clusterID, err)), nil
	}

	return MarshalProtoToolResponse(spec.SuccessProto(clusterID))
}

func handleLKEClusterRecycleRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runLKEClusterAction(ctx, request, cfg, &lkeClusterActionSpec{
		ToolName:       "linode_lke_cluster_recycle",
		Verb:           "recycle",
		ConfirmMessage: "Recycles all nodes in the cluster. This causes temporary disruption. Set confirm=true to proceed.",
		FailureFormat:  "Failed to recycle LKE cluster %d: %v",
		Execute: func(ctx context.Context, c *linode.Client, clusterID int) error {
			return c.RecycleLKECluster(ctx, clusterID)
		},
		SuccessProto: func(clusterID int) proto.Message {
			return &linodev1.LKEClusterActionResponse{
				Message:   fmt.Sprintf("LKE cluster %d recycle initiated successfully", clusterID),
				ClusterId: linodeIDToInt32(clusterID),
			}
		},
	})
}

// NewLinodeLKEClusterRegenerateTool creates a tool for regenerating an LKE cluster's service token.
func NewLinodeLKEClusterRegenerateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_cluster_regenerate",
		"Regenerates the service token for an LKE cluster. Existing tokens will stop working."+
			" Pass dry_run=true to preview without regenerating.",
		toolschemas.Schema("linode.mcp.v1.LKEClusterRegenerateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEClusterRegenerateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLKEClusterRegenerateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runLKEClusterAction(ctx, request, cfg, &lkeClusterActionSpec{
		ToolName:       "linode_lke_cluster_regenerate",
		Verb:           "regenerate",
		ConfirmMessage: "This regenerates the cluster service token. Existing tokens will stop working. Set confirm=true to proceed.",
		FailureFormat:  "Failed to regenerate service token for LKE cluster %d: %v",
		Execute: func(ctx context.Context, c *linode.Client, clusterID int) error {
			return c.RegenerateLKECluster(ctx, clusterID)
		},
		SuccessProto: func(clusterID int) proto.Message {
			return &linodev1.LKEClusterActionResponse{
				Message:   fmt.Sprintf("Service token for LKE cluster %d regenerated successfully", clusterID),
				ClusterId: linodeIDToInt32(clusterID),
			}
		},
	})
}

// NewLinodeLKEPoolCreateTool creates a tool for adding a node pool to an LKE cluster.
func NewLinodeLKEPoolCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_pool_create",
		"Creates a new node pool in an LKE cluster. WARNING: This creates billable compute resources."+
			" Pass dry_run=true to preview without creating.",
		toolschemas.Schema("linode.mcp.v1.LKENodePoolCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLKEPoolCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
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

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_lke_pool_create", httpMethodPost,
			fmt.Sprintf(lkeClustersPath+"/%d/pools", clusterID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKECluster(ctx, clusterID)
			})
	}

	if result := RequireConfirm(request, "This creates billable compute resources. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req := linode.CreateLKENodePoolRequest{
		Type:  nodeType,
		Count: count,
	}

	if raw, ok := request.GetArguments()["autoscaler"]; ok {
		autoscalerObj, isObject := raw.(map[string]any)
		if !isObject {
			return mcp.NewToolResultError("autoscaler must be an object"), nil
		}

		enabled, _ := autoscalerObj["enabled"].(bool)
		minNodes, _ := numberArgToInt(autoscalerObj["min"])
		maxNodes, _ := numberArgToInt(autoscalerObj["max"])
		req.Autoscaler = &linode.LKENodePoolAutoscaler{Enabled: enabled, Min: minNodes, Max: maxNodes}
	}

	if result := applyOptionalTags(request, &req.Tags); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pool, err := client.CreateLKENodePoolProto(ctx, clusterID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create node pool in cluster %d: %v", clusterID, err)), nil
	}

	response := &linodev1.LKENodePoolWriteResponse{
		Message: fmt.Sprintf("Node pool (ID: %d) created in cluster %d with %d %s node(s)", pool.GetId(), clusterID, pool.GetCount(), pool.GetType()),
		Pool:    pool,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeLKEPoolUpdateTool creates a tool for updating an LKE node pool.
func NewLinodeLKEPoolUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_pool_update",
		"Updates a node pool in an LKE cluster. Can change node count, autoscaler settings, or tags."+
			" Pass dry_run=true to preview without modifying.",
		toolschemas.Schema("linode.mcp.v1.LKENodePoolUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLKEPoolUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	poolID := request.GetInt("pool_id", 0)
	if poolID == 0 {
		return mcp.NewToolResultError("pool_id is required"), nil
	}

	if IsDryRun(request) {
		_, countProvided := request.GetArguments()["count"]
		_, autoscalerProvided := request.GetArguments()["autoscaler"]

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_lke_pool_update", "PUT",
			fmt.Sprintf(lkeClustersPath+"/%d/pools/%d", clusterID, poolID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKENodePool(ctx, clusterID, poolID)
			},
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return lkePoolUpdateSideEffects(ctx, state,
					request.GetInt("count", 0), countProvided, autoscalerProvided)
			})
	}

	if result := RequireConfirm(request, "This modifies the node pool configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req := linode.UpdateLKENodePoolRequest{}

	if _, ok := request.GetArguments()["count"]; ok {
		count := request.GetInt("count", 0)
		req.Count = &count
	}

	if raw, ok := request.GetArguments()["autoscaler"]; ok {
		autoscalerObj, isObject := raw.(map[string]any)
		if !isObject {
			return mcp.NewToolResultError("autoscaler must be an object"), nil
		}

		enabled, _ := autoscalerObj["enabled"].(bool)
		minNodes, _ := numberArgToInt(autoscalerObj["min"])
		maxNodes, _ := numberArgToInt(autoscalerObj["max"])
		req.Autoscaler = &linode.LKENodePoolAutoscaler{Enabled: enabled, Min: minNodes, Max: maxNodes}
	}

	if result := applyOptionalTags(request, &req.Tags); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pool, err := client.UpdateLKENodePoolProto(ctx, clusterID, poolID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify node pool %d in cluster %d: %v", poolID, clusterID, err)), nil
	}

	response := &linodev1.LKENodePoolWriteResponse{
		Message: fmt.Sprintf("Node pool %d in cluster %d modified successfully", poolID, clusterID),
		Pool:    pool,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeLKEPoolDeleteTool creates a tool for deleting a node pool from an LKE cluster.
func NewLinodeLKEPoolDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_pool_delete",
		"Deletes a node pool from an LKE cluster. All nodes in the pool will be removed."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.LKENodePoolDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// lkePoolDeleteProto builds the proto-canonical id-echo body for a successful
// node-pool delete, keeping the proto literal off the handler's struct literal
// so the delete handlers stay below the dupl threshold.
func lkePoolDeleteProto(clusterID, poolID int) proto.Message {
	return &linodev1.LKENodePoolDeleteResponse{
		Message:   fmt.Sprintf("Node pool %d deleted from cluster %d successfully", poolID, clusterID),
		ClusterId: linodeIDToInt32(clusterID),
		PoolId:    linodeIDToInt32(poolID),
	}
}

func handleLKEPoolDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_lke_pool_delete",
		OuterIDParam:   paramClusterID,
		InnerIDParam:   "pool_id",
		Method:         httpMethodDelete,
		PathPattern:    "/lke/clusters/%d/pools/%d",
		ConfirmMessage: "This deletes the node pool and all its nodes. Set confirm=true to proceed.",
		SuccessProto:   lkePoolDeleteProto,
		FetchState: func(ctx context.Context, c *linode.Client, clusterID, poolID int) (any, error) {
			return c.GetLKENodePool(ctx, clusterID, poolID)
		},
		Execute: func(ctx context.Context, c *linode.Client, clusterID, poolID int) error {
			return c.DeleteLKENodePool(ctx, clusterID, poolID)
		},
		DependencyWalk: lkePoolDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("LKENodePool"),
	})
}

// NewLinodeLKEPoolRecycleTool creates a tool for recycling all nodes in an LKE node pool.
func NewLinodeLKEPoolRecycleTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_pool_recycle",
		"Recycles all nodes in a specific LKE node pool. Nodes will be replaced with new ones."+
			" Pass dry_run=true to preview without recycling.",
		toolschemas.Schema("linode.mcp.v1.LKEPoolRecycleInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEPoolRecycleRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLKEPoolRecycleRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		clusterID := request.GetInt(paramClusterID, 0)
		if clusterID == 0 {
			return mcp.NewToolResultError("cluster_id is required"), nil
		}

		poolID := request.GetInt("pool_id", 0)
		if poolID == 0 {
			return mcp.NewToolResultError("pool_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_lke_pool_recycle", httpMethodPost,
			fmt.Sprintf(lkeClustersPath+"/%d/pools/%d/recycle", clusterID, poolID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKENodePool(ctx, clusterID, poolID)
			})
	}

	return handleLKESubResourceAction(
		ctx, request, cfg,
		"linode_lke_pool_recycle",
		"This recycles all nodes in the pool, causing temporary disruption. Set confirm=true to proceed.",
		"pool_id",
		func(r *mcp.CallToolRequest) (int, bool) {
			id := r.GetInt("pool_id", 0)

			return id, id != 0
		},
		func(ctx context.Context, c *linode.Client, clusterID, poolID int) error {
			return c.RecycleLKENodePool(ctx, clusterID, poolID)
		},
		"Failed to recycle node pool %d in cluster %d: %v",
		func(clusterID, poolID int) proto.Message {
			return &linodev1.LKEPoolRecycleResponse{
				Message:   fmt.Sprintf("Node pool %d in cluster %d recycle initiated successfully", poolID, clusterID),
				ClusterId: linodeIDToInt32(clusterID),
				PoolId:    linodeIDToInt32(poolID),
			}
		},
	)
}

// NewLinodeLKENodeDeleteTool creates a tool for deleting a specific node from an LKE cluster.
func NewLinodeLKENodeDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_node_delete",
		"Deletes a specific node from an LKE cluster. The node will be removed and may be replaced depending on pool settings."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.LKENodeDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKENodeDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLKENodeDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	nodeID := request.GetString("node_id", "")
	if nodeID == "" {
		return mcp.NewToolResultError("node_id is required"), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_lke_node_delete",
		Method:         httpMethodDelete,
		Path:           fmt.Sprintf("/lke/clusters/%d/nodes/%s", clusterID, nodeID),
		ConfirmMessage: "This deletes the specified node. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetLKENode(ctx, clusterID, nodeID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteLKENode(ctx, clusterID, nodeID)
		},
		Success: func() proto.Message {
			return &linodev1.LKENodeDeleteResponse{
				Message:   fmt.Sprintf("Node %s deleted from cluster %d successfully", nodeID, clusterID),
				ClusterId: linodeIDToInt32(clusterID),
				NodeId:    nodeID,
			}
		},
		DependencyWalk: lkeNodeDeleteDependencyWalk,
		// An LKE node record carries no cosmetic timestamp, so the whole
		// state is hashed; the unknown "LKENode" key returns nil.
		HashIgnore: twostage.HashIgnoreFields("LKENode"),
	})
}

// NewLinodeLKENodeRecycleTool creates a tool for recycling a specific node in an LKE cluster.
func NewLinodeLKENodeRecycleTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_node_recycle",
		"Recycles a specific node in an LKE cluster. The node will be drained and replaced with a new one."+
			" Pass dry_run=true to preview without recycling.",
		toolschemas.Schema("linode.mcp.v1.LKENodeRecycleInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKENodeRecycleRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLKENodeRecycleRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		clusterID := request.GetInt(paramClusterID, 0)
		if clusterID == 0 {
			return mcp.NewToolResultError("cluster_id is required"), nil
		}

		nodeID := request.GetString("node_id", "")
		if nodeID == "" {
			return mcp.NewToolResultError("node_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_lke_node_recycle", httpMethodPost,
			fmt.Sprintf(lkeClustersPath+"/%d/nodes/%s/recycle", clusterID, nodeID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKENode(ctx, clusterID, nodeID)
			})
	}

	return handleLKESubResourceAction(
		ctx, request, cfg,
		"linode_lke_node_recycle",
		"This recycles the specified node, replacing it with a new one. Set confirm=true to proceed.",
		"node_id",
		func(r *mcp.CallToolRequest) (string, bool) {
			id := r.GetString("node_id", "")

			return id, id != ""
		},
		func(ctx context.Context, c *linode.Client, clusterID int, nodeID string) error {
			return c.RecycleLKENode(ctx, clusterID, nodeID)
		},
		"Failed to recycle node %s in cluster %d: %v",
		func(clusterID int, nodeID string) proto.Message {
			return &linodev1.LKENodeRecycleResponse{
				Message:   fmt.Sprintf("Node %s in cluster %d recycle initiated successfully", nodeID, clusterID),
				ClusterId: linodeIDToInt32(clusterID),
				NodeId:    nodeID,
			}
		},
	)
}

// NewLinodeLKEKubeconfigDeleteTool creates a tool for deleting and regenerating an LKE cluster's kubeconfig.
func NewLinodeLKEKubeconfigDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_kubeconfig_delete",
		"Deletes and regenerates the kubeconfig for an LKE cluster. Existing kubeconfig files will stop working."+
			" Pass dry_run=true to preview without regenerating."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.LKEKubeconfigDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEKubeconfigDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// lkeKubeconfigDeleteProto builds the proto-canonical id-echo body for a
// successful kubeconfig delete, keeping the proto literal off the handler's
// struct literal so the delete handlers stay below the dupl threshold.
func lkeKubeconfigDeleteProto(clusterID int) proto.Message {
	return &linodev1.LKEKubeconfigDeleteResponse{
		Message:   fmt.Sprintf("Kubeconfig for LKE cluster %d deleted and regenerated successfully", clusterID),
		ClusterId: linodeIDToInt32(clusterID),
	}
}

func handleLKEKubeconfigDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_lke_kubeconfig_delete",
		IDParam:        paramClusterID,
		Method:         httpMethodDelete,
		PathPattern:    "/lke/clusters/%d/kubeconfig",
		ConfirmMessage: "This deletes the kubeconfig. Existing kubeconfig files will stop working. Set confirm=true to proceed.",
		SuccessProto:   lkeKubeconfigDeleteProto,
		// Fetch the cluster (not the kubeconfig contents) so dry_run
		// surfaces cluster metadata to the model without exposing the
		// kubeconfig credential material itself.
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetLKECluster(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteLKEKubeconfig(ctx, id)
		},
		HashIgnore: twostage.HashIgnoreFields("LKEKubeconfig"),
	})
}

// NewLinodeLKEServiceTokenDeleteTool creates a tool for deleting and regenerating an LKE cluster's service token.
func NewLinodeLKEServiceTokenDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_service_token_delete",
		"Deletes and regenerates the service token for an LKE cluster. Existing tokens will stop working."+
			" Pass dry_run=true to preview without regenerating."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.LKEServiceTokenDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEServiceTokenDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// lkeServiceTokenDeleteProto builds the proto-canonical id-echo body for a
// successful service-token delete, keeping the proto literal off the handler's
// struct literal so the delete handlers stay below the dupl threshold.
func lkeServiceTokenDeleteProto(clusterID int) proto.Message {
	return &linodev1.LKEServiceTokenDeleteResponse{
		Message:   fmt.Sprintf("Service token for LKE cluster %d deleted and regenerated successfully", clusterID),
		ClusterId: linodeIDToInt32(clusterID),
	}
}

func handleLKEServiceTokenDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_lke_service_token_delete",
		IDParam:        paramClusterID,
		Method:         httpMethodDelete,
		PathPattern:    "/lke/clusters/%d/servicetoken",
		ConfirmMessage: "This deletes the service token. Existing tokens will stop working. Set confirm=true to proceed.",
		SuccessProto:   lkeServiceTokenDeleteProto,
		// Fetch the cluster (not the service token) so dry_run surfaces
		// cluster metadata without exposing the token credential.
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetLKECluster(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteLKEServiceToken(ctx, id)
		},
		HashIgnore: twostage.HashIgnoreFields("LKEServiceToken"),
	})
}

// NewLinodeLKEACLUpdateTool creates a tool for updating the control plane ACL of an LKE cluster.
func NewLinodeLKEACLUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_acl_update",
		"Updates the control plane ACL for an LKE cluster. Controls which IP addresses can access the cluster's API server."+
			" Pass dry_run=true to preview without modifying.",
		toolschemas.Schema("linode.mcp.v1.LKEACLUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEACLUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// lkeControlPlaneACLFromObject builds the control plane ACL from the native acl
// object argument, reading enabled and the optional addresses.ipv4/ipv6 arrays.
func lkeControlPlaneACLFromObject(aclObj map[string]any) (linode.LKEControlPlaneACL, string) {
	enabled, _ := aclObj["enabled"].(bool)
	acl := linode.LKEControlPlaneACL{Enabled: enabled}

	addresses, ok := aclObj["addresses"].(map[string]any)
	if !ok {
		return acl, ""
	}

	if rawV4, present := addresses["ipv4"]; present {
		ipv4, validationMessage := stringSliceFromToolArg(rawV4, "acl.addresses.ipv4")
		if validationMessage != "" {
			return linode.LKEControlPlaneACL{}, validationMessage
		}

		acl.Addresses.IPv4 = ipv4
	}

	if rawV6, present := addresses["ipv6"]; present {
		ipv6, validationMessage := stringSliceFromToolArg(rawV6, "acl.addresses.ipv6")
		if validationMessage != "" {
			return linode.LKEControlPlaneACL{}, validationMessage
		}

		acl.Addresses.IPv6 = ipv6
	}

	return acl, ""
}

func handleLKEACLUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	aclObj, _ := request.GetArguments()["acl"].(map[string]any)

	if IsDryRun(request) {
		enabledForPreview, _ := aclObj["enabled"].(bool)

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_lke_acl_update", "PUT",
			fmt.Sprintf(lkeClustersPath+"/%d/control_plane_acl", clusterID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKEControlPlaneACL(ctx, clusterID)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return lkeACLUpdateSideEffects(ctx, enabledForPreview)
			})
	}

	if result := RequireConfirm(request, "This modifies the control plane ACL, which controls API server access. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if aclObj == nil {
		return mcp.NewToolResultError("acl is required and must be an object"), nil
	}

	acl, validationMessage := lkeControlPlaneACLFromObject(aclObj)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req := linode.UpdateLKEControlPlaneACLRequest{
		ACL: acl,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := client.UpdateLKEControlPlaneACLProto(ctx, clusterID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify control plane ACL for cluster %d: %v", clusterID, err)), nil
	}

	response := &linodev1.LKEACLWriteResponse{
		Message: fmt.Sprintf("Control plane ACL for cluster %d modified successfully", clusterID),
		Acl:     result,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeLKEACLDeleteTool creates a tool for deleting the control plane ACL of an LKE cluster.
func NewLinodeLKEACLDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_lke_acl_delete",
		"Deletes the control plane ACL for an LKE cluster. This removes all IP restrictions from the API server."+
			" Pass dry_run=true to preview without deleting.",
		toolschemas.Schema("linode.mcp.v1.LKEACLDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLKEACLDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLKEACLDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt(paramClusterID, 0)
	if clusterID == 0 {
		return mcp.NewToolResultError("cluster_id is required"), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_lke_acl_delete", httpMethodDelete,
			fmt.Sprintf(lkeClustersPath+"/%d/control_plane_acl", clusterID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLKEControlPlaneACL(ctx, clusterID)
			})
	}

	if result := requireDestroyConfirmation(ctx, request, "linode_lke_acl_delete", "This removes all IP restrictions from the API server. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteLKEControlPlaneACL(ctx, clusterID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove control plane ACL for cluster %d: %v", clusterID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.LKEACLDeleteResponse{
		Message:   fmt.Sprintf("Control plane ACL for cluster %d removed successfully", clusterID),
		ClusterId: linodeIDToInt32(clusterID),
	})
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

package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeNodeBalancerCreateTool creates a tool for creating a NodeBalancer.
func NewLinodeNodeBalancerCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_nodebalancer_create",
		"Creates a new NodeBalancer (load balancer). WARNING: Billing starts immediately. Use linode_region_list to find valid regions.",
		toolschemas.Schema("linode.mcp.v1.NodeBalancerCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeNodeBalancerCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	clientConnThrottle := request.GetInt("client_conn_throttle", 0)

	if IsDryRun(request) {
		if region == "" {
			return mcp.NewToolResultError("region is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_nodebalancer_create", httpMethodPost, "/nodebalancers", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return nodebalancerCreateSideEffects(ctx, label, region)
			})
	}

	if result := RequireConfirm(request, "This operation creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateNodeBalancerRequest{
		Region:             region,
		Label:              label,
		ClientConnThrottle: clientConnThrottle,
	}

	nodeBalancer, err := client.CreateNodeBalancerProto(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create NodeBalancer: %v", err)), nil
	}

	response := &linodev1.NodeBalancerWriteResponse{
		Message:      fmt.Sprintf("NodeBalancer '%s' (ID: %d) created successfully in %s", nodeBalancer.GetLabel(), nodeBalancer.GetId(), nodeBalancer.GetRegion()),
		Nodebalancer: nodeBalancer,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeNodeBalancerUpdateTool creates a tool for updating a NodeBalancer.
func NewLinodeNodeBalancerUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_nodebalancer_update",
		"Updates an existing NodeBalancer. Can modify label and connection throttle.",
		toolschemas.Schema("linode.mcp.v1.NodeBalancerUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// notProvided signals that an optional numeric parameter was not included in the request.
const notProvided = -1

func handleLinodeNodeBalancerUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)
	label := request.GetString("label", "")
	clientConnThrottle := request.GetInt("client_conn_throttle", notProvided)

	if IsDryRun(request) {
		if nodeBalancerID == 0 {
			return mcp.NewToolResultError("nodebalancer_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_nodebalancer_update", "PUT",
			fmt.Sprintf("/nodebalancers/%d", nodeBalancerID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetNodeBalancer(ctx, nodeBalancerID)
			},
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return nodebalancerUpdateSideEffects(ctx, state, label, clientConnThrottle)
			})
	}

	if result := RequireConfirm(request, "This updates a NodeBalancer. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if nodeBalancerID == 0 {
		return mcp.NewToolResultError("nodebalancer_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateNodeBalancerRequest{
		Label: label,
	}

	if clientConnThrottle >= 0 {
		req.ClientConnThrottle = &clientConnThrottle
	}

	nodeBalancer, err := client.UpdateNodeBalancerProto(ctx, nodeBalancerID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	response := &linodev1.NodeBalancerWriteResponse{
		Message:      fmt.Sprintf("NodeBalancer %d modified successfully", nodeBalancerID),
		Nodebalancer: nodeBalancer,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeNodeBalancerDeleteTool creates a tool for deleting a NodeBalancer.
func NewLinodeNodeBalancerDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_nodebalancer_delete",
		"Deletes a NodeBalancer. WARNING: This will remove the load balancer and all its configurations."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.NodeBalancerDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// nodebalancerDeleteProto builds the proto-canonical id-echo body for a
// successful NodeBalancer delete, keeping the proto literal off the handler's
// struct literal so the delete handlers stay below the dupl threshold.
func nodebalancerDeleteProto(id int) proto.Message {
	return &linodev1.NodeBalancerDeleteResponse{
		Message:        fmt.Sprintf("NodeBalancer %d removed successfully", id),
		NodebalancerId: linodeIDToInt32(id),
	}
}

func handleLinodeNodeBalancerDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_nodebalancer_delete",
		IDParam:        "nodebalancer_id",
		Method:         httpMethodDelete,
		PathPattern:    "/nodebalancers/%d",
		ConfirmMessage: destroyConfirmMessage,
		SuccessProto:   nodebalancerDeleteProto,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetNodeBalancer(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteNodeBalancer(ctx, id)
		},
		DependencyWalk: nodebalancerDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("NodeBalancer"),
	})
}

package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeNodeBalancerCreateTool creates a tool for creating a NodeBalancer.
func NewLinodeNodeBalancerCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_create",
		"Creates a new NodeBalancer (load balancer). WARNING: Billing starts immediately. Use linode_region_list to find valid regions.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region where the NodeBalancer will be created (e.g., 'us-east')")),
			mcp.WithString("label", mcp.Description("A label for the NodeBalancer (optional)")),
			mcp.WithNumber("client_conn_throttle",
				mcp.Description("Connections per second throttle limit (0-20). Default is 0 (no throttle).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm creation. This operation incurs billing charges. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNodeBalancerCreateRequest,
	)

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

	nodeBalancer, err := client.CreateNodeBalancer(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create NodeBalancer: %v", err)), nil
	}

	response := struct {
		Message      string               `json:"message"`
		NodeBalancer *linode.NodeBalancer `json:"nodebalancer"`
	}{
		Message:      fmt.Sprintf("NodeBalancer '%s' (ID: %d) created successfully in %s", nodeBalancer.Label, nodeBalancer.ID, nodeBalancer.Region),
		NodeBalancer: nodeBalancer,
	}

	return MarshalToolResponse(response)
}

// NewLinodeNodeBalancerUpdateTool creates a tool for updating a NodeBalancer.
func NewLinodeNodeBalancerUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_update",
		"Updates an existing NodeBalancer. Can modify label and connection throttle.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer to update")),
			mcp.WithString("label", mcp.Description("New label for the NodeBalancer (optional)")),
			mcp.WithNumber("client_conn_throttle",
				mcp.Description("New connections per second throttle limit (0-20) (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm NodeBalancer update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNodeBalancerUpdateRequest,
	)

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

	nodeBalancer, err := client.UpdateNodeBalancer(ctx, nodeBalancerID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	response := struct {
		Message      string               `json:"message"`
		NodeBalancer *linode.NodeBalancer `json:"nodebalancer"`
	}{
		Message:      fmt.Sprintf("NodeBalancer %d modified successfully", nodeBalancerID),
		NodeBalancer: nodeBalancer,
	}

	return MarshalToolResponse(response)
}

// NewLinodeNodeBalancerDeleteTool creates a tool for deleting a NodeBalancer.
func NewLinodeNodeBalancerDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_delete",
		"Deletes a NodeBalancer. WARNING: This will remove the load balancer and all its configurations."+
			" Pass dry_run=true to preview without deleting.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNodeBalancerDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeNodeBalancerDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_nodebalancer_delete",
		IDParam:        "nodebalancer_id",
		Method:         httpMethodDelete,
		PathPattern:    "/nodebalancers/%d",
		ConfirmMessage: destroyConfirmMessage,
		SuccessFormat:  "NodeBalancer %d removed successfully",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetNodeBalancer(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteNodeBalancer(ctx, id)
		},
		DependencyWalk: nodebalancerDeleteDependencyWalk,
	})
}

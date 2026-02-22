package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeNodeBalancerCreateTool creates a tool for creating a NodeBalancer.
func NewLinodeNodeBalancerCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_nodebalancer_create",
		mcp.WithDescription("Creates a new NodeBalancer (load balancer). WARNING: Billing starts immediately. Use linode_regions_list to find valid regions."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the NodeBalancer will be created (e.g., 'us-east')"),
		),
		mcp.WithString("label",
			mcp.Description("A label for the NodeBalancer (optional)"),
		),
		mcp.WithNumber("client_conn_throttle",
			mcp.Description("Connections per second throttle limit (0-20). Default is 0 (no throttle)."),
		),
		mcp.WithBoolean(paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm creation. This operation incurs billing charges."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerCreateRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeNodeBalancerCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This operation creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	region := request.GetString("region", "")
	label := request.GetString("label", "")
	clientConnThrottle := request.GetInt("client_conn_throttle", 0)

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

	return marshalToolResponse(response)
}

// NewLinodeNodeBalancerUpdateTool creates a tool for updating a NodeBalancer.
func NewLinodeNodeBalancerUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_nodebalancer_update",
		mcp.WithDescription("Updates an existing NodeBalancer. Can modify label and connection throttle."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("nodebalancer_id",
			mcp.Required(),
			mcp.Description("The ID of the NodeBalancer to update"),
		),
		mcp.WithString("label",
			mcp.Description("New label for the NodeBalancer (optional)"),
		),
		mcp.WithNumber("client_conn_throttle",
			mcp.Description("New connections per second throttle limit (0-20) (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerUpdateRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeNodeBalancerUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)
	label := request.GetString("label", "")
	clientConnThrottle := request.GetInt("client_conn_throttle", -1) // -1 indicates not provided

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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	response := struct {
		Message      string               `json:"message"`
		NodeBalancer *linode.NodeBalancer `json:"nodebalancer"`
	}{
		Message:      fmt.Sprintf("NodeBalancer %d updated successfully", nodeBalancerID),
		NodeBalancer: nodeBalancer,
	}

	return marshalToolResponse(response)
}

// NewLinodeNodeBalancerDeleteTool creates a tool for deleting a NodeBalancer.
func NewLinodeNodeBalancerDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_nodebalancer_delete",
		mcp.WithDescription("Deletes a NodeBalancer. WARNING: This will remove the load balancer and all its configurations."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("nodebalancer_id",
			mcp.Required(),
			mcp.Description("The ID of the NodeBalancer to delete"),
		),
		mcp.WithBoolean(paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerDeleteRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeNodeBalancerDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This operation is destructive. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	nodeBalancerID := request.GetInt("nodebalancer_id", 0)

	if nodeBalancerID == 0 {
		return mcp.NewToolResultError("nodebalancer_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteNodeBalancer(ctx, nodeBalancerID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	response := struct {
		Message        string `json:"message"`
		NodeBalancerID int    `json:"nodebalancer_id"`
	}{
		Message:        fmt.Sprintf("NodeBalancer %d deleted successfully", nodeBalancerID),
		NodeBalancerID: nodeBalancerID,
	}

	return marshalToolResponse(response)
}

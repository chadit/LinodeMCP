package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeNodeBalancersListTool creates a tool for listing NodeBalancers.
func NewLinodeNodeBalancersListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_nodebalancers_list",
		"Lists all NodeBalancers on your account. Can filter by region or label.",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.NodeBalancer, error) {
			return client.ListNodeBalancers(ctx)
		},
		[]listFilterParam[linode.NodeBalancer]{
			fieldFilter("region", "Filter by region ID (e.g., us-east, eu-west)",
				func(n linode.NodeBalancer) string { return n.Region }),
			containsFilter("label_contains", "Filter NodeBalancers by label containing this string (case-insensitive)",
				func(n linode.NodeBalancer) string { return n.Label }),
		},
		"nodebalancers",
	)
}

// NewLinodeNodeBalancerGetTool creates a tool for getting a single NodeBalancer.
func NewLinodeNodeBalancerGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_nodebalancer_get",
		mcp.WithDescription("Gets detailed information about a specific NodeBalancer by its ID."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("nodebalancer_id",
			mcp.Required(),
			mcp.Description("The ID of the NodeBalancer to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeNodeBalancerGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)

	if nodeBalancerID == 0 {
		return mcp.NewToolResultError("nodebalancer_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeBalancer, err := client.GetNodeBalancer(ctx, nodeBalancerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	return marshalToolResponse(nodeBalancer)
}

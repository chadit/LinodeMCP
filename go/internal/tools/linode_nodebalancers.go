package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeNodeBalancerTypesTool creates a tool for listing available NodeBalancer types.
func NewLinodeNodeBalancerTypesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_types",
		"Lists available NodeBalancer types.",
		nil,
		handleLinodeNodeBalancerTypesRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeNodeBalancerTypesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	types, listFailureMessage := listNodeBalancerTypes(ctx, client)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_nodebalancer_types: " + listFailureMessage), nil
	}

	return MarshalToolResponse(types)
}

func listNodeBalancerTypes(ctx context.Context, client *linode.Client) (*linode.PaginatedResponse[linode.NodeBalancerType], string) {
	types, err := client.ListNodeBalancerTypes(ctx)
	if err != nil {
		return nil, err.Error()
	}

	return types, ""
}

// NewLinodeNodeBalancerListTool creates a tool for listing NodeBalancers.
func NewLinodeNodeBalancerListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newListTool(
		cfg,
		"linode_nodebalancer_list",
		"Lists all NodeBalancers on your account. Can filter by region or label.",
		func(ctx context.Context, client *linode.Client) ([]linode.NodeBalancer, error) {
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

	return tool, profiles.CapRead, handler
}

// NewLinodeInstanceNodeBalancerListTool creates a tool for listing NodeBalancers assigned to a Linode instance.
func NewLinodeInstanceNodeBalancerListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_nodebalancer_list",
		"Lists NodeBalancers assigned to a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleLinodeInstanceNodeBalancerListRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerConfigListTool creates a tool for listing configs on a NodeBalancer.
func NewLinodeNodeBalancerConfigListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_list",
		"Lists configs for a specific NodeBalancer by its ID.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer whose configs should be listed")),
		},
		handleLinodeNodeBalancerConfigListRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerGetTool creates a tool for getting a single NodeBalancer.
func NewLinodeNodeBalancerGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_nodebalancer_get",
		mcp.WithDescription("Gets detailed information about a specific NodeBalancer by its ID."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"nodebalancer_id",
			mcp.Required(),
			mcp.Description("The ID of the NodeBalancer to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeInstanceNodeBalancerListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeBalancers, err := client.ListInstanceNodeBalancers(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list NodeBalancers for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count         int                   `json:"count"`
		NodeBalancers []linode.NodeBalancer `json:"nodebalancers"`
	}{
		Count:         len(nodeBalancers),
		NodeBalancers: nodeBalancers,
	}

	return MarshalToolResponse(response)
}

func handleLinodeNodeBalancerConfigListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list configs for NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	response := struct {
		Count   int                         `json:"count"`
		Configs []linode.NodeBalancerConfig `json:"configs"`
	}{
		Count:   len(configs),
		Configs: configs,
	}

	return MarshalToolResponse(response)
}

func nodeBalancerIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["nodebalancer_id"]; !exists {
		return 0, "nodebalancer_id is required"
	}

	nodeBalancerID, validationMessage := optionalPaginationInt(args, "nodebalancer_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return nodeBalancerID, ""
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

	return MarshalToolResponse(nodeBalancer)
}

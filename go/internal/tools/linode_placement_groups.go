package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	placementGroupsPageSizeMin = 25
	placementGroupsPageSizeMax = 500
)

// NewLinodePlacementGroupListTool creates a tool for listing placement groups.
func NewLinodePlacementGroupListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_placement_groups_list",
		mcp.WithDescription("Lists placement groups for the authenticated account with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handlePlacementGroupsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handlePlacementGroupsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := placementGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	placementGroups, err := client.ListPlacementGroups(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve placement groups: %v", err)), nil
	}

	return FormatListResponse(placementGroups.Data, nil, "placement_groups")
}

func placementGroupsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", placementGroupsPageSizeMin, placementGroupsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

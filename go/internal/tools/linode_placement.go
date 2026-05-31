package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodePlacementGroupGetTool creates a tool for retrieving a single placement group by ID.
func NewLinodePlacementGroupGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_placement_group_get",
		"Retrieves details of a single placement group by its ID",
		[]mcp.ToolOption{
			mcp.WithString(
				"group_id",
				mcp.Required(),
				mcp.Description("The ID of the placement group to retrieve"),
			),
		},
		handlePlacementGroupGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handlePlacementGroupGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, err := parsePlacementGroupID(request.GetString("group_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	group, err := client.GetPlacementGroup(ctx, groupID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve placement group %d: %v", groupID, err)), nil
	}

	return MarshalToolResponse(group)
}

func parsePlacementGroupID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrPlacementGroupIDRequired
	}

	groupID, err := strconv.Atoi(raw)
	if err != nil || groupID <= 0 {
		return 0, ErrPlacementGroupIDPositive
	}

	return groupID, nil
}

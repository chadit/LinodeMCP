package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodePlacementGroupDeleteTool creates a tool for deleting a placement group by ID.
func NewLinodePlacementGroupDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_placement_group_delete",
		"Deletes a placement group by its ID. This destructive action requires confirm=true.",
		[]mcp.ToolOption{
			mcp.WithString(
				"group_id",
				mcp.Required(),
				mcp.Description("The ID of the placement group to delete"),
			),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm placement group deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handlePlacementGroupDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

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

func handlePlacementGroupDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, err := parsePlacementGroupID(request.GetString("group_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_placement_group_delete",
		Method:         httpMethodDelete,
		Path:           "/placement/groups/" + strconv.Itoa(groupID),
		ConfirmMessage: "confirm=true is required to delete the placement group",
		FetchState: func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetPlacementGroup(ctx, groupID)
		},
		Execute: func(ctx context.Context, client *linode.Client) error {
			return client.DeletePlacementGroup(ctx, groupID)
		},
		Success: func() any {
			return map[string]any{responseKeyMessage: "Placement group " + strconv.Itoa(groupID) + " deleted successfully"}
		},
	})
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

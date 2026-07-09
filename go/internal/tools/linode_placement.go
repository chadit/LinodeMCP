package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodePlacementGroupAssignTool creates a tool for assigning Linodes to a placement group.
func NewLinodePlacementGroupAssignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_placement_group_assign",
		"Assigns one or more Linodes to a placement group.",
		toolschemas.Schema("linode.mcp.v1.PlacementGroupAssignInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handlePlacementGroupAssignRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handlePlacementGroupAssignRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, validationMessage := placementGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	linodes, validationMessage := parsePlacementGroupLinodes(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	endpoint := fmt.Sprintf("/placement/groups/%d/assign", groupID)
	req := linode.AssignPlacementGroupLinodesRequest{Linodes: linodes}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, "linode_placement_group_assign", httpMethodPost, endpoint, req,
			func(ctx context.Context, client *linode.Client) (any, error) {
				return client.GetPlacementGroup(ctx, groupID)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return placementGroupMembershipSideEffects(ctx, linodes, groupID, "assigned to")
			})
	}

	if result := RequireConfirm(request, "This assigns Linodes to a placement group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	group, err := client.AssignPlacementGroupLinodesProto(ctx, groupID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to assign Linodes to placement group %d: %v", groupID, err)), nil
	}

	response := &linodev1.PlacementGroupWriteResponse{
		Message:        fmt.Sprintf("Assigned %d Linode(s) to placement group %d", len(linodes), groupID),
		PlacementGroup: group,
	}

	return MarshalProtoToolResponse(response)
}

func parsePlacementGroupLinodes(request *mcp.CallToolRequest) ([]int, string) {
	raw, exists := request.GetArguments()["linodes"]
	if !exists {
		return nil, ErrPlacementGroupLinodesRequired.Error()
	}

	rawLinodes, ok := raw.([]any)
	if !ok {
		return nil, ErrPlacementGroupLinodesJSON.Error()
	}

	if len(rawLinodes) == 0 {
		return nil, ErrPlacementGroupLinodesEmpty.Error()
	}

	linodes := make([]int, 0, len(rawLinodes))
	seen := make(map[int]struct{}, len(rawLinodes))

	for _, rawLinode := range rawLinodes {
		linodeID, ok := numberArgToInt(rawLinode)
		if !ok || linodeID <= 0 {
			return nil, ErrPlacementGroupLinodesPositive.Error()
		}

		if _, exists := seen[linodeID]; exists {
			return nil, ErrPlacementGroupLinodesDuplicate.Error()
		}

		seen[linodeID] = struct{}{}
		linodes = append(linodes, linodeID)
	}

	return linodes, ""
}

// NewLinodePlacementGroupDeleteTool creates a tool for deleting a placement group by ID.
func NewLinodePlacementGroupDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_placement_group_delete",
		"Deletes a placement group by its ID. This destructive action requires confirm=true."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.PlacementGroupDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handlePlacementGroupDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// NewLinodePlacementGroupGetTool creates a tool for retrieving a single placement group by ID.
func NewLinodePlacementGroupGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_placement_group_get",
		"Retrieves details of a single placement group by its ID",
		toolschemas.Schema("linode.mcp.v1.PlacementGroupGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handlePlacementGroupGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handlePlacementGroupGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, validationMessage := placementGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	group, err := client.GetPlacementGroupProto(ctx, groupID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve placement group %d: %v", groupID, err)), nil
	}

	return MarshalProtoToolResponse(group)
}

func handlePlacementGroupDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, validationMessage := placementGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_placement_group_delete",
		Method:         httpMethodDelete,
		Path:           "/placement/groups/" + strconv.Itoa(groupID),
		ConfirmMessage: "confirm=true is required to delete the placement group",
		FetchState: func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetPlacementGroup(ctx, groupID)
		},
		DependencyWalk: placementGroupDeleteDependencyWalk,
		Execute: func(ctx context.Context, client *linode.Client) error {
			return client.DeletePlacementGroup(ctx, groupID)
		},
		Success: func() proto.Message {
			return &linodev1.PlacementGroupDeleteResponse{
				Message: "Placement group " + strconv.Itoa(groupID) + " deleted successfully",
			}
		},
	})
}

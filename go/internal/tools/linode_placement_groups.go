package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	placementGroupsPageSizeMin = 25
	placementGroupsPageSizeMax = 500

	placementGroupIDParam           = "group_id"
	placementGroupLabelParam        = "label"
	placementGroupRegionParam       = "region"
	placementGroupTypeParam         = "placement_group_type"
	placementGroupPolicyParam       = "placement_group_policy"
	placementGroupTypeAntiAffinity  = "anti_affinity:local"
	placementGroupPolicyStrict      = "strict"
	placementGroupPolicyFlexible    = "flexible"
	errPlacementGroupTypeRequired   = "placement_group_type is required"
	errPlacementGroupTypeNonEmpty   = "placement_group_type must be a non-empty string"
	errPlacementGroupPolicyRequired = "placement_group_policy is required"
	errPlacementGroupPolicyNonEmpty = "placement_group_policy must be a non-empty string"
	errPlacementGroupTypeInvalid    = "placement_group_type must be anti_affinity:local"
	errPlacementGroupPolicyInvalid  = "placement_group_policy must be strict or flexible"
	placementGroupLinodesParam      = "linodes"
)

// NewLinodePlacementGroupListTool creates a tool for listing placement groups.
func NewLinodePlacementGroupListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_placement_group_list",
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

	client, prepareFailure := prepareClient(request, cfg)
	if prepareFailure != nil {
		return mcp.NewToolResultError(prepareFailure.Error()), nil
	}

	placementGroups, err := client.ListPlacementGroups(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve placement groups: %v", err)), nil
	}

	return FormatListResponse(placementGroups.Data, nil, "placement_groups")
}

// NewLinodePlacementGroupUpdateTool creates a tool for updating a placement group label.
func NewLinodePlacementGroupUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_placement_group_update",
		mcp.WithDescription("Updates one placement group label by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(placementGroupIDParam, mcp.Required(), mcp.Description("Placement group ID to update.")),
		mcp.WithString(placementGroupLabelParam, mcp.Required(), mcp.Description("New placement group label.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm placement group update.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handlePlacementGroupUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handlePlacementGroupUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, validationMessage := placementGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateRequest, validationMessage := placementGroupUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_placement_group_update", "PUT",
			fmt.Sprintf("/placement/groups/%d", groupID), nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return placementGroupUpdateSideEffects(ctx, updateRequest.Label)
			})
	}

	if result := RequireConfirm(request, "This updates a placement group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepareFailure := prepareClient(request, cfg)
	if prepareFailure != nil {
		return mcp.NewToolResultError(prepareFailure.Error()), nil
	}

	placementGroup, updateFailure := client.UpdatePlacementGroup(ctx, groupID, updateRequest)
	if updateFailure == nil {
		return MarshalToolResponse(placementGroup)
	}

	return mcp.NewToolResultError("Failed to update linode_placement_group_update: " + updateFailure.Error()), nil
}

func placementGroupIDFromTool(request *mcp.CallToolRequest) (int, string) {
	groupID, validationMessage := optionalPaginationInt(request.GetArguments(), placementGroupIDParam, 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	if groupID == 0 {
		return 0, placementGroupIDParam + " is required"
	}

	return groupID, ""
}

func placementGroupUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdatePlacementGroupRequest, string) {
	label, validationMessage := nonEmptyToolString(request.GetArguments()[placementGroupLabelParam], placementGroupLabelParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.UpdatePlacementGroupRequest{Label: label}, ""
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

// NewLinodePlacementGroupCreateTool creates a tool for creating placement groups.
func NewLinodePlacementGroupCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_placement_group_create",
		"Creates a Linode placement group.",
		[]mcp.ToolOption{
			mcp.WithString(placementGroupLabelParam, mcp.Required(), mcp.Description("Placement group label.")),
			mcp.WithString(placementGroupRegionParam, mcp.Required(), mcp.Description("Region where the placement group is created.")),
			mcp.WithString(placementGroupTypeParam, mcp.Required(), mcp.Description("Placement group type. Currently anti_affinity:local.")),
			mcp.WithString(placementGroupPolicyParam, mcp.Required(), mcp.Description("Placement group policy: strict or flexible.")),
			mcp.WithBoolean(paramDryRun, mcp.Description("Preview placement group creation without creating it.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm placement group creation. Ignored when dry_run=true.")),
		},
		handleLinodePlacementGroupCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodePlacementGroupCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label, validationMessage := requiredTrimmedString(request, placementGroupLabelParam, errLabelRequired, "label must be a non-empty string")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	region, validationMessage := requiredTrimmedString(request, placementGroupRegionParam, "region is required", "region must be a non-empty string")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	placementGroupType, validationMessage := requiredTrimmedString(request, placementGroupTypeParam, errPlacementGroupTypeRequired, errPlacementGroupTypeNonEmpty)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if placementGroupType != placementGroupTypeAntiAffinity {
		return mcp.NewToolResultError(errPlacementGroupTypeInvalid), nil
	}

	placementGroupPolicy, validationMessage := requiredTrimmedString(request, placementGroupPolicyParam, errPlacementGroupPolicyRequired, errPlacementGroupPolicyNonEmpty)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if placementGroupPolicy != placementGroupPolicyStrict && placementGroupPolicy != placementGroupPolicyFlexible {
		return mcp.NewToolResultError(errPlacementGroupPolicyInvalid), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_placement_group_create", "POST", "/placement/groups", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return placementGroupCreateSideEffects(ctx, label, region, placementGroupType, placementGroupPolicy)
			})
	}

	if result := RequireConfirm(request, "This creates a placement group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreatePlacementGroupRequest{
		Label:                label,
		Region:               region,
		PlacementGroupType:   placementGroupType,
		PlacementGroupPolicy: placementGroupPolicy,
	}

	placementGroup, err := client.CreatePlacementGroup(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create placement group: %v", err)), nil
	}

	response := struct {
		Message        string                 `json:"message"`
		PlacementGroup *linode.PlacementGroup `json:"placement_group"`
	}{
		Message:        fmt.Sprintf("Placement group '%s' created successfully", placementGroup.Label),
		PlacementGroup: placementGroup,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format placement group response: %v", err)), nil
	}

	return result, nil
}

func requiredTrimmedString(request *mcp.CallToolRequest, name, missingMessage, invalidMessage string) (string, string) {
	raw, found := request.GetArguments()[name]
	if !found {
		return "", missingMessage
	}

	value, valid := raw.(string)
	if !valid {
		return "", invalidMessage
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", invalidMessage
	}

	return value, ""
}

// NewLinodePlacementGroupUnassignTool creates a tool for unassigning Linodes from a placement group.
func NewLinodePlacementGroupUnassignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_placement_group_unassign",
		"Unassigns Linodes from a placement group.",
		[]mcp.ToolOption{
			mcp.WithString("group_id", mcp.Required(), mcp.Description("The ID of the placement group.")),
			mcp.WithArray(placementGroupLinodesParam, mcp.Required(), mcp.Description("Linode IDs to unassign from the placement group.")),
			mcp.WithBoolean(paramDryRun, mcp.Description("Preview placement group unassignment without changing it.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm placement group unassignment. Ignored when dry_run=true.")),
		},
		handleLinodePlacementGroupUnassignRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodePlacementGroupUnassignRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, err := parsePlacementGroupID(request.GetString("group_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	linodes, validationMessage := parsePlacementGroupLinodes(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	path := fmt.Sprintf("/placement/groups/%d/unassign", groupID)

	req := linode.PlacementGroupUnassignRequest{Linodes: linodes}
	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, "linode_placement_group_unassign", httpMethodPost, path, req,
			func(ctx context.Context, client *linode.Client) (any, error) {
				return client.GetPlacementGroup(ctx, groupID)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return placementGroupMembershipSideEffects(ctx, linodes, groupID, "removed from")
			})
	}

	if result := RequireConfirm(request, "This unassigns Linodes from a placement group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	placementGroup, err := client.UnassignPlacementGroup(ctx, groupID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to unassign placement group %d: %v", groupID, err)), nil
	}

	response := struct {
		Message        string                 `json:"message"`
		PlacementGroup *linode.PlacementGroup `json:"placement_group"`
	}{
		Message:        fmt.Sprintf("Linodes unassigned from placement group %d successfully", groupID),
		PlacementGroup: placementGroup,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format placement group response: %v", err)), nil
	}

	return result, nil
}

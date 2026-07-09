package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
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
	errPlacementGroupTypeRequired   = "placement_group_type is required"
	errPlacementGroupTypeNonEmpty   = "placement_group_type must be a non-empty string"
	errPlacementGroupPolicyRequired = "placement_group_policy is required"
	errPlacementGroupPolicyNonEmpty = "placement_group_policy must be a non-empty string"
	errPlacementGroupTypeInvalid    = "placement_group_type must be anti_affinity:local"
	errPlacementGroupLabelPattern   = "label must start and end with an alphanumeric character and contain only alphanumeric characters, hyphens, underscores, or periods"
)

// placementGroupLabelPattern mirrors Python's _LABEL_PATTERN so both languages
// reject a label that does not start/end alphanumeric or uses characters other
// than letters, digits, hyphens, underscores, or periods (strictest-wins).
var placementGroupLabelPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

// NewLinodePlacementGroupListTool creates a tool for listing placement groups.
func NewLinodePlacementGroupListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_placement_group_list",
		"Lists placement groups for the authenticated account with optional pagination.",
		"linode.mcp.v1.PlacementGroupListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.PlacementGroup, error) {
			return client.ListPlacementGroupsProto(ctx, page, pageSize)
		},
		placementGroupsPaginationFromTool,
		nil,
		placementGroupListResponse,
	)

	return tool, profiles.CapRead, handler
}

func placementGroupListResponse(items []*linodev1.PlacementGroup, count int32, filter *string) *linodev1.PlacementGroupListResponse {
	return &linodev1.PlacementGroupListResponse{Count: count, Filter: filter, PlacementGroups: items}
}

// NewLinodePlacementGroupUpdateTool creates a tool for updating a placement group label.
func NewLinodePlacementGroupUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_placement_group_update",
		"Updates one placement group label by ID.",
		toolschemas.Schema("linode.mcp.v1.PlacementGroupUpdateInput"),
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

	placementGroup, updateFailure := client.UpdatePlacementGroupProto(ctx, groupID, updateRequest)
	if updateFailure == nil {
		return MarshalProtoToolResponse(placementGroup)
	}

	return mcp.NewToolResultError("Failed to update linode_placement_group_update: " + updateFailure.Error()), nil
}

func placementGroupIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredIDArgument(request, placementGroupIDParam)
}

func placementGroupUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdatePlacementGroupRequest, string) {
	label, validationMessage := nonEmptyToolString(request.GetArguments()[placementGroupLabelParam], placementGroupLabelParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if !placementGroupLabelPattern.MatchString(label) {
		return nil, errPlacementGroupLabelPattern
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
	tool := mcp.NewToolWithRawSchema(
		"linode_placement_group_create",
		"Creates a Linode placement group.",
		toolschemas.Schema("linode.mcp.v1.PlacementGroupCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodePlacementGroupCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodePlacementGroupCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label, validationMessage := requiredTrimmedString(request, placementGroupLabelParam, errLabelRequired, "label must be a non-empty string")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if !placementGroupLabelPattern.MatchString(label) {
		return mcp.NewToolResultError(errPlacementGroupLabelPattern), nil
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

	if message := enumChoiceError(placementGroupPolicy, placementGroupPolicyParam, linodev1.PlacementGroupPolicy_Value_value); message != "" {
		return mcp.NewToolResultError(message), nil
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

	placementGroup, err := client.CreatePlacementGroupProto(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create placement group: %v", err)), nil
	}

	response := &linodev1.PlacementGroupWriteResponse{
		Message:        fmt.Sprintf("Placement group '%s' created successfully", placementGroup.GetLabel()),
		PlacementGroup: placementGroup,
	}

	result, err := MarshalProtoToolResponse(response)
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
	tool := mcp.NewToolWithRawSchema(
		"linode_placement_group_unassign",
		"Unassigns Linodes from a placement group.",
		toolschemas.Schema("linode.mcp.v1.PlacementGroupUnassignInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodePlacementGroupUnassignRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodePlacementGroupUnassignRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	groupID, validationMessage := placementGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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

	placementGroup, err := client.UnassignPlacementGroupProto(ctx, groupID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to unassign placement group %d: %v", groupID, err)), nil
	}

	response := &linodev1.PlacementGroupWriteResponse{
		Message:        fmt.Sprintf("Linodes unassigned from placement group %d successfully", groupID),
		PlacementGroup: placementGroup,
	}

	result, err := MarshalProtoToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format placement group response: %v", err)), nil
	}

	return result, nil
}

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	placementGroupsPageSizeMin = 25
	placementGroupsPageSizeMax = 500

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
		return RunDryRunPreview(ctx, request, cfg, "linode_placement_group_create", "POST", "/placement/groups", nil)
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

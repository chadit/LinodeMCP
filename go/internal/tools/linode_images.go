package tools

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

const (
	imageShareGroupsPageSizeMin  = 25
	imageShareGroupsPageSizeMax  = 500
	imageIDPrefixPrivate         = "private"
	errImageShareGroupIDPositive = "sharegroup_id must be a positive integer"
	errImageIDPrivateIdentifier  = "image_id must be a private image identifier like private/12345"
)

var (
	imageShareGroupImageIDSlugPattern = regexp.MustCompile(`^private/[1-9]\d*$`)
	imageShareGroupTokenUUIDPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// NewLinodeImageListTool creates a tool for listing Linode images.
func NewLinodeImageListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_list",
		mcp.WithDescription("Lists all available Linode images (OS images and custom images) with optional filtering by type, public status, or deprecated status"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("type", mcp.Description("Filter images by type (manual, automatic)")),
		mcp.WithString("is_public", mcp.Description("Filter by public status (true, false)")),
		mcp.WithString("deprecated", mcp.Description("Filter by deprecated status (true, false)")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListRequest(
			ctx, &request, cfg,
			func(ctx context.Context, client *linode.Client) ([]linode.Image, error) {
				return client.ListImages(ctx)
			},
			[]filterDef[linode.Image]{
				{"type", func(items []linode.Image, v string) []linode.Image {
					return FilterByField(items, v, func(img linode.Image) string { return img.Type })
				}},
				{"is_public", filterImagesByPublic},
				{"deprecated", filterImagesByDeprecated},
			},
			func(items []linode.Image, appliedFilters []string) (*mcp.CallToolResult, error) {
				return FormatListResponse(items, appliedFilters, "images")
			},
		)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageGetTool creates a tool for retrieving one Linode image.
func NewLinodeImageGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_get",
		mcp.WithDescription("Gets one Linode image by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("image_id", mcp.Required(), mcp.Description("Image ID, such as linode/debian11, private/123, or shared/123.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// newImageTwoStageDeleteTool builds a two-stage-capable image-family delete
// tool. The resource-specific identifier option (image_id, sharegroup_id,
// token_uuid) is passed in so the otherwise-identical constructors route
// through one builder and stay below dupl's threshold.
func newImageTwoStageDeleteTool(
	cfg *config.Config,
	name string,
	description string,
	idOption mcp.ToolOption,
	confirmDescription string,
	handle func(context.Context, *mcp.CallToolRequest, *config.Config) (*mcp.CallToolResult, error),
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		name,
		mcp.WithDescription(description+twoStageNote),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		idOption,
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description(confirmDescription)),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
		mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handle(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// NewLinodeImageDeleteTool creates a tool for deleting a private image.
func NewLinodeImageDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newImageTwoStageDeleteTool(
		cfg,
		"linode_image_delete",
		"Deletes a private image by image ID. WARNING: this cannot be undone and replicated instances are also deleted. Pass dry_run=true to preview without deleting.",
		mcp.WithString("image_id", mcp.Required(), mcp.Description("The image ID to delete, for example private/12345.")),
		"Must be true to confirm image deletion. Ignored when dry_run=true.",
		handleImageDeleteRequest,
	)
}

// NewLinodeImageShareGroupsListTool creates a tool for listing image share groups.
func NewLinodeImageShareGroupsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_list",
		mcp.WithDescription("Lists owned image share groups with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupGetTool creates a tool for retrieving one image share group.
func NewLinodeImageShareGroupGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_get",
		mcp.WithDescription("Gets a single image share group by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("Image share group ID.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupsByImageListTool creates a tool for listing share groups that contain an image.
func NewLinodeImageShareGroupsByImageListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_by_image_list",
		mcp.WithDescription("Lists owned image share groups that currently include a private image."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("image_id", mcp.Required(), mcp.Description("Private image ID, for example private/12345.")),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupsByImageListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupImagesListTool creates a tool for listing images shared in an owned image share group.
func NewLinodeImageShareGroupImagesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_image_list",
		mcp.WithDescription("Lists images shared in an owned image share group."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("Image share group ID.")),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupImagesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupMembersListTool creates a tool for listing members linked to an owned image share group.
func NewLinodeImageShareGroupMembersListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_member_list",
		mcp.WithDescription("Lists members linked to an owned image share group."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("Image share group ID.")),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupMembersListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupMemberTokenGetTool creates a tool for retrieving one share group member token as the owner.
func NewLinodeImageShareGroupMemberTokenGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_member_token_get",
		mcp.WithDescription("Gets details for one membership token in an owned image share group."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("Image share group ID.")),
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group member token UUID.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupMemberTokenGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupTokensListTool creates a tool for listing image share group tokens.
func NewLinodeImageShareGroupTokensListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_list",
		mcp.WithDescription("Lists image share group tokens for the authenticated user with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupTokensListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupDeleteTool creates a tool for deleting an owned image share group.
func NewLinodeImageShareGroupDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newImageTwoStageDeleteTool(
		cfg,
		"linode_image_sharegroup_delete",
		"Deletes an owned image share group by ID. WARNING: members lose access to images in the group. Pass dry_run=true to preview without deleting.",
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric ID of the image share group to delete.")),
		"Must be true to confirm share group deletion. Ignored when dry_run=true.",
		handleImageShareGroupDeleteRequest,
	)
}

// NewLinodeImageShareGroupImageDeleteTool creates a tool for revoking a shared image from an owned image share group.
func NewLinodeImageShareGroupImageDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_image_delete",
		"Revokes access to one shared image in an owned image share group. Pass dry_run=true to preview without removing.",
		[]mcp.ToolOption{
			mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID.")),
			mcp.WithNumber("image_id", mcp.Required(), mcp.Description("The numeric shared image ID to remove from the group.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm shared image removal. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleImageShareGroupImageDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

// NewLinodeImageShareGroupTokenGetTool creates a tool for retrieving one image share group token.
func NewLinodeImageShareGroupTokenGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_get",
		mcp.WithDescription("Gets a single image share group token by token UUID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group token UUID.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupTokenGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupTokenDeleteTool creates a tool for removing one image share group token.
func NewLinodeImageShareGroupTokenDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newImageTwoStageDeleteTool(
		cfg,
		"linode_image_sharegroup_token_delete",
		"Removes a single image share group membership token by token UUID. Pass dry_run=true to preview without removing.",
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group token UUID.")),
		"Must be true to confirm token removal. Ignored when dry_run=true.",
		handleImageShareGroupTokenDeleteRequest,
	)
}

// NewLinodeImageShareGroupMemberTokenDeleteTool creates a tool for revoking one accepted image share group membership token.
func NewLinodeImageShareGroupMemberTokenDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_member_token_delete",
		"Revokes an accepted image share group membership token from an owned share group. Pass dry_run=true to preview without revoking.",
		[]mcp.ToolOption{
			mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID.")),
			mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group member token UUID.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm member token revocation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleImageShareGroupMemberTokenDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

// NewLinodeImageShareGroupTokenImagesListTool creates a tool for listing images available through an image share group token.
func NewLinodeImageShareGroupTokenImagesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_image_list",
		mcp.WithDescription("Lists images available through an image share group token."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group token UUID.")),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupTokenImagesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeImageShareGroupByTokenGetTool creates a tool for retrieving a token's share group.
func NewLinodeImageShareGroupByTokenGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_by_token_get",
		mcp.WithDescription("Gets a share group by membership token UUID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group token UUID.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupByTokenGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleImageDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	imageID, validationMessage := privateImageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_image_delete",
		Method:         httpMethodDelete,
		Path:           "/images/" + imageID,
		ConfirmMessage: "confirm=true is required to delete the image",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetImage(ctx, imageID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteImage(ctx, imageID)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: fmt.Sprintf("Image %s deleted successfully", imageID),
			}
		},
	})
}

func privateImageIDFromTool(request *mcp.CallToolRequest) (string, string) {
	imageID := strings.TrimSpace(request.GetString("image_id", ""))
	if imageID == "" {
		return "", "image_id must be a non-empty string"
	}

	if strings.ContainsAny(imageID, "?#") || hasTraversalSegment(imageID) || strings.HasPrefix(imageID, "/") || strings.HasSuffix(imageID, "/") || strings.Contains(imageID, "//") {
		return "", errImageIDPrivateIdentifier
	}

	segments := strings.Split(imageID, "/")
	if len(segments) != 2 || segments[0] != imageIDPrefixPrivate {
		return "", errImageIDPrivateIdentifier
	}

	if !isPositiveDecimalString(segments[1]) {
		return "", errImageIDPrivateIdentifier
	}

	return imageID, ""
}

func isPositiveDecimalString(value string) bool {
	if value == "" {
		return false
	}

	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	imageNumber, err := strconv.Atoi(value)

	return err == nil && imageNumber > 0
}

func hasTraversalSegment(value string) bool {
	return slices.Contains(strings.Split(value, "/"), "..")
}

func handleImageShareGroupsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := imageShareGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	shareGroups, err := client.ListImageShareGroups(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share groups: %v", err)), nil
	}

	return FormatListResponse(shareGroups.Data, nil, "image_sharegroups")
}

func handleImageShareGroupGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	shareGroup, err := client.GetImageShareGroup(ctx, shareGroupID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group: %v", err)), nil
	}

	return MarshalToolResponse(shareGroup)
}

func handleImageGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	imageID, validationMessage := imageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	image, err := client.GetImage(ctx, imageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image: %v", err)), nil
	}

	return MarshalToolResponse(image)
}

func handleImageShareGroupsByImageListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	imageID, validationMessage := imageShareGroupSourceImageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := imageShareGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	shareGroups, err := client.ListImageShareGroupsByImage(ctx, imageID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share groups by image: %v", err)), nil
	}

	return FormatListResponse(shareGroups.Data, nil, "image_sharegroups")
}

func handleImageShareGroupImagesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := imageShareGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	images, err := client.ListImagesByShareGroup(ctx, shareGroupID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group images: %v", err)), nil
	}

	return FormatListResponse(images.Data, nil, "images")
}

func handleImageShareGroupMembersListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := imageShareGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	members, err := client.ListMembersByImageShareGroup(ctx, shareGroupID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group members: %v", err)), nil
	}

	return FormatListResponse(members.Data, nil, "image_sharegroup_members")
}

func handleImageShareGroupMemberTokenGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	member, err := client.GetImageShareGroupMemberToken(ctx, shareGroupID, tokenUUID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group member token: %v", err)), nil
	}

	return MarshalToolResponse(member)
}

func handleImageShareGroupTokensListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := imageShareGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tokens, err := client.ListImageShareGroupTokens(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group tokens: %v", err)), nil
	}

	return FormatListResponse(tokens.Data, nil, "image_sharegroup_tokens")
}

func handleImageShareGroupDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID := request.GetInt("sharegroup_id", 0)
	if shareGroupID <= 0 {
		return mcp.NewToolResultError("sharegroup_id must be a positive integer"), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_image_sharegroup_delete",
		Method:         httpMethodDelete,
		Path:           fmt.Sprintf("/images/sharegroups/%d", shareGroupID),
		ConfirmMessage: "confirm=true is required to delete the image share group",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetImageShareGroup(ctx, shareGroupID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteImageShareGroup(ctx, shareGroupID)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: fmt.Sprintf("Image share group %d removed successfully", shareGroupID),
			}
		},
		HashIgnore: twostage.HashIgnoreFields("ImageShareGroup"),
	})
}

// runImageShareGroupChildDestroy runs the destroy flow for a child of an
// image share group (a shared image, or a member/membership token). The
// dry-run preview fetches the PARENT share group: the children have no
// single-GET, and for token children this avoids surfacing the secret.
func runImageShareGroupChildDestroy(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, path, confirmMessage, successMessage string,
	shareGroupID int,
	execute func(ctx context.Context, c *linode.Client) error,
) (*mcp.CallToolResult, error) {
	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       toolName,
		Method:         httpMethodDelete,
		Path:           path,
		ConfirmMessage: confirmMessage,
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetImageShareGroup(ctx, shareGroupID)
		},
		Execute: execute,
		Success: func() any {
			return map[string]any{responseKeyMessage: successMessage}
		},
	})
}

func handleImageShareGroupImageDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	imageID, validationMessage := imageShareGroupImageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runImageShareGroupChildDestroy(
		ctx, request, cfg,
		"linode_image_sharegroup_image_delete",
		fmt.Sprintf("/images/sharegroups/%d/images/%d", shareGroupID, imageID),
		"confirm=true is required to remove the shared image",
		fmt.Sprintf("Shared image %d removed from image share group %d successfully", imageID, shareGroupID),
		shareGroupID,
		func(ctx context.Context, c *linode.Client) error {
			return c.DeleteImageShareGroupImage(ctx, shareGroupID, imageID)
		},
	)
}

func handleImageShareGroupTokenGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	token, err := client.GetImageShareGroupToken(ctx, tokenUUID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group token: %v", err)), nil
	}

	return MarshalToolResponse(token)
}

func handleImageShareGroupTokenImagesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := imageShareGroupsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	images, err := client.ListImagesByShareGroupToken(ctx, tokenUUID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group token images: %v", err)), nil
	}

	return FormatListResponse(images.Data, nil, "images")
}

func handleImageShareGroupTokenDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_image_sharegroup_token_delete",
		Method:         httpMethodDelete,
		Path:           "/images/sharegroups/tokens/" + tokenUUID,
		ConfirmMessage: "confirm=true is required to remove the share group token",
		// Credential safety: resolve the token to its PARENT share group
		// rather than fetching the token entity, so dry-run never surfaces
		// the token secret to the model.
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetImageShareGroupByToken(ctx, tokenUUID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteImageShareGroupToken(ctx, tokenUUID)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: "Image share group token removed successfully",
			}
		},
		HashIgnore: twostage.HashIgnoreFields("ImageShareGroupToken"),
	})
}

func handleImageShareGroupMemberTokenDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runImageShareGroupChildDestroy(
		ctx, request, cfg,
		"linode_image_sharegroup_member_token_delete",
		fmt.Sprintf("/images/sharegroups/%d/members/%s", shareGroupID, tokenUUID),
		"confirm=true is required to revoke the member token",
		fmt.Sprintf("Image share group member token %s revoked from share group %d successfully", tokenUUID, shareGroupID),
		shareGroupID,
		func(ctx context.Context, c *linode.Client) error {
			return c.DeleteImageShareGroupMemberToken(ctx, shareGroupID, tokenUUID)
		},
	)
}

func handleImageShareGroupByTokenGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	shareGroup, err := client.GetImageShareGroupByToken(ctx, tokenUUID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve image share group by token: %v", err)), nil
	}

	return MarshalToolResponse(shareGroup)
}

func imageIDFromTool(request *mcp.CallToolRequest) (string, string) {
	imageID, validationMessage := requiredStringArg(request.GetArguments(), "image_id")
	if validationMessage != "" {
		return "", validationMessage
	}

	parts := strings.Split(imageID, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "image_id must be a prefixed image ID such as linode/debian11, private/123, or shared/123"
	}

	switch parts[0] {
	case "linode", imageIDPrefixPrivate, "shared":
	default:
		return "", "image_id prefix must be linode, private, or shared"
	}

	if strings.ContainsAny(imageID, "?#") || strings.Contains(imageID, "..") {
		return "", "image_id must not contain query separators, fragments, or traversal segments"
	}

	return imageID, ""
}

func imageShareGroupIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["sharegroup_id"]
	if !exists {
		return 0, "sharegroup_id must be a positive integer"
	}

	shareGroupID, ok := numberArgToInt(raw)
	if !ok || shareGroupID <= 0 {
		return 0, errImageShareGroupIDPositive
	}

	return shareGroupID, ""
}

func imageShareGroupSourceImageIDFromTool(request *mcp.CallToolRequest) (string, string) {
	imageID, validationMessage := requiredStringArg(request.GetArguments(), "image_id")
	if validationMessage != "" {
		return "", validationMessage
	}

	if strings.ContainsAny(imageID, "?#") || strings.Contains(imageID, "..") {
		return "", "image_id must not contain query separators, fragments, or traversal segments"
	}

	if !imageShareGroupImageIDSlugPattern.MatchString(imageID) {
		return "", errImageIDPrivateIdentifier
	}

	return imageID, ""
}

func imageShareGroupImageIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["image_id"]
	if !exists {
		return 0, "image_id must be a positive integer"
	}

	imageID, ok := numberArgToInt(raw)
	if !ok || imageID <= 0 {
		return 0, "image_id must be a positive integer"
	}

	return imageID, ""
}

func imageShareGroupTokenUUIDFromTool(request *mcp.CallToolRequest) (string, string) {
	tokenUUID, validationMessage := requiredStringArg(request.GetArguments(), "token_uuid")
	if validationMessage != "" {
		return "", validationMessage
	}

	if strings.ContainsAny(tokenUUID, "/?#") || strings.Contains(tokenUUID, "..") {
		return "", "token_uuid must not contain path separators, query separators, fragments, or traversal segments"
	}

	if !imageShareGroupTokenUUIDPattern.MatchString(tokenUUID) {
		return "", "token_uuid must be a UUID"
	}

	return tokenUUID, ""
}

func imageShareGroupsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", imageShareGroupsPageSizeMin, imageShareGroupsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func filterImagesByPublic(images []linode.Image, isPublicFilter string) []linode.Image {
	filtered := make([]linode.Image, 0, len(images))

	wantPublic := strings.EqualFold(isPublicFilter, boolTrue)

	for i := range images {
		if images[i].IsPublic == wantPublic {
			filtered = append(filtered, images[i])
		}
	}

	return filtered
}

func filterImagesByDeprecated(images []linode.Image, deprecatedFilter string) []linode.Image {
	filtered := make([]linode.Image, 0, len(images))

	wantDeprecated := strings.EqualFold(deprecatedFilter, boolTrue)

	for i := range images {
		if images[i].Deprecated == wantDeprecated {
			filtered = append(filtered, images[i])
		}
	}

	return filtered
}

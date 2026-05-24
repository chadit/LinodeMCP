package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	imageShareGroupsPageSizeMin = 25
	imageShareGroupsPageSizeMax = 500
)

var imageShareGroupTokenUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

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

// NewLinodeImageShareGroupsListTool creates a tool for listing image share groups.
func NewLinodeImageShareGroupsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroups_list",
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

// NewLinodeImageShareGroupTokensListTool creates a tool for listing image share group tokens.
func NewLinodeImageShareGroupTokensListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_tokens_list",
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
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_delete",
		mcp.WithDescription("Removes a single image share group membership token by token UUID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("Image share group token UUID.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm token removal.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleImageShareGroupTokenDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// NewLinodeImageShareGroupTokenImagesListTool creates a tool for listing images available through an image share group token.
func NewLinodeImageShareGroupTokenImagesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_images_list",
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
	if confirmResult := RequireConfirm(request, "confirm=true is required"); confirmResult != nil {
		return confirmResult, nil
	}

	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteImageShareGroupToken(ctx, tokenUUID); err != nil {
		return mcp.NewToolResultError(formatImageShareGroupTokenDeleteError(err)), nil
	}

	return mcp.NewToolResultText("Image share group token removed successfully"), nil
}

func formatImageShareGroupTokenDeleteError(err error) string {
	return "Failed to remove image share group token: " + err.Error()
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

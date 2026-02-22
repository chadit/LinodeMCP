package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeImagesListTool creates a tool for listing Linode images.
func NewLinodeImagesListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_images_list",
		mcp.WithDescription("Lists all available Linode images (OS images and custom images) with optional filtering by type, public status, or deprecated status"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("type", mcp.Description("Filter images by type (manual, automatic)")),
		mcp.WithString("is_public", mcp.Description("Filter by public status (true, false)")),
		mcp.WithString("deprecated", mcp.Description("Filter by deprecated status (true, false)")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListRequest(ctx, &request, cfg,
			func(ctx context.Context, client *linode.RetryableClient) ([]linode.Image, error) {
				return client.ListImages(ctx)
			},
			[]filterDef[linode.Image]{
				{"type", func(items []linode.Image, v string) []linode.Image {
					return filterByField(items, v, func(img linode.Image) string { return img.Type })
				}},
				{"is_public", filterImagesByPublic},
				{"deprecated", filterImagesByDeprecated},
			},
			func(items []linode.Image, appliedFilters []string) (*mcp.CallToolResult, error) {
				return formatListResponse(items, appliedFilters, "images")
			},
		)
	}

	return tool, handler
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

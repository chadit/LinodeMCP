package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeImagesListTool creates a tool for listing Linode images.
func NewLinodeImagesListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_images_list",
		mcp.WithDescription("Lists all available Linode images (OS images and custom images) with optional filtering by type, public status, or deprecated status"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("type",
			mcp.Description("Filter images by type (manual, automatic)"),
		),
		mcp.WithString("is_public",
			mcp.Description("Filter by public status (true, false)"),
		),
		mcp.WithString("deprecated",
			mcp.Description("Filter by deprecated status (true, false)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImagesListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeImagesListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	typeFilter := request.GetString("type", "")
	isPublicFilter := request.GetString("is_public", "")
	deprecatedFilter := request.GetString("deprecated", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	images, err := client.ListImages(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode images: %v", err)), nil
	}

	if typeFilter != "" {
		images = filterImagesByType(images, typeFilter)
	}

	if isPublicFilter != "" {
		images = filterImagesByPublic(images, isPublicFilter)
	}

	if deprecatedFilter != "" {
		images = filterImagesByDeprecated(images, deprecatedFilter)
	}

	return formatImagesResponse(images, typeFilter, isPublicFilter, deprecatedFilter)
}

func filterImagesByType(images []linode.Image, typeFilter string) []linode.Image {
	var filtered []linode.Image

	typeFilter = strings.ToLower(typeFilter)

	for _, image := range images {
		if strings.ToLower(image.Type) == typeFilter {
			filtered = append(filtered, image)
		}
	}

	return filtered
}

func filterImagesByPublic(images []linode.Image, isPublicFilter string) []linode.Image {
	var filtered []linode.Image

	wantPublic := strings.ToLower(isPublicFilter) == "true"

	for _, image := range images {
		if image.IsPublic == wantPublic {
			filtered = append(filtered, image)
		}
	}

	return filtered
}

func filterImagesByDeprecated(images []linode.Image, deprecatedFilter string) []linode.Image {
	var filtered []linode.Image

	wantDeprecated := strings.ToLower(deprecatedFilter) == "true"

	for _, image := range images {
		if image.Deprecated == wantDeprecated {
			filtered = append(filtered, image)
		}
	}

	return filtered
}

func formatImagesResponse(images []linode.Image, typeFilter, isPublicFilter, deprecatedFilter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count  int            `json:"count"`
		Filter string         `json:"filter,omitempty"`
		Images []linode.Image `json:"images"`
	}{
		Count:  len(images),
		Images: images,
	}

	var filters []string
	if typeFilter != "" {
		filters = append(filters, "type="+typeFilter)
	}

	if isPublicFilter != "" {
		filters = append(filters, "is_public="+isPublicFilter)
	}

	if deprecatedFilter != "" {
		filters = append(filters, "deprecated="+deprecatedFilter)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

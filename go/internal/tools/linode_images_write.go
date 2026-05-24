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

// NewLinodeImageCreateTool creates a tool for creating private Linode images.
func NewLinodeImageCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_create",
		mcp.WithDescription("Creates a private Linode image from an existing Linode disk."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("disk_id", mcp.Required(), mcp.Description("The ID of the Linode disk to image.")),
		mcp.WithString("label", mcp.Description("Short label for the new image (optional).")),
		mcp.WithString("description", mcp.Description("Detailed description for the new image (optional).")),
		mcp.WithBoolean("cloud_init", mcp.Description("Whether the image supports cloud-init (optional).")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags for the image (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm private image creation.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeImageShareGroupTokenCreateTool creates a tool for creating image share group membership tokens.
func NewLinodeImageShareGroupTokenCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_create",
		mcp.WithDescription("Creates a single-use image share group membership token. The response includes token material that is only useful if handled carefully."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("valid_for_sharegroup_uuid", mcp.Required(), mcp.Description("The UUID of the share group this token is valid for.")),
		mcp.WithString("label", mcp.Description("Optional descriptive label for the token.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm image share group token creation.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupTokenCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

func handleLinodeImageShareGroupTokenCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates single-use image share group token material. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	shareGroupUUID := strings.TrimSpace(request.GetString("valid_for_sharegroup_uuid", ""))
	if shareGroupUUID == "" {
		return mcp.NewToolResultError("valid_for_sharegroup_uuid is required"), nil
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateImageShareGroupTokenRequest{
		Label:                  request.GetString("label", ""),
		ValidForShareGroupUUID: shareGroupUUID,
	}

	created, err := client.CreateImageShareGroupToken(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create image share group token: %v", err)), nil
	}

	response := struct {
		Message string                       `json:"message"`
		Token   *linode.ImageShareGroupToken `json:"token"`
	}{
		Message: fmt.Sprintf("Image share group token '%s' created successfully", created.TokenUUID),
		Token:   created,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group token response: %v", err)), nil
	}

	return result, nil
}

func handleLinodeImageCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	confirm, confirmOK := request.GetArguments()[paramConfirm].(bool)
	if !confirmOK || !confirm {
		return mcp.NewToolResultError("This creates a private image from a Linode disk. Set confirm=true to proceed."), nil
	}

	diskID := request.GetInt("disk_id", 0)
	if diskID <= 0 {
		return mcp.NewToolResultError(ErrDiskIDRequired.Error()), nil
	}

	tagsRaw := request.GetString("tags", "")

	var tags []string

	for {
		tag, rest, found := strings.Cut(tagsRaw, ",")
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			tags = append(tags, trimmed)
		}

		if !found {
			break
		}

		tagsRaw = rest
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateImageRequest{
		DiskID:      diskID,
		Label:       request.GetString("label", ""),
		Description: request.GetString("description", ""),
		CloudInit:   request.GetBool("cloud_init", false),
		Tags:        tags,
	}

	created, err := client.CreateImage(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create image: %v", err)), nil
	}

	response := struct {
		Message string        `json:"message"`
		Image   *linode.Image `json:"image"`
	}{
		Message: fmt.Sprintf("Image '%s' (%s) created successfully", created.Label, created.ID),
		Image:   created,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image response: %v", err)), nil
	}

	return result, nil
}

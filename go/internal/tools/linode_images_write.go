package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeImageShareGroupCreateTool creates a tool for creating image share groups.
func NewLinodeImageShareGroupCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_create",
		mcp.WithDescription("Creates a share group for sharing images with other users."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("label", mcp.Required(), mcp.Description("The share group's descriptive name.")),
		mcp.WithString("description", mcp.Description("Detailed description for the share group (optional).")),
		mcp.WithString("images", mcp.Description("JSON array of images to include, each with required id and optional label/description.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm image share group creation.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	confirm, confirmOK := request.GetArguments()[paramConfirm].(bool)
	if !confirmOK || !confirm {
		return mcp.NewToolResultError("This creates an image share group. Set confirm=true to proceed."), nil
	}

	label := strings.TrimSpace(request.GetString("label", ""))
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	var imagesJSON string

	if imagesArg, ok := request.GetArguments()["images"]; ok {
		var imagesOK bool

		imagesJSON, imagesOK = imagesArg.(string)
		if !imagesOK {
			return mcp.NewToolResultError("images must be a JSON string"), nil
		}
	}

	images, err := imageShareGroupImagesFromTool(imagesJSON)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateImageShareGroupRequest{
		Label:       label,
		Description: request.GetString("description", ""),
		Images:      images,
	}

	created, err := client.CreateImageShareGroup(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create image share group: %v", err)), nil
	}

	response := struct {
		Message    string                  `json:"message"`
		ShareGroup *linode.ImageShareGroup `json:"share_group"`
	}{
		Message:    fmt.Sprintf("Image share group '%s' (%d) created successfully", created.Label, created.ID),
		ShareGroup: created,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group response: %v", err)), nil
	}

	return result, nil
}

func imageShareGroupImagesFromTool(imagesJSON string) ([]linode.ImageShareGroupImage, error) {
	if strings.TrimSpace(imagesJSON) == "" {
		return nil, nil
	}

	var images []linode.ImageShareGroupImage
	if err := json.Unmarshal([]byte(imagesJSON), &images); err != nil {
		return nil, fmt.Errorf("invalid images JSON: %w", err)
	}

	for i := range images {
		images[i].ID = strings.TrimSpace(images[i].ID)
		if images[i].ID == "" {
			return nil, fmt.Errorf("images[%d].id: %w", i, ErrImageShareGroupImageIDRequired)
		}
	}

	return images, nil
}

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

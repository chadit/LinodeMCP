package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

var imageShareGroupImageIDPattern = regexp.MustCompile(`^shared/[1-9]\d*$`)

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

// NewLinodeImageShareGroupImagesAddTool creates a tool for adding images to an image share group.
func NewLinodeImageShareGroupImagesAddTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_images_add",
		mcp.WithDescription("Adds one or more private images to an image share group."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID to add images to.")),
		mcp.WithString("images", mcp.Required(), mcp.Description("JSON array of images to add, each with required id and optional label/description.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm adding images to the share group.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupImagesAddRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupImagesAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This adds images to an image share group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	images, validationMessage := requiredImageShareGroupImagesFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	image, err := client.AddImageShareGroupImages(ctx, shareGroupID, &linode.AddImageShareGroupImagesRequest{Images: images})
	if err != nil {
		return mcp.NewToolResultError(formatImageShareGroupImagesAddError(err)), nil
	}

	response := struct {
		Message string        `json:"message"`
		Image   *linode.Image `json:"image"`
	}{
		Message: fmt.Sprintf("Added image set to image share group %d; last returned image: '%s'", shareGroupID, image.ID),
		Image:   image,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group image response: %v", err)), nil
	}

	return result, nil
}

func requiredImageShareGroupImagesFromTool(request *mcp.CallToolRequest) ([]linode.ImageShareGroupImage, string) {
	imagesArg, imagesPresent := request.GetArguments()["images"]
	if !imagesPresent {
		return nil, "images is required"
	}

	imagesJSON, imagesIsString := imagesArg.(string)
	if !imagesIsString {
		return nil, "images must be a JSON string"
	}

	images, err := imageShareGroupImagesFromTool(imagesJSON)
	if err != nil {
		return nil, err.Error()
	}

	if len(images) == 0 {
		return nil, "images must contain at least one image"
	}

	return images, ""
}

func formatImageShareGroupImagesAddError(err error) string {
	return "Failed to add image to share group: " + err.Error()
}

// NewLinodeImageShareGroupImageUpdateTool creates a tool for updating a shared image.
func NewLinodeImageShareGroupImageUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_image_update",
		mcp.WithDescription("Updates a shared image's label or description within an image share group."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID that contains the shared image.")),
		mcp.WithString("image_id", mcp.Required(), mcp.Description("The shared image ID, for example shared/1.")),
		mcp.WithString("label", mcp.Description("New descriptive name for the shared image (optional).")),
		mcp.WithString("description", mcp.Description("New detailed description for the shared image (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm shared image update.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupImageUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupImageUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates a shared image. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	imageID, validationMessage := imageShareGroupSharedImageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := imageShareGroupImageUpdateFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updated, err := client.UpdateImageShareGroupImage(ctx, shareGroupID, imageID, req)
	if err != nil {
		return mcp.NewToolResultError(formatImageShareGroupImageUpdateError(err)), nil
	}

	response := struct {
		Message string        `json:"message"`
		Image   *linode.Image `json:"image"`
	}{
		Message: fmt.Sprintf("Shared image '%s' in image share group %d updated successfully", updated.ID, shareGroupID),
		Image:   updated,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format shared image response: %v", err)), nil
	}

	return result, nil
}

func imageShareGroupSharedImageIDFromTool(request *mcp.CallToolRequest) (string, string) {
	imageID, validationMessage := requiredStringArg(request.GetArguments(), "image_id")
	if validationMessage != "" {
		return "", validationMessage
	}

	if !imageShareGroupImageIDPattern.MatchString(imageID) {
		return "", "image_id must match shared/<positive integer>"
	}

	return imageID, ""
}

func imageShareGroupImageUpdateFromTool(args map[string]any) (*linode.UpdateImageShareGroupImageRequest, string) {
	req := &linode.UpdateImageShareGroupImageRequest{}

	var hasUpdate bool

	label, hasLabel, validationMessage := optionalStringField(args, "label")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasLabel {
		req.Label = &label
		hasUpdate = true
	}

	description, hasDescription, validationMessage := optionalStringField(args, "description")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasDescription {
		req.Description = &description
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, "at least one of label or description is required"
	}

	return req, ""
}

func formatImageShareGroupImageUpdateError(err error) string {
	return "Failed to update shared image: " + err.Error()
}

// NewLinodeImageShareGroupMembersAddTool creates a tool for adding members to an image share group.
func NewLinodeImageShareGroupMembersAddTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_members_add",
		mcp.WithDescription("Adds members to an image share group using a membership token."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID to add members to.")),
		mcp.WithString("label", mcp.Required(), mcp.Description("Label for the member being added.")),
		mcp.WithString("token", mcp.Required(), mcp.Description("Membership token used to add the member.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm adding members to the share group.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupMembersAddRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupMembersAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This adds members to an image share group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := imageShareGroupMemberAddFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	shareGroup, err := client.AddImageShareGroupMembers(ctx, shareGroupID, req)
	if err != nil {
		return mcp.NewToolResultError(formatImageShareGroupMembersAddError(err)), nil
	}

	response := struct {
		Message    string                  `json:"message"`
		ShareGroup *linode.ImageShareGroup `json:"share_group"`
	}{
		Message:    fmt.Sprintf("Added members to image share group %d", shareGroupID),
		ShareGroup: shareGroup,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group member response: %v", err)), nil
	}

	return result, nil
}

func imageShareGroupMemberAddFromTool(args map[string]any) (*linode.AddImageShareGroupMembersRequest, string) {
	label, labelOK := requiredTrimmedStringArg(args, "label")
	if !labelOK {
		return nil, "label is required"
	}

	token, tokenOK := requiredTrimmedStringArg(args, "token")
	if !tokenOK {
		return nil, "token is required"
	}

	return &linode.AddImageShareGroupMembersRequest{Label: label, Token: token}, ""
}

func requiredTrimmedStringArg(args map[string]any, name string) (string, bool) {
	raw, exists := args[name]
	if !exists {
		return "", false
	}

	value, ok := raw.(string)
	if !ok {
		return "", false
	}

	value = strings.TrimSpace(value)

	return value, value != ""
}

func formatImageShareGroupMembersAddError(err error) string {
	return "Failed to add members to image share group: " + err.Error()
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

// NewLinodeImageShareGroupUpdateTool creates a tool for updating image share groups.
func NewLinodeImageShareGroupUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_update",
		mcp.WithDescription("Updates an image share group's label or description."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID to update.")),
		mcp.WithString("label", mcp.Description("New descriptive name for the share group (optional).")),
		mcp.WithString("description", mcp.Description("New detailed description for the share group (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm image share group update.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates an image share group. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	shareGroupID, validationMessage := imageShareGroupIDFromWriteTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := imageShareGroupUpdateFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updated, err := client.UpdateImageShareGroup(ctx, shareGroupID, req)
	if err != nil {
		return mcp.NewToolResultError(formatImageShareGroupUpdateError(err)), nil
	}

	response := struct {
		Message    string                  `json:"message"`
		ShareGroup *linode.ImageShareGroup `json:"share_group"`
	}{
		Message:    fmt.Sprintf("Image share group '%s' (%d) updated successfully", updated.Label, updated.ID),
		ShareGroup: updated,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group response: %v", err)), nil
	}

	return result, nil
}

func imageShareGroupIDFromWriteTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["sharegroup_id"]
	if !exists {
		return 0, errImageShareGroupIDPositive
	}

	shareGroupID, ok := numberArgToInt(raw)
	if !ok || shareGroupID <= 0 {
		return 0, errImageShareGroupIDPositive
	}

	return shareGroupID, ""
}

func imageShareGroupUpdateFromTool(args map[string]any) (*linode.UpdateImageShareGroupRequest, string) {
	req := &linode.UpdateImageShareGroupRequest{}

	var hasUpdate bool

	label, hasLabel, validationMessage := optionalStringField(args, "label")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasLabel {
		req.Label = &label
		hasUpdate = true
	}

	description, hasDescription, validationMessage := optionalStringField(args, "description")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasDescription {
		req.Description = &description
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, "at least one of label or description is required"
	}

	return req, ""
}

func formatImageShareGroupUpdateError(err error) string {
	return "Failed to update image share group: " + err.Error()
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

// NewLinodeImageShareGroupTokenUpdateTool creates a tool for updating image share group membership token labels.
func NewLinodeImageShareGroupTokenUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_image_sharegroup_token_update",
		mcp.WithDescription("Updates an image share group membership token label."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("token_uuid", mcp.Required(), mcp.Description("The UUID of the token to update.")),
		mcp.WithString("label", mcp.Required(), mcp.Description("The new descriptive label for the token.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm image share group token update.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupTokenUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

func handleLinodeImageShareGroupTokenUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates an image share group token. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	label := strings.TrimSpace(request.GetString("label", ""))
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	environment := request.GetString(paramEnvironment, "")
	if environment != "" {
		request.GetArguments()[paramEnvironment] = environment
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updated, err := client.UpdateImageShareGroupToken(ctx, tokenUUID, &linode.UpdateImageShareGroupTokenRequest{Label: label})
	if err != nil {
		return mcp.NewToolResultError(formatImageShareGroupTokenUpdateError(err)), nil
	}

	response := struct {
		Message string                       `json:"message"`
		Token   *linode.ImageShareGroupToken `json:"token"`
	}{
		Message: fmt.Sprintf("Image share group token '%s' updated successfully", updated.TokenUUID),
		Token:   updated,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group token response: %v", err)), nil
	}

	return result, nil
}

func formatImageShareGroupTokenUpdateError(err error) string {
	return "Failed to update image share group token: " + err.Error()
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

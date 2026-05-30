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

// NewLinodeImageUploadTool creates a tool for uploading a custom image.
func NewLinodeImageUploadTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := []mcp.ToolOption{
		mcp.WithDescription("Creates an upload target for a custom image."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("label", mcp.Required(), mcp.Description("The custom image label.")),
		mcp.WithString("region", mcp.Required(), mcp.Description("The region for the image upload.")),
		mcp.WithString("description", mcp.Description("Detailed description for the image (optional).")),
		mcp.WithBoolean("cloud_init", mcp.Description("Whether the image supports cloud-init.")),
		mcp.WithString("tags", mcp.Description("JSON array of tag strings to apply to the image (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(),
			mcp.Description("Must be true to confirm image upload creation. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	}
	tool := mcp.NewTool("linode_image_upload", options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageUploadRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeImageReplicateTool creates a tool for replicating an image to regions.
func NewLinodeImageReplicateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_replicate",
		"Replicates an image to one or more compute regions.",
		[]mcp.ToolOption{
			mcp.WithString("image_id", mcp.Required(), mcp.Description("Image ID, such as private/123.")),
			mcp.WithString("regions", mcp.Required(), mcp.Description("JSON array of region slug strings to keep or replicate the image to.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm image replication. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageReplicateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeImageUpdateTool creates a tool for updating editable image metadata.
func NewLinodeImageUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_update",
		"Updates editable metadata for a Linode image.",
		[]mcp.ToolOption{
			mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
			mcp.WithString("image_id", mcp.Required(), mcp.Description("The editable image ID, for example private/12345 or shared/123.")),
			mcp.WithString("label", mcp.Description("New image label (optional).")),
			mcp.WithString("description", mcp.Description("New image description (optional).")),
			mcp.WithString("tags", mcp.Description("JSON array of tag strings to apply to the image (optional).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm image update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func validateImageUploadArgs(request *mcp.CallToolRequest) string {
	if strings.TrimSpace(request.GetString("label", "")) == "" {
		return errLabelRequired
	}

	if strings.TrimSpace(request.GetString("region", "")) == "" {
		return errRegionRequired
	}

	if _, err := optionalTagsFromTool(request); err != nil {
		return err.Error()
	}

	return ""
}

func handleLinodeImageUploadRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if msg := validateImageUploadArgs(request); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_upload", httpMethodPost, "/images/upload", nil)
	}

	if result := RequireConfirm(request, "This creates an image upload. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	label := strings.TrimSpace(request.GetString("label", ""))
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	region := strings.TrimSpace(request.GetString("region", ""))
	if region == "" {
		return mcp.NewToolResultError(errRegionRequired), nil
	}

	tags, err := optionalTagsFromTool(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UploadImageRequest{
		Label:       label,
		Region:      region,
		Description: request.GetString("description", ""),
		CloudInit:   request.GetBool("cloud_init", false),
		Tags:        tags,
	}

	upload, err := client.UploadImage(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to upload image: %v", err)), nil
	}

	response := struct {
		Message  string                      `json:"message"`
		UploadTo string                      `json:"upload_to"`
		Image    *linode.UploadImageResponse `json:"upload"`
	}{
		Message:  fmt.Sprintf("Image upload '%s' (%s) created successfully", upload.Image.Label, upload.Image.ID),
		UploadTo: upload.UploadTo,
		Image:    upload,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image upload response: %v", err)), nil
	}

	return result, nil
}

func optionalTagsFromTool(request *mcp.CallToolRequest) ([]string, error) {
	rawTags, tagsPresent := request.GetArguments()["tags"]
	if !tagsPresent {
		return nil, nil
	}

	tagsText, tagsOK := rawTags.(string)
	if !tagsOK {
		return nil, ErrTagsMustBeJSONStringArray
	}

	tagsText = strings.TrimSpace(tagsText)
	if tagsText == "" {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal([]byte(tagsText), &values); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTagsMustBeJSONStringArray, err)
	}

	for index, value := range values {
		values[index] = strings.TrimSpace(value)
		if values[index] == "" {
			return nil, ErrTagsEntriesNonEmpty
		}
	}

	return values, nil
}

func handleLinodeImageUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		imageID, validationMessage := editableImageIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if _, updateMessage := imageUpdateFromTool(request.GetArguments()); updateMessage != "" {
			return mcp.NewToolResultError(updateMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_update", "PUT",
			"/images/"+imageID,
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetImage(ctx, imageID) })
	}

	if result := RequireConfirm(request, "This updates image metadata. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	imageID, validationMessage := editableImageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := imageUpdateFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updated, err := client.UpdateImage(ctx, imageID, req)
	if err != nil {
		return mcp.NewToolResultError(formatImageUpdateError(err)), nil
	}

	response := struct {
		Message string        `json:"message"`
		Image   *linode.Image `json:"image"`
	}{
		Message: fmt.Sprintf("Image '%s' updated successfully", updated.ID),
		Image:   updated,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image response: %v", err)), nil
	}

	return result, nil
}

func editableImageIDFromTool(request *mcp.CallToolRequest) (string, string) {
	imageID, validationMessage := imageIDFromTool(request)
	if validationMessage != "" {
		return "", validationMessage
	}

	prefix, _, _ := strings.Cut(imageID, "/")
	if prefix == "linode" {
		return "", "image_id must reference an editable private or shared image"
	}

	return imageID, ""
}

func imageUpdateFromTool(args map[string]any) (*linode.UpdateImageRequest, string) {
	req := &linode.UpdateImageRequest{}

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

	tags, hasTags, validationMessage := optionalTagsField(args, "tags")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasTags {
		req.Tags = &tags
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, "at least one of label, description, or tags is required"
	}

	return req, ""
}

func optionalTagsField(args map[string]any, name string) ([]string, bool, string) {
	rawTags, tagsPresent := args[name]
	if !tagsPresent {
		return nil, false, ""
	}

	values, validationMessage := tagsValueFromToolArg(rawTags)
	if validationMessage != "" {
		return nil, false, validationMessage
	}

	return values, true, ""
}

func tagsValueFromToolArg(rawTags any) ([]string, string) {
	switch tags := rawTags.(type) {
	case string:
		tagsText := strings.TrimSpace(tags)

		var values []string
		if err := json.Unmarshal([]byte(tagsText), &values); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTagsMustBeJSONStringArray, err).Error()
		}

		if values == nil {
			return nil, ErrTagsMustBeJSONStringArray.Error()
		}

		return normalizeTags(values)
	case []string:
		return normalizeTags(tags)
	case []any:
		values := make([]string, 0, len(tags))
		for _, tag := range tags {
			tagText, ok := tag.(string)
			if !ok {
				return nil, ErrTagsMustBeJSONStringArray.Error()
			}

			values = append(values, tagText)
		}

		return normalizeTags(values)
	default:
		return nil, ErrTagsMustBeJSONStringArray.Error()
	}
}

func normalizeTags(values []string) ([]string, string) {
	normalized := make([]string, len(values))
	for index, value := range values {
		normalized[index] = strings.TrimSpace(value)
		if normalized[index] == "" {
			return nil, ErrTagsEntriesNonEmpty.Error()
		}
	}

	return normalized, ""
}

func formatImageUpdateError(err error) string {
	return "Failed to update image: " + err.Error()
}

// NewLinodeImageShareGroupCreateTool creates a tool for creating image share groups.
func NewLinodeImageShareGroupCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_create",
		"Creates a share group for sharing images with other users.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(), mcp.Description("The share group's descriptive name.")),
			mcp.WithString("description", mcp.Description("Detailed description for the share group (optional).")),
			mcp.WithString("images", mcp.Description("JSON array of images to include, each with required id and optional label/description.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm image share group creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageShareGroupCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// shareGroupCreateArgs parses+validates the share group create args, returning
// the parsed images plus an error message. Shared by the real create path and
// the dry-run preview.
func shareGroupCreateArgs(request *mcp.CallToolRequest) ([]linode.ImageShareGroupImage, string) {
	if strings.TrimSpace(request.GetString("label", "")) == "" {
		return nil, errLabelRequired
	}

	var imagesJSON string

	if imagesArg, ok := request.GetArguments()["images"]; ok {
		imagesText, imagesOK := imagesArg.(string)
		if !imagesOK {
			return nil, "images must be a JSON string"
		}

		imagesJSON = imagesText
	}

	images, err := imageShareGroupImagesFromTool(imagesJSON)
	if err != nil {
		return nil, err.Error()
	}

	return images, ""
}

func handleLinodeImageShareGroupCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if _, msg := shareGroupCreateArgs(request); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_create", httpMethodPost, "/images/sharegroups", nil)
	}

	confirm, confirmOK := request.GetArguments()[paramConfirm].(bool)
	if !confirmOK || !confirm {
		return mcp.NewToolResultError("This creates an image share group. Set confirm=true to proceed."), nil
	}

	images, msg := shareGroupCreateArgs(request)
	if msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	label := strings.TrimSpace(request.GetString("label", ""))

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
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_images_add",
		"Adds one or more private images to an image share group.",
		[]mcp.ToolOption{
			mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID to add images to.")),
			mcp.WithString("images", mcp.Required(), mcp.Description("JSON array of images to add, each with required id and optional label/description.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm adding images to the share group. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageShareGroupImagesAddRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupImagesAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if _, imagesMessage := requiredImageShareGroupImagesFromTool(request); imagesMessage != "" {
			return mcp.NewToolResultError(imagesMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_images_add", httpMethodPost,
			fmt.Sprintf("/images/sharegroups/%d/images", shareGroupID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetImageShareGroup(ctx, shareGroupID)
			})
	}

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
			mcp.Description("Must be true to confirm shared image update. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupImageUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupImageUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		imageID, imageMessage := imageShareGroupSharedImageIDFromTool(request)
		if imageMessage != "" {
			return mcp.NewToolResultError(imageMessage), nil
		}

		if _, updateMessage := imageShareGroupImageUpdateFromTool(request.GetArguments()); updateMessage != "" {
			return mcp.NewToolResultError(updateMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_image_update", "PUT",
			fmt.Sprintf("/images/sharegroups/%d/images/%s", shareGroupID, imageID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetImageShareGroup(ctx, shareGroupID)
			})
	}

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
			mcp.Description("Must be true to confirm adding members to the share group. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupMembersAddRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupMembersAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if _, memberMessage := imageShareGroupMemberAddFromTool(request.GetArguments()); memberMessage != "" {
			return mcp.NewToolResultError(memberMessage), nil
		}

		// Fetch the parent share group, never the membership token material.
		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_members_add", httpMethodPost,
			fmt.Sprintf("/images/sharegroups/%d/members", shareGroupID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetImageShareGroup(ctx, shareGroupID)
			})
	}

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
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_update",
		"Updates an image share group's label or description.",
		[]mcp.ToolOption{
			mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric image share group ID to update.")),
			mcp.WithString("label", mcp.Description("New descriptive name for the share group (optional).")),
			mcp.WithString("description", mcp.Description("New detailed description for the share group (optional).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm image share group update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageShareGroupUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		shareGroupID, validationMessage := imageShareGroupIDFromWriteTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if _, updateMessage := imageShareGroupUpdateFromTool(request.GetArguments()); updateMessage != "" {
			return mcp.NewToolResultError(updateMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_update", "PUT",
			fmt.Sprintf("/images/sharegroups/%d", shareGroupID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetImageShareGroup(ctx, shareGroupID)
			})
	}

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
			mcp.Description("Must be true to confirm private image creation. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
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
			mcp.Description("Must be true to confirm image share group token creation. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeImageShareGroupTokenCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeImageShareGroupTokenUpdateTool creates a tool for updating image share group membership token labels.
func NewLinodeImageShareGroupTokenUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_token_update",
		"Updates an image share group membership token label.",
		[]mcp.ToolOption{
			mcp.WithString("token_uuid", mcp.Required(), mcp.Description("The UUID of the token to update.")),
			mcp.WithString("label", mcp.Required(), mcp.Description("The new descriptive label for the token.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm image share group token update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageShareGroupTokenUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeImageShareGroupMemberUpdateTool creates a tool for updating image share group member token labels.
func NewLinodeImageShareGroupMemberUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_image_sharegroup_member_update",
		"Updates a member token label for an owned image share group.",
		[]mcp.ToolOption{
			mcp.WithNumber("sharegroup_id", mcp.Required(), mcp.Description("The numeric ID of the image share group.")),
			mcp.WithString("token_uuid", mcp.Required(), mcp.Description("The UUID of the member token to update.")),
			mcp.WithString("label", mcp.Required(), mcp.Description("The new descriptive label for the member token.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm image share group member token update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeImageShareGroupMemberUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeImageShareGroupTokenUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		tokenUUID, validationMessage := imageShareGroupTokenUUIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if strings.TrimSpace(request.GetString("label", "")) == "" {
			return mcp.NewToolResultError(errLabelRequired), nil
		}

		// Fetch the parent share group by token, never the token secret itself.
		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_token_update", "PUT",
			"/images/sharegroups/tokens/"+tokenUUID,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetImageShareGroupByToken(ctx, tokenUUID)
			})
	}

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

func handleLinodeImageShareGroupMemberUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		tokenUUID, tokenMessage := imageShareGroupTokenUUIDFromTool(request)
		if tokenMessage != "" {
			return mcp.NewToolResultError(tokenMessage), nil
		}

		if strings.TrimSpace(request.GetString("label", "")) == "" {
			return mcp.NewToolResultError(errLabelRequired), nil
		}

		// Fetch the parent share group, never the member token secret.
		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_member_update", "PUT",
			fmt.Sprintf("/images/sharegroups/%d/members/%s", shareGroupID, tokenUUID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetImageShareGroup(ctx, shareGroupID)
			})
	}

	if result := RequireConfirm(request, "This updates an image share group member token. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	shareGroupID, validationMessage := imageShareGroupIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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

	member, err := client.UpdateImageShareGroupMember(ctx, shareGroupID, tokenUUID, &linode.UpdateImageShareGroupMemberRequest{Label: label})
	if err != nil {
		return mcp.NewToolResultError(formatImageShareGroupMemberUpdateError(err)), nil
	}

	response := struct {
		Message string                        `json:"message"`
		Member  *linode.ImageShareGroupMember `json:"member"`
	}{
		Message: fmt.Sprintf("Image share group member token '%s' updated successfully", member.TokenUUID),
		Member:  member,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image share group member token response: %v", err)), nil
	}

	return result, nil
}

func formatImageShareGroupMemberUpdateError(err error) string {
	return "Failed to update image share group member token: " + err.Error()
}

func handleLinodeImageShareGroupTokenCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if strings.TrimSpace(request.GetString("valid_for_sharegroup_uuid", "")) == "" {
			return mcp.NewToolResultError("valid_for_sharegroup_uuid is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_sharegroup_token_create", httpMethodPost, "/images/sharegroups/tokens", nil)
	}

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

func handleLinodeImageReplicateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		imageID, validationMessage := privateImageIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if _, regionsMessage := requiredImageReplicationRegionsFromTool(request); regionsMessage != "" {
			return mcp.NewToolResultError(regionsMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_replicate", httpMethodPost,
			"/images/"+imageID+"/regions",
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetImage(ctx, imageID) })
	}

	if result := RequireConfirm(request, "This replicates an image to the requested regions. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	imageID, validationMessage := privateImageIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	regions, validationMessage := requiredImageReplicationRegionsFromTool(request)
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

	image, err := client.ReplicateImage(ctx, imageID, &linode.ReplicateImageRequest{Regions: regions})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to replicate image: %v", err)), nil
	}

	response := struct {
		Message string        `json:"message"`
		Image   *linode.Image `json:"image"`
	}{
		Message: fmt.Sprintf("Image '%s' replicated successfully", image.ID),
		Image:   image,
	}

	result, err := MarshalToolResponse(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format image replication response: %v", err)), nil
	}

	return result, nil
}

func requiredImageReplicationRegionsFromTool(request *mcp.CallToolRequest) ([]string, string) {
	regionsArg, regionsPresent := request.GetArguments()["regions"]
	if !regionsPresent {
		return nil, "regions is required"
	}

	regionsJSON, regionsIsString := regionsArg.(string)
	if !regionsIsString {
		return nil, "regions must be a JSON string array"
	}

	var regions []string
	if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
		return nil, "regions must be a JSON string array"
	}

	if len(regions) == 0 {
		return nil, "regions must contain at least one region"
	}

	for _, region := range regions {
		trimmed := strings.TrimSpace(region)
		if trimmed == "" {
			return nil, "regions entries must be non-empty strings"
		}

		if trimmed != region {
			return nil, "regions entries must be lowercase region slugs"
		}

		if err := validateRegionSlug(region); err != nil {
			return nil, "regions entries must be lowercase region slugs"
		}
	}

	return regions, ""
}

func handleLinodeImageCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if request.GetInt("disk_id", 0) <= 0 {
			return mcp.NewToolResultError(ErrDiskIDRequired.Error()), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_image_create", httpMethodPost, "/images", nil)
	}

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

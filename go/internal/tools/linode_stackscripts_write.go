package tools

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeStackScriptCreateTool creates a tool for creating a StackScript.
func NewLinodeStackScriptCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_stackscript_create",
		mcp.WithDescription("Creates a new StackScript for automated deployments. The script can be used when creating Linode instances."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"label",
			mcp.Required(),
			mcp.Description("A label for the StackScript (max 64 characters, must be unique)"),
		),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The script content (bash, compatible with the selected images)"),
		),
		mcp.WithString(
			"images",
			mcp.Required(),
			mcp.Description("Comma-separated list of Image IDs that the StackScript supports (e.g., \"linode/debian12,linode/ubuntu24.04\")"),
		),
		mcp.WithString(
			"description",
			mcp.Description("A description of the StackScript"),
		),
		mcp.WithBoolean(
			"is_public",
			mcp.Description("Whether the StackScript should be public (default: false)"),
		),
		mcp.WithString(
			"rev_note",
			mcp.Description("A revision note for this version of the StackScript"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm StackScript creation."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeStackScriptCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a new StackScript in your account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	label := strings.TrimSpace(request.GetString("label", ""))
	script := request.GetString("script", "")
	imagesRaw := request.GetString("images", "")
	description := request.GetString("description", "")
	isPublic := request.GetBool("is_public", false)
	revNote := request.GetString("rev_note", "")

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if strings.TrimSpace(script) == "" {
		return mcp.NewToolResultError("script is required"), nil
	}

	if imagesRaw == "" {
		return mcp.NewToolResultError("images is required and must contain at least one image ID"), nil
	}

	var images []string

	for img := range strings.SplitSeq(imagesRaw, ",") {
		if trimmed := strings.TrimSpace(img); trimmed != "" {
			images = append(images, trimmed)
		}
	}

	if len(images) == 0 {
		return mcp.NewToolResultError("images is required and must contain at least one image ID"), nil
	}

	if slices.ContainsFunc(images, invalidStackScriptImageID) {
		return mcp.NewToolResultError("images entries must be valid image IDs"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateStackScriptRequest{
		Label:       label,
		Script:      script,
		Images:      images,
		Description: description,
		IsPublic:    isPublic,
		RevNote:     revNote,
	}

	created, err := client.CreateStackScript(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create StackScript: %v", err)), nil
	}

	response := struct {
		Message     string              `json:"message"`
		StackScript *linode.StackScript `json:"stackscript"`
	}{
		Message:     fmt.Sprintf("StackScript '%s' (ID: %d) created successfully", created.Label, created.ID),
		StackScript: created,
	}

	return MarshalToolResponse(response)
}

func formatStackScriptDeleteError(stackScriptID int, err error) string {
	return "Failed to remove StackScript " + strconv.Itoa(stackScriptID) + ": " + err.Error()
}

// NewLinodeStackScriptDeleteTool creates a tool for deleting a StackScript.
func NewLinodeStackScriptDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_stackscript_delete",
		mcp.WithDescription("Deletes a StackScript. WARNING: This permanently removes the StackScript from your account."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"stackscript_id",
			mcp.Required(),
			mcp.Description("The ID of the StackScript to delete"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLinodeStackScriptDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This operation is destructive. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	stackScriptID, validationMessage := optionalPaginationInt(request.GetArguments(), "stackscript_id", 1, 0)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if stackScriptID == 0 {
		return mcp.NewToolResultError("stackscript_id must be a positive integer"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteStackScript(ctx, stackScriptID); err != nil {
		return mcp.NewToolResultError(formatStackScriptDeleteError(stackScriptID, err)), nil
	}

	response := struct {
		Message       string `json:"message"`
		StackScriptID int    `json:"stackscript_id"`
	}{
		Message:       fmt.Sprintf("StackScript %d deleted successfully", stackScriptID),
		StackScriptID: stackScriptID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeStackScriptUpdateTool creates a tool for updating a StackScript.
func NewLinodeStackScriptUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_stackscript_update",
		mcp.WithDescription("Updates editable fields on an existing StackScript."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"stackscript_id",
			mcp.Required(),
			mcp.Description("The StackScript ID to update."),
		),
		mcp.WithString(
			"label",
			mcp.Description("A new label for the StackScript."),
		),
		mcp.WithString(
			"script",
			mcp.Description("Updated script content."),
		),
		mcp.WithString(
			"images",
			mcp.Description("Comma-separated list of Image IDs that the StackScript supports."),
		),
		mcp.WithString(
			"description",
			mcp.Description("Updated StackScript description."),
		),
		mcp.WithBoolean(
			"is_public",
			mcp.Description("Whether the StackScript should be public."),
		),
		mcp.WithString(
			"rev_note",
			mcp.Description("A revision note for this update."),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm StackScript update."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeStackScriptUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates a StackScript in your account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	stackScriptID, ok := numberArgToInt(request.GetArguments()["stackscript_id"])
	if !ok || stackScriptID <= 0 {
		return mcp.NewToolResultError("stackscript_id must be a positive integer"), nil
	}

	req, validationMessage := stackScriptUpdateFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updated, err := client.UpdateStackScript(ctx, stackScriptID, req)
	if err != nil {
		return mcp.NewToolResultError(formatStackScriptUpdateError(err)), nil
	}

	response := struct {
		Message     string              `json:"message"`
		StackScript *linode.StackScript `json:"stackscript"`
	}{
		Message:     fmt.Sprintf("StackScript '%s' (ID: %d) updated successfully", updated.Label, updated.ID),
		StackScript: updated,
	}

	return MarshalToolResponse(response)
}

func stackScriptUpdateFromTool(args map[string]any) (*linode.UpdateStackScriptRequest, string) {
	req := &linode.UpdateStackScriptRequest{}

	var hasUpdate bool

	label, hasLabel, validationMessage := optionalStringField(args, "label")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasLabel {
		req.Label = &label
		hasUpdate = true
	}

	script, hasScript, validationMessage := optionalStringField(args, "script")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasScript {
		req.Script = &script
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

	revNote, hasRevNote, validationMessage := optionalStringField(args, "rev_note")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasRevNote {
		req.RevNote = &revNote
		hasUpdate = true
	}

	imagesRaw, hasImages, validationMessage := optionalStringField(args, "images")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasImages {
		images := splitStackScriptImages(imagesRaw)
		if len(images) == 0 {
			return nil, "images must contain at least one image ID"
		}

		if slices.ContainsFunc(images, invalidStackScriptImageID) {
			return nil, "images entries must be valid image IDs"
		}

		req.Images = images
		hasUpdate = true
	}

	if rawIsPublic, exists := args["is_public"]; exists {
		isPublic, ok := rawIsPublic.(bool)
		if !ok {
			return nil, "is_public must be a boolean"
		}

		req.IsPublic = &isPublic
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, "at least one editable field is required"
	}

	return req, ""
}

func splitStackScriptImages(imagesRaw string) []string {
	var images []string

	for img := range strings.SplitSeq(imagesRaw, ",") {
		if trimmed := strings.TrimSpace(img); trimmed != "" {
			images = append(images, trimmed)
		}
	}

	return images
}

func invalidStackScriptImageID(imageID string) bool {
	return strings.Count(imageID, "/") != 1 || strings.Contains(imageID, "?") || strings.Contains(imageID, "#") || strings.Contains(imageID, "..")
}

func formatStackScriptUpdateError(err error) string {
	return "Failed to update StackScript: " + err.Error()
}

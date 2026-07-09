package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeStackScriptCreateTool creates a tool for creating a StackScript.
func NewLinodeStackScriptCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_stackscript_create",
		"Creates a new StackScript for automated deployments. The script can be used when creating Linode instances.",
		toolschemas.Schema("linode.mcp.v1.StackScriptCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeStackScriptCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if _, validationMessage := stackScriptCreateRequestFromTool(request); validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_stackscript_create", httpMethodPost, "/linode/stackscripts", nil)
	}

	if result := RequireConfirm(request, "This creates a new StackScript in your account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := stackScriptCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	created, err := client.CreateStackScriptProto(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create StackScript: %v", err)), nil
	}

	response := &linodev1.StackScriptWriteResponse{
		Message:     fmt.Sprintf("StackScript '%s' (ID: %d) created successfully", created.GetLabel(), created.GetId()),
		Stackscript: created,
	}

	return MarshalProtoToolResponse(response)
}

// stackScriptCreateRequestFromTool parses and validates the create args.
// Shared by the real create path and the dry-run preview so both reject
// the same malformed inputs.
func stackScriptCreateRequestFromTool(request *mcp.CallToolRequest) (linode.CreateStackScriptRequest, string) {
	label := strings.TrimSpace(request.GetString("label", ""))
	script := request.GetString("script", "")

	if label == "" {
		return linode.CreateStackScriptRequest{}, errLabelRequired
	}

	if strings.TrimSpace(script) == "" {
		return linode.CreateStackScriptRequest{}, "script is required"
	}

	images, validationMessage := stackScriptImagesFromTool(request.GetArguments())
	if validationMessage != "" {
		return linode.CreateStackScriptRequest{}, validationMessage
	}

	if len(images) == 0 {
		return linode.CreateStackScriptRequest{}, "images is required and must contain at least one image ID"
	}

	if slices.ContainsFunc(images, invalidStackScriptImageID) {
		return linode.CreateStackScriptRequest{}, "images entries must be valid image IDs"
	}

	return linode.CreateStackScriptRequest{
		Label:       label,
		Script:      script,
		Images:      images,
		Description: request.GetString("description", ""),
		IsPublic:    request.GetBool("is_public", false),
		RevNote:     request.GetString("rev_note", ""),
	}, ""
}

// NewLinodeStackScriptDeleteTool creates a tool for deleting a StackScript.
func NewLinodeStackScriptDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_stackscript_delete",
		"Deletes a StackScript. WARNING: This permanently removes the StackScript from your account."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.StackScriptDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// stackscriptDeleteProto builds the proto-canonical id-echo body for a
// successful StackScript delete, keeping the proto literal off the handler's
// struct literal so the delete handlers stay below the dupl threshold.
func stackscriptDeleteProto(id int) proto.Message {
	return &linodev1.StackScriptDeleteResponse{
		Message:       fmt.Sprintf("StackScript %d deleted successfully", id),
		StackscriptId: linodeIDToInt32(id),
	}
}

func handleLinodeStackScriptDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if _, validationMessage := requiredIDArgument(request, "stackscript_id"); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_stackscript_delete",
		IDParam:        "stackscript_id",
		Method:         httpMethodDelete,
		PathPattern:    "/linode/stackscripts/%d",
		ConfirmMessage: destroyConfirmMessage,
		SuccessProto:   stackscriptDeleteProto,
		FetchState:     func(ctx context.Context, c *linode.Client, id int) (any, error) { return c.GetStackScript(ctx, id) },
		Execute:        func(ctx context.Context, c *linode.Client, id int) error { return c.DeleteStackScript(ctx, id) },
		HashIgnore:     twostage.HashIgnoreFields("StackScript"),
	})
}

// NewLinodeStackScriptUpdateTool creates a tool for updating a StackScript.
func NewLinodeStackScriptUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_stackscript_update",
		"Updates editable fields on an existing StackScript.",
		toolschemas.Schema("linode.mcp.v1.StackScriptUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeStackScriptUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return handleLinodeStackScriptUpdateDryRun(ctx, request, cfg)
	}

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

	updated, err := client.UpdateStackScriptProto(ctx, stackScriptID, req)
	if err != nil {
		return mcp.NewToolResultError(formatStackScriptUpdateError(err)), nil
	}

	response := &linodev1.StackScriptWriteResponse{
		Message:     fmt.Sprintf("StackScript '%s' (ID: %d) updated successfully", updated.GetLabel(), updated.GetId()),
		Stackscript: updated,
	}

	return MarshalProtoToolResponse(response)
}

// handleLinodeStackScriptUpdateDryRun validates the update args, fetches the
// current StackScript, and returns the preview without issuing the PUT.
func handleLinodeStackScriptUpdateDryRun(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	stackScriptID, ok := numberArgToInt(request.GetArguments()["stackscript_id"])
	if !ok || stackScriptID <= 0 {
		return mcp.NewToolResultError("stackscript_id must be a positive integer"), nil
	}

	if _, validationMessage := stackScriptUpdateFromTool(request.GetArguments()); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_stackscript_update", "PUT",
		fmt.Sprintf("/linode/stackscripts/%d", stackScriptID),
		func(ctx context.Context, c *linode.Client) (any, error) { return c.GetStackScript(ctx, stackScriptID) },
		func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
			return stackScriptUpdateSideEffects(ctx, state,
				request.GetString("label", ""),
				request.GetString("script", ""),
				request.GetString("description", ""))
		})
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

	if _, hasImages := args["images"]; hasImages {
		images, validationMessage := stackScriptImagesFromTool(args)
		if validationMessage != "" {
			return nil, validationMessage
		}

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

// stackScriptImagesFromTool reads the native "images" array argument, trims each
// entry, and drops blanks. An absent argument returns an empty slice; malformed
// input returns a validation message.
func stackScriptImagesFromTool(args map[string]any) ([]string, string) {
	raw, present := args["images"]
	if !present {
		return nil, ""
	}

	values, validationMessage := stringSliceFromToolArg(raw, "images")
	if validationMessage != "" {
		return nil, validationMessage
	}

	images := make([]string, 0, len(values))

	for _, img := range values {
		if trimmed := strings.TrimSpace(img); trimmed != "" {
			images = append(images, trimmed)
		}
	}

	return images, ""
}

func invalidStackScriptImageID(imageID string) bool {
	return strings.Count(imageID, "/") != 1 || strings.Contains(imageID, "?") || strings.Contains(imageID, "#") || strings.Contains(imageID, "..")
}

func formatStackScriptUpdateError(err error) string {
	return "Failed to update StackScript: " + err.Error()
}

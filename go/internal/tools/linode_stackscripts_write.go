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

	label := request.GetString("label", "")
	script := request.GetString("script", "")
	imagesRaw := request.GetString("images", "")
	description := request.GetString("description", "")
	isPublic := request.GetBool("is_public", false)
	revNote := request.GetString("rev_note", "")

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if script == "" {
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

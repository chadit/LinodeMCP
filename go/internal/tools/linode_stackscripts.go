package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeStackScriptGetTool creates a tool for retrieving one StackScript.
func NewLinodeStackScriptGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_stackscript_get",
		"Gets one StackScript by ID.",
		toolschemas.Schema("linode.mcp.v1.StackScriptGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeStackScriptListTool creates a tool for listing StackScripts.
func NewLinodeStackScriptListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_stackscript_list",
		mcp.WithDescription("Lists StackScripts. By default returns your own StackScripts. Can filter by public status, ownership, or label."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"is_public",
			mcp.Description("Filter by public status (true, false)"),
		),
		mcp.WithString(
			"mine",
			mcp.Description("Filter by ownership - only your own StackScripts (true, false)"),
		),
		mcp.WithString(
			"label_contains",
			mcp.Description("Filter StackScripts by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeStackScriptGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	stackScriptID, validationMessage := stackScriptIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	script, err := client.GetStackScriptProto(ctx, stackScriptID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve StackScript: %v", err)), nil
	}

	return MarshalProtoToolResponse(script)
}

func stackScriptIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["stackscript_id"]
	if !exists {
		return 0, "stackscript_id must be a positive integer"
	}

	stackScriptID, ok := numberArgToInt(raw)
	if !ok || stackScriptID <= 0 {
		return 0, "stackscript_id must be a positive integer"
	}

	return stackScriptID, ""
}

func handleLinodeStackScriptsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	isPublicFilter := request.GetString("is_public", "")
	mineFilter := request.GetString("mine", "")
	labelContains := request.GetString("label_contains", "")

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	scripts, err := client.ListStackScripts(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve StackScripts: %v", err)), nil
	}

	if isPublicFilter != "" {
		scripts = FilterByField(scripts, isPublicFilter, func(s linode.StackScript) string {
			if s.IsPublic {
				return boolTrue
			}

			return boolFalse
		})
	}

	if mineFilter != "" {
		scripts = FilterByField(scripts, mineFilter, func(s linode.StackScript) string {
			if s.Mine {
				return boolTrue
			}

			return boolFalse
		})
	}

	if labelContains != "" {
		scripts = FilterByContains(scripts, labelContains, func(s linode.StackScript) string {
			return s.Label
		})
	}

	return formatStackScriptsResponse(scripts, isPublicFilter, mineFilter, labelContains)
}

func formatStackScriptsResponse(scripts []linode.StackScript, isPublicFilter, mineFilter, labelContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count        int                  `json:"count"`
		Filter       string               `json:"filter,omitempty"`
		StackScripts []linode.StackScript `json:"stackscripts"`
	}{
		Count:        len(scripts),
		StackScripts: scripts,
	}

	var filters []string
	if isPublicFilter != "" {
		filters = append(filters, "is_public="+isPublicFilter)
	}

	if mineFilter != "" {
		filters = append(filters, "mine="+mineFilter)
	}

	if labelContains != "" {
		filters = append(filters, "label_contains="+labelContains)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return MarshalToolResponse(response)
}

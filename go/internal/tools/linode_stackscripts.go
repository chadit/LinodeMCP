package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeStackScriptsListTool creates a tool for listing StackScripts.
func NewLinodeStackScriptsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_stackscripts_list",
		mcp.WithDescription("Lists StackScripts. By default returns your own StackScripts. Can filter by public status, ownership, or label."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("is_public",
			mcp.Description("Filter by public status (true, false)"),
		),
		mcp.WithString("mine",
			mcp.Description("Filter by ownership - only your own StackScripts (true, false)"),
		),
		mcp.WithString("label_contains",
			mcp.Description("Filter StackScripts by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptsListRequest(ctx, &request, cfg)
	}

	return tool, handler
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
		scripts = filterByField(scripts, isPublicFilter, func(s linode.StackScript) string {
			if s.IsPublic {
				return boolTrue
			}

			return "false"
		})
	}

	if mineFilter != "" {
		scripts = filterByField(scripts, mineFilter, func(s linode.StackScript) string {
			if s.Mine {
				return boolTrue
			}

			return "false"
		})
	}

	if labelContains != "" {
		scripts = filterByContains(scripts, labelContains, func(s linode.StackScript) string {
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

	return marshalToolResponse(response)
}

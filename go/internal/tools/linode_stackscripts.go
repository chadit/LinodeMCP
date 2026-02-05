//nolint:dupl // Tool implementations have similar structure by design
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

// NewLinodeStackScriptsListTool creates a tool for listing StackScripts.
func NewLinodeStackScriptsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_stackscripts_list",
		mcp.WithDescription("Lists StackScripts. By default returns your own StackScripts. Can filter by public status, ownership, or label."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
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
		return handleLinodeStackScriptsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeStackScriptsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	isPublicFilter := request.GetString("is_public", "")
	mineFilter := request.GetString("mine", "")
	labelContains := request.GetString("label_contains", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	scripts, err := client.ListStackScripts(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve StackScripts: %v", err)), nil
	}

	if isPublicFilter != "" {
		scripts = filterStackScriptsByPublic(scripts, isPublicFilter)
	}

	if mineFilter != "" {
		scripts = filterStackScriptsByMine(scripts, mineFilter)
	}

	if labelContains != "" {
		scripts = filterStackScriptsByLabel(scripts, labelContains)
	}

	return formatStackScriptsResponse(scripts, isPublicFilter, mineFilter, labelContains)
}

func filterStackScriptsByPublic(scripts []linode.StackScript, isPublicFilter string) []linode.StackScript {
	var filtered []linode.StackScript

	wantPublic := strings.ToLower(isPublicFilter) == boolTrue

	for _, script := range scripts {
		if script.IsPublic == wantPublic {
			filtered = append(filtered, script)
		}
	}

	return filtered
}

func filterStackScriptsByMine(scripts []linode.StackScript, mineFilter string) []linode.StackScript {
	var filtered []linode.StackScript

	wantMine := strings.ToLower(mineFilter) == boolTrue

	for _, script := range scripts {
		if script.Mine == wantMine {
			filtered = append(filtered, script)
		}
	}

	return filtered
}

func filterStackScriptsByLabel(scripts []linode.StackScript, labelContains string) []linode.StackScript {
	var filtered []linode.StackScript

	labelContains = strings.ToLower(labelContains)

	for _, script := range scripts {
		if strings.Contains(strings.ToLower(script.Label), labelContains) {
			filtered = append(filtered, script)
		}
	}

	return filtered
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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

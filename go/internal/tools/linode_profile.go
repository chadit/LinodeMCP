package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeProfileTool creates a tool for retrieving Linode profile info.
func NewLinodeProfileTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg, "linode_profile",
		"Retrieves Linode user account profile information",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetProfile(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeProfileLoginsTool creates a tool for listing login history for the authenticated profile.
func NewLinodeProfileLoginsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_logins",
		"Lists login history for the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeProfileLoginsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeProfileLoginsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := profileLoginsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	logins, listFailure := client.ListProfileLogins(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(logins)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_logins: " + listFailure.Error()), nil
}

func profileLoginsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountLoginsPageSizeMin, accountLoginsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

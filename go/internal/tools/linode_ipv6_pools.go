package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	ipv6PoolsPageSizeMin = 25
	ipv6PoolsPageSizeMax = 500
)

// NewLinodeIPv6PoolsListTool creates a tool for listing IPv6 pools.
func NewLinodeIPv6PoolsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_ipv6_pools_list",
		"Lists IPv6 pools on the account with optional pagination.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeIPv6PoolsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeIPv6PoolsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := ipv6PoolsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pools, listFailureMessage := listIPv6Pools(ctx, client, page, pageSize)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_ipv6_pools_list: " + listFailureMessage), nil
	}

	return MarshalToolResponse(pools)
}

func listIPv6Pools(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.IPv6Pool], string) {
	pools, err := client.ListIPv6Pools(ctx, page, pageSize)
	if err != nil {
		return nil, err.Error()
	}

	return pools, ""
}

func ipv6PoolsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", ipv6PoolsPageSizeMin, ipv6PoolsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

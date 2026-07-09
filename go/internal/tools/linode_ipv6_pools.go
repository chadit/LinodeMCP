package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeIPv6PoolsListTool creates a tool for listing IPv6 pools.
func NewLinodeIPv6PoolsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_ipv6_pool_list",
		"Lists IPv6 pools on the account with optional pagination.",
		"linode.mcp.v1.IPv6PoolListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.IPv6Pool, error) {
			return client.ListIPv6PoolsProto(ctx, page, pageSize)
		},
		ipv6ListPaginationFromTool,
		nil,
		ipv6PoolListResponse,
	)

	return tool, profiles.CapRead, handler
}

func ipv6PoolListResponse(items []*linodev1.IPv6Pool, count int32, filter *string) *linodev1.IPv6PoolListResponse {
	return &linodev1.IPv6PoolListResponse{Count: count, Filter: filter, Ipv6Pools: items}
}

package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeIPv6PoolsListTool creates a tool for listing IPv6 pools.
func NewLinodeIPv6PoolsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newLinodeIPv6ListTool(
		cfg,
		"linode_ipv6_pool_list",
		"Lists IPv6 pools on the account with optional pagination.",
		func(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.IPv6Pool], string) {
			items, err := client.ListIPv6Pools(ctx, page, pageSize)
			if err != nil {
				return nil, err.Error()
			}

			return items, ""
		},
	)
}

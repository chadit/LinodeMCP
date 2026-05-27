package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeIPv6RangesListTool creates a tool for listing IPv6 ranges.
func NewLinodeIPv6RangesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newLinodeIPv6ListTool(
		cfg,
		"linode_ipv6_ranges_list",
		"Lists IPv6 ranges on the account with optional pagination.",
		func(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.IPv6Range], string) {
			items, err := client.ListIPv6Ranges(ctx, page, pageSize)
			if err != nil {
				return nil, err.Error()
			}

			return items, ""
		},
	)
}

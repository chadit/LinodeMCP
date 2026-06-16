package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeNetworkTransferPricesTool creates a tool for listing network transfer prices.
func NewLinodeNetworkTransferPricesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg,
		"linode_network_transfer_price_list",
		"Lists Linode network transfer prices, including default and region-specific rates.",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.ListNetworkTransferPrices(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

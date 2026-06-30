package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeNetworkTransferPricesTool creates a tool for listing network transfer prices.
func NewLinodeNetworkTransferPricesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_network_transfer_price_list",
		"Lists Linode network transfer prices, including default and region-specific rates.",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LinodeType, error) {
			return client.ListNetworkTransferPricesProto(ctx)
		},
		nil,
		networkTransferPriceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func networkTransferPriceListResponse(items []*linodev1.LinodeType, count int32, filter *string) *linodev1.NetworkTransferPriceListResponse {
	return &linodev1.NetworkTransferPriceListResponse{Count: count, Filter: filter, NetworkTransferPrices: items}
}

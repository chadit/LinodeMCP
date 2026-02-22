package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeRegionsListTool creates a tool for listing Linode regions.
func NewLinodeRegionsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_regions_list",
		mcp.WithDescription("Lists all available Linode regions (datacenters) with optional filtering by country or capabilities"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("country", mcp.Description("Filter regions by country code (e.g., 'us', 'de', 'jp')")),
		mcp.WithString("capability", mcp.Description("Filter regions by capability (e.g., 'Linodes', 'Block Storage', 'GPU Linodes')")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListRequest(ctx, &request, cfg,
			func(ctx context.Context, client *linode.RetryableClient) ([]linode.Region, error) {
				return client.ListRegions(ctx)
			},
			[]filterDef[linode.Region]{
				{"country", func(items []linode.Region, v string) []linode.Region {
					return filterByField(items, v, func(r linode.Region) string { return r.Country })
				}},
				{"capability", filterRegionsByCapability},
			},
			func(items []linode.Region, appliedFilters []string) (*mcp.CallToolResult, error) {
				return formatListResponse(items, appliedFilters, "regions")
			},
		)
	}

	return tool, handler
}

func filterRegionsByCapability(regions []linode.Region, capabilityFilter string) []linode.Region {
	filtered := make([]linode.Region, 0, len(regions))

	for i := range regions {
		for _, cap := range regions[i].Capabilities {
			if strings.EqualFold(cap, capabilityFilter) {
				filtered = append(filtered, regions[i])

				break
			}
		}
	}

	return filtered
}

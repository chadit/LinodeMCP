//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeRegionsListTool creates a tool for listing Linode regions.
func NewLinodeRegionsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_regions_list",
		mcp.WithDescription("Lists all available Linode regions (datacenters) with optional filtering by country or capabilities"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("country",
			mcp.Description("Filter regions by country code (e.g., 'us', 'de', 'jp')"),
		),
		mcp.WithString("capability",
			mcp.Description("Filter regions by capability (e.g., 'Linodes', 'Block Storage', 'GPU Linodes')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeRegionsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeRegionsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	countryFilter := request.GetString("country", "")
	capabilityFilter := request.GetString("capability", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	regions, err := client.ListRegions(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode regions: %v", err)), nil
	}

	if countryFilter != "" {
		regions = filterRegionsByCountry(regions, countryFilter)
	}

	if capabilityFilter != "" {
		regions = filterRegionsByCapability(regions, capabilityFilter)
	}

	return formatRegionsResponse(regions, countryFilter, capabilityFilter)
}

func filterRegionsByCountry(regions []linode.Region, countryFilter string) []linode.Region {
	filtered := make([]linode.Region, 0, len(regions))

	countryFilter = strings.ToLower(countryFilter)

	for _, region := range regions {
		if strings.ToLower(region.Country) == countryFilter {
			filtered = append(filtered, region)
		}
	}

	return filtered
}

func filterRegionsByCapability(regions []linode.Region, capabilityFilter string) []linode.Region {
	filtered := make([]linode.Region, 0, len(regions))

	capabilityFilter = strings.ToLower(capabilityFilter)

	for _, region := range regions {
		for _, cap := range region.Capabilities {
			if strings.ToLower(cap) == capabilityFilter {
				filtered = append(filtered, region)

				break
			}
		}
	}

	return filtered
}

func formatRegionsResponse(regions []linode.Region, countryFilter, capabilityFilter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count   int             `json:"count"`
		Filter  string          `json:"filter,omitempty"`
		Regions []linode.Region `json:"regions"`
	}{
		Count:   len(regions),
		Regions: regions,
	}

	var filters []string
	if countryFilter != "" {
		filters = append(filters, "country="+countryFilter)
	}

	if capabilityFilter != "" {
		filters = append(filters, "capability="+capabilityFilter)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return marshalToolResponse(response)
}

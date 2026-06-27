package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeRegionListTool creates a tool for listing Linode regions.
func NewLinodeRegionListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_region_list",
		mcp.WithDescription("Lists all available Linode regions (datacenters) with optional filtering by country or capabilities"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("country", mcp.Description("Filter regions by country code (e.g., 'us', 'de', 'jp')")),
		mcp.WithString("capability", mcp.Description("Filter regions by capability (e.g., 'Linodes', 'Block Storage', 'GPU Linodes')")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListRequest(
			ctx, &request, cfg,
			func(ctx context.Context, client *linode.Client) ([]linode.Region, error) {
				return client.ListRegions(ctx)
			},
			[]filterDef[linode.Region]{
				{"country", func(items []linode.Region, v string) []linode.Region {
					return FilterByField(items, v, func(r linode.Region) string { return r.Country })
				}},
				{"capability", filterRegionsByCapability},
			},
			func(items []linode.Region, appliedFilters []string) (*mcp.CallToolResult, error) {
				return FormatListResponse(items, appliedFilters, "regions")
			},
		)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeRegionGetTool creates a tool for retrieving one Linode region.
func NewLinodeRegionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_region_get",
		"Gets one Linode region by region ID",
		toolschemas.Schema("linode.mcp.v1.RegionGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		regionID, validationMessage := regionIDFromTool(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		region, getErr := client.GetRegionProto(ctx, regionID)
		if getErr != nil {
			return regionGetToolFailure(getErr)
		}

		return MarshalProtoToolResponse(region)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeRegionAvailabilityListTool creates a tool for listing compute type availability across regions.
func NewLinodeRegionAvailabilityListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_region_availability_list",
		mcp.WithDescription("Lists compute instance type availability across Linode regions"),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListRequest(
			ctx, &request, cfg,
			func(ctx context.Context, client *linode.Client) ([]linode.RegionAvailability, error) {
				return client.ListRegionsAvailability(ctx)
			},
			nil,
			func(items []linode.RegionAvailability, appliedFilters []string) (*mcp.CallToolResult, error) {
				return FormatListResponse(items, appliedFilters, "region availability")
			},
		)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeRegionAvailabilityGetTool creates a tool for listing compute type availability in one region.
func NewLinodeRegionAvailabilityGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_region_availability_get",
		mcp.WithDescription("Lists compute instance type availability for a Linode region"),
		mcp.WithString("region_id", mcp.Required(), mcp.Description("Region slug to inspect, for example 'us-east'")),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		regionID, validationMessage := regionAvailabilityRegionIDFromTool(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		availability, failureMessage := getRegionAvailability(ctx, client, regionID)
		if failureMessage != "" {
			return mcp.NewToolResultError(failureMessage), nil
		}

		return MarshalToolResponse(availability)
	}

	return tool, profiles.CapRead, handler
}

func regionGetToolFailure(failure error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("Failed to retrieve linode_region_get: " + failure.Error()), nil
}

func regionIDFromTool(request *mcp.CallToolRequest) (string, string) {
	regionID, validationMessage := requiredStringArg(request.GetArguments(), "region_id")
	if validationMessage != "" {
		return "", validationMessage
	}

	if err := validateRegionSlug(regionID); err != nil {
		return "", "region_id must be a lowercase region slug containing only letters, numbers, and hyphens"
	}

	return regionID, ""
}

func regionAvailabilityRegionIDFromTool(request *mcp.CallToolRequest) (string, string) {
	regionID, validationMessage := requiredStringArg(request.GetArguments(), "region_id")
	if validationMessage != "" {
		return "", validationMessage
	}

	if err := validateRegionSlug(regionID); err != nil {
		return "", "region_id " + err.Error()
	}

	return regionID, ""
}

func getRegionAvailability(ctx context.Context, client *linode.Client, regionID string) ([]linode.RegionAvailability, string) {
	availability, err := client.GetRegionAvailability(ctx, regionID)
	if err != nil {
		return nil, "Failed to retrieve linode_region_availability_get: " + err.Error()
	}

	return availability, ""
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

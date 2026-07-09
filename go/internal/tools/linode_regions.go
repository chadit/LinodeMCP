package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeRegionListTool creates a tool for listing Linode regions.
func NewLinodeRegionListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_region_list",
		"Lists all available Linode regions (datacenters) with optional filtering by country or capabilities",
		"linode.mcp.v1.RegionListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.Region, error) {
			return client.ListRegionsProto(ctx)
		},
		[]listFilterParam[*linodev1.Region]{
			fieldFilter("country", "Filter regions by country code (e.g., 'us', 'de', 'jp')",
				func(r *linodev1.Region) string { return r.GetCountry() }),
			{
				paramName:   "capability",
				description: "Filter regions by capability (e.g., 'Linodes', 'Block Storage', 'GPU Linodes')",
				matchFunc:   filterRegionsByCapability,
			},
		},
		regionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func regionListResponse(items []*linodev1.Region, count int32, filter *string) *linodev1.RegionListResponse {
	return &linodev1.RegionListResponse{Count: count, Filter: filter, Regions: items}
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
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_region_availability_list",
		"Lists compute instance type availability across Linode regions",
		"linode.mcp.v1.RegionAvailabilityListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.RegionAvailability, error) {
			return client.ListRegionsAvailabilityProto(ctx)
		},
		nil,
		regionAvailabilityListResponse,
	)

	return tool, profiles.CapRead, handler
}

func regionAvailabilityListResponse(items []*linodev1.RegionAvailability, count int32, filter *string) *linodev1.RegionAvailabilityListResponse {
	return &linodev1.RegionAvailabilityListResponse{Count: count, Filter: filter, RegionAvailabilities: items}
}

// NewLinodeRegionAvailabilityGetTool creates a tool for listing compute type availability in one region.
func NewLinodeRegionAvailabilityGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_region_availability_get",
		"Lists compute instance type availability for a Linode region",
		toolschemas.Schema("linode.mcp.v1.RegionAvailabilityGetInput"),
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

		return finishProtoList(&request, availability, nil, regionAvailabilityListResponse)
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

func getRegionAvailability(ctx context.Context, client *linode.Client, regionID string) ([]*linodev1.RegionAvailability, string) {
	availability, err := client.GetRegionAvailabilityProto(ctx, regionID)
	if err != nil {
		return nil, "Failed to retrieve linode_region_availability_get: " + err.Error()
	}

	return availability, ""
}

func filterRegionsByCapability(regions []*linodev1.Region, capabilityFilter string) []*linodev1.Region {
	filtered := make([]*linodev1.Region, 0, len(regions))

	for i := range regions {
		for _, cap := range regions[i].GetCapabilities() {
			if strings.EqualFold(cap, capabilityFilter) {
				filtered = append(filtered, regions[i])

				break
			}
		}
	}

	return filtered
}

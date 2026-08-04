package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeVPCListTool creates a tool for listing all VPCs with optional label and region filtering.
func NewLinodeVPCListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_vpc_list",
		"Lists all VPCs. Can filter by label or region.",
		"linode.mcp.v1.VpcListInput",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.Vpc, error) {
			return client.ListVPCsProto(ctx, page, pageSize)
		},
		standardPaginationFromTool,
		[]listFilterParam[*linodev1.Vpc]{
			containsFilter("label", "Filter VPCs by label containing this string (case-insensitive)",
				func(v *linodev1.Vpc) string { return v.GetLabel() }),
			fieldFilter("region", "Filter VPCs by region (exact match, case-insensitive)",
				func(v *linodev1.Vpc) string { return v.GetRegion() }),
		},
		vpcListResponse,
	)

	return tool, profiles.CapRead, handler
}

func vpcListResponse(items []*linodev1.Vpc, count int32, filter *string) *linodev1.VpcListResponse {
	return &linodev1.VpcListResponse{Count: count, Filter: filter, Vpcs: items}
}

// NewLinodeVPCGetTool creates a tool for getting a single VPC by ID.
func NewLinodeVPCGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_get",
		"Retrieves details of a single VPC by its ID",
		toolschemas.Schema("linode.mcp.v1.VpcGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleVPCGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID, validationMessage := requiredIDArgument(request, "vpc_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	vpc, err := client.GetVPCProto(ctx, vpcID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve VPC %d: %v", vpcID, err)), nil
	}

	return MarshalProtoToolResponse(vpc)
}

// NewLinodeVPCIPsListTool creates a tool for listing all VPC IP addresses across all VPCs.
func NewLinodeVPCIPsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolPaginated(
		cfg,
		"linode_vpc_ip_all_list",
		"Lists all IP addresses across all VPCs",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.VPCIP, error) {
			return client.ListVPCIPsProto(ctx, page, pageSize)
		},
		standardPaginationFromTool,
		nil,
		vpcIPListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_ip_all_list",
		"Lists all IP addresses across all VPCs",
		toolschemas.Schema("linode.mcp.v1.VPCIPAllListInput"),
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeVPCIPListTool creates a tool for listing IP addresses for a specific VPC.
func NewLinodeVPCIPListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_vpc_ip_list",
		"Lists all IP addresses for a specific VPC",
		protoListPathID{
			option: mcp.WithNumber("vpc_id", mcp.Required(), mcp.Description("The ID of the VPC")),
			parse:  parseVPCSubnetListPathID,
		},
		standardPaginationFromTool,
		func(ctx context.Context, client *linode.Client, vpcID, page, pageSize int) ([]*linodev1.VPCIP, error) {
			return client.ListVPCIPAddressesProto(ctx, vpcID, page, pageSize)
		},
		nil,
		vpcIPListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_ip_list",
		"Lists all IP addresses for a specific VPC",
		toolschemas.Schema("linode.mcp.v1.VPCIPListInput"),
	)

	return tool, profiles.CapRead, handler
}

func vpcIPListResponse(items []*linodev1.VPCIP, count int32, filter *string) *linodev1.VPCIPListResponse {
	return &linodev1.VPCIPListResponse{Count: count, Filter: filter, Ips: items}
}

// NewLinodeVPCSubnetListTool creates a tool for listing subnets in a specific VPC.
func NewLinodeVPCSubnetListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_vpc_subnet_list",
		"Lists all subnets for a specific VPC",
		protoListPathID{
			option: mcp.WithNumber("vpc_id", mcp.Required(), mcp.Description("The ID of the VPC")),
			parse:  parseVPCSubnetListPathID,
		},
		standardPaginationFromTool,
		func(ctx context.Context, client *linode.Client, vpcID, page, pageSize int) ([]*linodev1.VpcSubnet, error) {
			return client.ListVPCSubnetsProto(ctx, vpcID, page, pageSize)
		},
		nil,
		vpcSubnetListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_subnet_list",
		"Lists all subnets for a specific VPC",
		toolschemas.Schema("linode.mcp.v1.VpcSubnetListInput"),
	)

	return tool, profiles.CapRead, handler
}

// parseVPCSubnetListPathID reads the vpc_id path param for the sub-resource
// list factories, which take the reader as a value rather than calling
// requiredIDArgument themselves.
func parseVPCSubnetListPathID(request *mcp.CallToolRequest) (int, string) {
	return requiredIDArgument(request, "vpc_id")
}

func vpcSubnetListResponse(items []*linodev1.VpcSubnet, count int32, filter *string) *linodev1.VpcSubnetListResponse {
	return &linodev1.VpcSubnetListResponse{Count: count, Filter: filter, Subnets: items}
}

// NewLinodeVPCSubnetGetTool creates a tool for getting a specific subnet within a VPC.
func NewLinodeVPCSubnetGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_subnet_get",
		"Retrieves details of a specific subnet within a VPC",
		toolschemas.Schema("linode.mcp.v1.VpcSubnetGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCSubnetGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleVPCSubnetGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID, validationMessage := requiredIDArgument(request, "vpc_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	subnetID, validationMessage := requiredIDArgument(request, "subnet_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnet, err := client.GetVPCSubnetProto(ctx, vpcID, subnetID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve subnet %d for VPC %d: %v", subnetID, vpcID, err)), nil
	}

	return MarshalProtoToolResponse(subnet)
}

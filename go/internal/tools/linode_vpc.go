package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeVPCListTool creates a tool for listing all VPCs with optional label and region filtering.
func NewLinodeVPCListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_vpc_list",
		"Lists all VPCs. Can filter by label or region.",
		"linode.mcp.v1.VpcListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.Vpc, error) {
			return client.ListVPCsProto(ctx)
		},
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
	vpcID, err := parseVPCID(request.GetString("vpc_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
	_, handler := newProtoListTool(
		cfg,
		"linode_vpc_ip_all_list",
		"Lists all IP addresses across all VPCs",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.VPCIP, error) {
			return client.ListVPCIPsProto(ctx)
		},
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
	_, handler := newProtoListToolSubresource(
		cfg,
		"linode_vpc_ip_list",
		"Lists all IP addresses for a specific VPC",
		protoListPathID{
			option: mcp.WithString("vpc_id", mcp.Required(), mcp.Description("The ID of the VPC")),
			parse:  parseVPCSubnetListPathID,
		},
		func(ctx context.Context, client *linode.Client, vpcID int) ([]*linodev1.VPCIP, error) {
			return client.ListVPCIPAddressesProto(ctx, vpcID)
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
	_, handler := newProtoListToolSubresource(
		cfg,
		"linode_vpc_subnet_list",
		"Lists all subnets for a specific VPC",
		protoListPathID{
			option: mcp.WithString("vpc_id", mcp.Required(), mcp.Description("The ID of the VPC")),
			parse:  parseVPCSubnetListPathID,
		},
		func(ctx context.Context, client *linode.Client, vpcID int) ([]*linodev1.VpcSubnet, error) {
			return client.ListVPCSubnetsProto(ctx, vpcID)
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

// parseVPCSubnetListPathID validates the vpc_id path param the same way the
// non-proto handler did, returning the same error text (ErrVPCIDRequired /
// ErrVPCIDInvalid via parseVPCID).
func parseVPCSubnetListPathID(request *mcp.CallToolRequest) (int, string) {
	vpcID, err := parseVPCID(request.GetString("vpc_id", ""))
	if err != nil {
		return 0, err.Error()
	}

	return vpcID, ""
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
	vpcID, err := parseVPCID(request.GetString("vpc_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnetID, err := parseSubnetID(request.GetString("subnet_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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

// parseVPCID validates and converts the VPC ID string to an integer.
func parseVPCID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrVPCIDRequired
	}

	vpcID, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrVPCIDInvalid, raw)
	}

	return vpcID, nil
}

// parseSubnetID validates and converts the subnet ID string to an integer.
func parseSubnetID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrSubnetIDRequired
	}

	subnetID, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrSubnetIDInvalid, raw)
	}

	return subnetID, nil
}

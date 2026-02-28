package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeVPCsListTool creates a tool for listing all VPCs with optional label and region filtering.
func NewLinodeVPCsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_vpcs_list",
		"Lists all VPCs. Can filter by label or region.",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.VPC, error) {
			return client.ListVPCs(ctx)
		},
		[]listFilterParam[linode.VPC]{
			containsFilter("label", "Filter VPCs by label containing this string (case-insensitive)",
				func(v linode.VPC) string { return v.Label }),
			fieldFilter("region", "Filter VPCs by region (exact match, case-insensitive)",
				func(v linode.VPC) string { return v.Region }),
		},
		"vpcs",
	)
}

// NewLinodeVPCGetTool creates a tool for getting a single VPC by ID.
func NewLinodeVPCGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_get",
		"Retrieves details of a single VPC by its ID",
		[]mcp.ToolOption{
			mcp.WithString("vpc_id",
				mcp.Required(),
				mcp.Description("The ID of the VPC to retrieve"),
			),
		},
		handleVPCGetRequest,
	)
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

	vpc, err := client.GetVPC(ctx, vpcID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve VPC %d: %v", vpcID, err)), nil
	}

	return marshalToolResponse(vpc)
}

// NewLinodeVPCIPsListTool creates a tool for listing all VPC IP addresses across all VPCs.
func NewLinodeVPCIPsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_vpc_ips_list",
		"Lists all IP addresses across all VPCs",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.VPCIP, error) {
			return client.ListVPCIPs(ctx)
		},
		nil,
		"ips",
	)
}

// NewLinodeVPCIPListTool creates a tool for listing IP addresses for a specific VPC.
func NewLinodeVPCIPListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_ip_list",
		"Lists all IP addresses for a specific VPC",
		[]mcp.ToolOption{
			mcp.WithString("vpc_id",
				mcp.Required(),
				mcp.Description("The ID of the VPC"),
			),
		},
		handleVPCIPListRequest,
	)
}

func handleVPCIPListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID, err := parseVPCID(request.GetString("vpc_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ips, err := client.ListVPCIPAddresses(ctx, vpcID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list IP addresses for VPC %d: %v", vpcID, err)), nil
	}

	response := struct {
		Count int            `json:"count"`
		IPs   []linode.VPCIP `json:"ips"`
	}{
		Count: len(ips),
		IPs:   ips,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCSubnetsListTool creates a tool for listing subnets in a specific VPC.
func NewLinodeVPCSubnetsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_subnets_list",
		"Lists all subnets for a specific VPC",
		[]mcp.ToolOption{
			mcp.WithString("vpc_id",
				mcp.Required(),
				mcp.Description("The ID of the VPC"),
			),
		},
		handleVPCSubnetsListRequest,
	)
}

func handleVPCSubnetsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID, err := parseVPCID(request.GetString("vpc_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnets, err := client.ListVPCSubnets(ctx, vpcID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list subnets for VPC %d: %v", vpcID, err)), nil
	}

	response := struct {
		Count   int                `json:"count"`
		Subnets []linode.VPCSubnet `json:"subnets"`
	}{
		Count:   len(subnets),
		Subnets: subnets,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCSubnetGetTool creates a tool for getting a specific subnet within a VPC.
func NewLinodeVPCSubnetGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_subnet_get",
		"Retrieves details of a specific subnet within a VPC",
		[]mcp.ToolOption{
			mcp.WithString("vpc_id",
				mcp.Required(),
				mcp.Description("The ID of the VPC"),
			),
			mcp.WithString("subnet_id",
				mcp.Required(),
				mcp.Description("The ID of the subnet to retrieve"),
			),
		},
		handleVPCSubnetGetRequest,
	)
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

	subnet, err := client.GetVPCSubnet(ctx, vpcID, subnetID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve subnet %d for VPC %d: %v", subnetID, vpcID, err)), nil
	}

	return marshalToolResponse(subnet)
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

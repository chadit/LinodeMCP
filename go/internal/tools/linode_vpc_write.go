package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeVPCCreateTool creates a tool for creating a new VPC.
func NewLinodeVPCCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_create",
		"Creates a new VPC. WARNING: This creates a billable resource. "+
			"Use linode_regions_list to find valid region values.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the VPC (1-64 characters)")),
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region for the VPC (e.g. us-east). Use linode_regions_list to find valid values.")),
			mcp.WithString("description",
				mcp.Description("Description for the VPC (optional)")),
			mcp.WithString("subnets",
				mcp.Description("JSON array of subnets to create: [{\"label\": \"my-subnet\", \"ipv4\": \"10.0.0.0/24\"}] (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm VPC creation. This creates a billable resource.")),
		},
		handleVPCCreateRequest,
	)
}

func handleVPCCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This creates a billable VPC resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	label := request.GetString("label", "")
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	region := request.GetString("region", "")
	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	req := linode.CreateVPCRequest{
		Label:  label,
		Region: region,
	}

	if description := request.GetString("description", ""); description != "" {
		req.Description = description
	}

	if subnetsJSON := request.GetString("subnets", ""); subnetsJSON != "" {
		var subnets []linode.CreateSubnetRequest
		if err := json.Unmarshal([]byte(subnetsJSON), &subnets); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid subnets JSON: %v", err)), nil
		}

		req.Subnets = subnets
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	vpc, err := client.CreateVPC(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create VPC: %v", err)), nil
	}

	response := struct {
		Message string      `json:"message"`
		VPC     *linode.VPC `json:"vpc"`
	}{
		Message: fmt.Sprintf("VPC '%s' (ID: %d) created in %s", vpc.Label, vpc.ID, vpc.Region),
		VPC:     vpc,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCUpdateTool creates a tool for updating an existing VPC.
func NewLinodeVPCUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_update",
		"Updates an existing VPC's label or description.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC to update")),
			mcp.WithString("label",
				mcp.Description("New label for the VPC (optional)")),
			mcp.WithString("description",
				mcp.Description("New description for the VPC (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm VPC update.")),
		},
		handleVPCUpdateRequest,
	)
}

func handleVPCUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This modifies the VPC configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	vpcID := request.GetInt("vpc_id", 0)
	if vpcID == 0 {
		return mcp.NewToolResultError("vpc_id is required"), nil
	}

	req := linode.UpdateVPCRequest{}

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
	}

	if description := request.GetString("description", ""); description != "" {
		req.Description = description
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	vpc, err := client.UpdateVPC(ctx, vpcID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update VPC %d: %v", vpcID, err)), nil
	}

	response := struct {
		Message string      `json:"message"`
		VPC     *linode.VPC `json:"vpc"`
	}{
		Message: fmt.Sprintf("VPC %d updated successfully", vpcID),
		VPC:     vpc,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCDeleteTool creates a tool for deleting a VPC.
func NewLinodeVPCDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_delete",
		"Deletes a VPC. WARNING: This is irreversible. All subnets within the VPC will also be deleted.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deletion. This action is irreversible and deletes all subnets.")),
		},
		handleVPCDeleteRequest,
	)
}

func handleVPCDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This is irreversible. All subnets in the VPC will also be deleted. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	vpcID := request.GetInt("vpc_id", 0)
	if vpcID == 0 {
		return mcp.NewToolResultError("vpc_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteVPC(ctx, vpcID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete VPC %d: %v", vpcID, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		VPCID   int    `json:"vpc_id"`
	}{
		Message: fmt.Sprintf("VPC %d deleted successfully", vpcID),
		VPCID:   vpcID,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCSubnetCreateTool creates a tool for creating a subnet within a VPC.
func NewLinodeVPCSubnetCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_subnet_create",
		"Creates a new subnet within a VPC.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC to add the subnet to")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the subnet")),
			mcp.WithString("ipv4", mcp.Required(),
				mcp.Description("IPv4 range for the subnet in CIDR notation (e.g. 10.0.0.0/24)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm subnet creation.")),
		},
		handleVPCSubnetCreateRequest,
	)
}

func handleVPCSubnetCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This creates a new subnet in the VPC. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	vpcID := request.GetInt("vpc_id", 0)
	if vpcID == 0 {
		return mcp.NewToolResultError("vpc_id is required"), nil
	}

	label := request.GetString("label", "")
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	ipv4 := request.GetString("ipv4", "")
	if ipv4 == "" {
		return mcp.NewToolResultError("ipv4 is required (CIDR notation, e.g. 10.0.0.0/24)"), nil
	}

	req := linode.CreateSubnetRequest{
		Label: label,
		IPv4:  ipv4,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnet, err := client.CreateVPCSubnet(ctx, vpcID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create subnet in VPC %d: %v", vpcID, err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		Subnet  *linode.VPCSubnet `json:"subnet"`
	}{
		Message: fmt.Sprintf("Subnet '%s' (ID: %d) created in VPC %d", subnet.Label, subnet.ID, vpcID),
		Subnet:  subnet,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCSubnetUpdateTool creates a tool for updating a subnet within a VPC.
func NewLinodeVPCSubnetUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_subnet_update",
		"Updates the label of a subnet within a VPC.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC containing the subnet")),
			mcp.WithNumber("subnet_id", mcp.Required(),
				mcp.Description("The ID of the subnet to update")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("New label for the subnet")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm subnet update.")),
		},
		handleVPCSubnetUpdateRequest,
	)
}

func handleVPCSubnetUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This modifies the subnet configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	vpcID := request.GetInt("vpc_id", 0)
	if vpcID == 0 {
		return mcp.NewToolResultError("vpc_id is required"), nil
	}

	subnetID := request.GetInt("subnet_id", 0)
	if subnetID == 0 {
		return mcp.NewToolResultError("subnet_id is required"), nil
	}

	label := request.GetString("label", "")
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	req := linode.UpdateSubnetRequest{
		Label: label,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnet, err := client.UpdateVPCSubnet(ctx, vpcID, subnetID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update subnet %d in VPC %d: %v", subnetID, vpcID, err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		Subnet  *linode.VPCSubnet `json:"subnet"`
	}{
		Message: fmt.Sprintf("Subnet %d in VPC %d updated successfully", subnetID, vpcID),
		Subnet:  subnet,
	}

	return marshalToolResponse(response)
}

// NewLinodeVPCSubnetDeleteTool creates a tool for deleting a subnet from a VPC.
func NewLinodeVPCSubnetDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_vpc_subnet_delete",
		"Deletes a subnet from a VPC. WARNING: This is irreversible.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC containing the subnet")),
			mcp.WithNumber("subnet_id", mcp.Required(),
				mcp.Description("The ID of the subnet to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm subnet deletion. This action is irreversible.")),
		},
		handleVPCSubnetDeleteRequest,
	)
}

func handleVPCSubnetDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := requireConfirm(request, "This is irreversible. The subnet will be permanently deleted. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	vpcID := request.GetInt("vpc_id", 0)
	if vpcID == 0 {
		return mcp.NewToolResultError("vpc_id is required"), nil
	}

	subnetID := request.GetInt("subnet_id", 0)
	if subnetID == 0 {
		return mcp.NewToolResultError("subnet_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteVPCSubnet(ctx, vpcID, subnetID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete subnet %d from VPC %d: %v", subnetID, vpcID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		VPCID    int    `json:"vpc_id"`
		SubnetID int    `json:"subnet_id"`
	}{
		Message:  fmt.Sprintf("Subnet %d deleted from VPC %d successfully", subnetID, vpcID),
		VPCID:    vpcID,
		SubnetID: subnetID,
	}

	return marshalToolResponse(response)
}

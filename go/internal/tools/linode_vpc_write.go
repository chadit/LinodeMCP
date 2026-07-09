package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeVPCCreateTool creates a tool for creating a new VPC.
func NewLinodeVPCCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_create",
		"Creates a new VPC. WARNING: This creates a billable resource. "+
			"Use linode_region_list to find valid region values.",
		toolschemas.Schema("linode.mcp.v1.VpcCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateVPCCreateArgs validates the VPC create args, returning an error
// message or "". Shared by the real create path and the dry-run preview.
func validateVPCCreateArgs(label, region string) string {
	if label == "" {
		return errLabelRequired
	}

	if region == "" {
		return errRegionRequired
	}

	return ""
}

func handleVPCCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label := request.GetString("label", "")
	region := request.GetString("region", "")

	if IsDryRun(request) {
		if msg := validateVPCCreateArgs(label, region); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_vpc_create", httpMethodPost, "/vpcs", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return vpcCreateSideEffects(ctx, label, region)
			})
	}

	if result := RequireConfirm(request, "This creates a billable VPC resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateVPCCreateArgs(label, region); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	req := linode.CreateVPCRequest{
		Label:  label,
		Region: region,
	}

	if description := request.GetString("description", ""); description != "" {
		req.Description = description
	}

	if rawSubnets, present := request.GetArguments()["subnets"]; present {
		subnets, validationMessage := objectSliceFromToolArg[linode.CreateSubnetRequest](rawSubnets, "subnets")
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		req.Subnets = subnets
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	vpc, err := client.CreateVPCProto(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create VPC: %v", err)), nil
	}

	response := &linodev1.VpcWriteResponse{
		Message: fmt.Sprintf("VPC '%s' (ID: %d) created in %s", vpc.GetLabel(), vpc.GetId(), vpc.GetRegion()),
		Vpc:     vpc,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVPCUpdateTool creates a tool for updating an existing VPC.
func NewLinodeVPCUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_update",
		"Updates an existing VPC's label or description.",
		toolschemas.Schema("linode.mcp.v1.VpcUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleVPCUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID := request.GetInt("vpc_id", 0)

	if IsDryRun(request) {
		if vpcID == 0 {
			return mcp.NewToolResultError("vpc_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_vpc_update", "PUT",
			fmt.Sprintf("/vpcs/%d", vpcID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVPC(ctx, vpcID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return vpcUpdateSideEffects(ctx, state,
					request.GetString("label", ""), request.GetString("description", ""))
			})
	}

	if result := RequireConfirm(request, "This modifies the VPC configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

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

	vpc, err := client.UpdateVPCProto(ctx, vpcID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify VPC %d: %v", vpcID, err)), nil
	}

	response := &linodev1.VpcWriteResponse{
		Message: fmt.Sprintf("VPC %d modified successfully", vpcID),
		Vpc:     vpc,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVPCDeleteTool creates a tool for deleting a VPC.
func NewLinodeVPCDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_delete",
		"Deletes a VPC. WARNING: This is irreversible. All subnets within the VPC will also be deleted."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.VpcDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// vpcDeleteProto builds the proto-canonical id-echo body for a successful VPC
// delete, keeping the proto literal off the handler's struct literal so the
// delete handlers stay below the dupl threshold.
func vpcDeleteProto(id int) proto.Message {
	return &linodev1.VpcDeleteResponse{
		Message: fmt.Sprintf("VPC %d removed successfully", id),
		VpcId:   linodeIDToInt32(id),
	}
}

func handleVPCDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_vpc_delete",
		IDParam:        "vpc_id",
		Method:         httpMethodDelete,
		PathPattern:    "/vpcs/%d",
		ConfirmMessage: "This is irreversible. All subnets in the VPC will also be deleted. Set confirm=true to proceed.",
		SuccessProto:   vpcDeleteProto,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetVPC(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteVPC(ctx, id)
		},
		DependencyWalk: vpcDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("VPC"),
	})
}

// NewLinodeVPCSubnetCreateTool creates a tool for creating a subnet within a VPC.
func NewLinodeVPCSubnetCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_subnet_create",
		"Creates a new subnet within a VPC.",
		toolschemas.Schema("linode.mcp.v1.VpcSubnetCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCSubnetCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateVPCSubnetCreateArgs validates the subnet create args, returning an
// error message or "". Shared by the real create path and the dry-run preview.
func validateVPCSubnetCreateArgs(vpcID int, label, ipv4 string) string {
	if vpcID == 0 {
		return "vpc_id is required"
	}

	if label == "" {
		return errLabelRequired
	}

	if ipv4 == "" {
		return "ipv4 is required (CIDR notation, e.g. 10.0.0.0/24)"
	}

	return ""
}

func handleVPCSubnetCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID := request.GetInt("vpc_id", 0)
	label := request.GetString("label", "")
	ipv4 := request.GetString("ipv4", "")

	if IsDryRun(request) {
		if msg := validateVPCSubnetCreateArgs(vpcID, label, ipv4); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_vpc_subnet_create", httpMethodPost,
			fmt.Sprintf("/vpcs/%d/subnets", vpcID), nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return vpcSubnetCreateSideEffects(ctx, label, ipv4, vpcID)
			})
	}

	if result := RequireConfirm(request, "This creates a new subnet in the VPC. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateVPCSubnetCreateArgs(vpcID, label, ipv4); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	req := linode.CreateSubnetRequest{
		Label: label,
		IPv4:  ipv4,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnet, err := client.CreateVPCSubnetProto(ctx, vpcID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create subnet in VPC %d: %v", vpcID, err)), nil
	}

	response := &linodev1.VpcSubnetWriteResponse{
		Message: fmt.Sprintf("Subnet '%s' (ID: %d) created in VPC %d", subnet.GetLabel(), subnet.GetId(), vpcID),
		Subnet:  subnet,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVPCSubnetUpdateTool creates a tool for updating a subnet within a VPC.
func NewLinodeVPCSubnetUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_subnet_update",
		"Updates the label of a subnet within a VPC.",
		toolschemas.Schema("linode.mcp.v1.VpcSubnetUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCSubnetUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateVPCSubnetUpdateArgs validates the subnet update args, returning an
// error message or "". Shared by the real update path and the dry-run preview.
func validateVPCSubnetUpdateArgs(vpcID, subnetID int, label string) string {
	if vpcID == 0 {
		return "vpc_id is required"
	}

	if subnetID == 0 {
		return "subnet_id is required"
	}

	if label == "" {
		return errLabelRequired
	}

	return ""
}

func handleVPCSubnetUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	vpcID := request.GetInt("vpc_id", 0)
	subnetID := request.GetInt("subnet_id", 0)
	label := request.GetString("label", "")

	if IsDryRun(request) {
		if msg := validateVPCSubnetUpdateArgs(vpcID, subnetID, label); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_vpc_subnet_update", "PUT",
			fmt.Sprintf("/vpcs/%d/subnets/%d", vpcID, subnetID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVPCSubnet(ctx, vpcID, subnetID) })
	}

	if result := RequireConfirm(request, "This modifies the subnet configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateVPCSubnetUpdateArgs(vpcID, subnetID, label); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	req := linode.UpdateSubnetRequest{
		Label: label,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subnet, err := client.UpdateVPCSubnetProto(ctx, vpcID, subnetID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify subnet %d in VPC %d: %v", subnetID, vpcID, err)), nil
	}

	response := &linodev1.VpcSubnetWriteResponse{
		Message: fmt.Sprintf("Subnet %d in VPC %d modified successfully", subnetID, vpcID),
		Subnet:  subnet,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVPCSubnetDeleteTool creates a tool for deleting a subnet from a VPC.
func NewLinodeVPCSubnetDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vpc_subnet_delete",
		"Deletes a subnet from a VPC. WARNING: This is irreversible."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.VpcSubnetDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleVPCSubnetDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// vpcSubnetDeleteProto builds the proto-canonical id-echo body for a successful
// subnet delete, keeping the proto literal off the handler's struct literal so
// the delete handlers stay below the dupl threshold.
func vpcSubnetDeleteProto(vpcID, subnetID int) proto.Message {
	return &linodev1.VpcSubnetDeleteResponse{
		Message:  fmt.Sprintf("Subnet %d deleted from VPC %d successfully", subnetID, vpcID),
		VpcId:    linodeIDToInt32(vpcID),
		SubnetId: linodeIDToInt32(subnetID),
	}
}

func handleVPCSubnetDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_vpc_subnet_delete",
		OuterIDParam:   "vpc_id",
		InnerIDParam:   "subnet_id",
		Method:         httpMethodDelete,
		PathPattern:    "/vpcs/%d/subnets/%d",
		ConfirmMessage: "This is irreversible. The subnet will be permanently deleted. Set confirm=true to proceed.",
		SuccessProto:   vpcSubnetDeleteProto,
		FetchState: func(ctx context.Context, c *linode.Client, vpcID, subnetID int) (any, error) {
			return c.GetVPCSubnet(ctx, vpcID, subnetID)
		},
		Execute: func(ctx context.Context, c *linode.Client, vpcID, subnetID int) error {
			return c.DeleteVPCSubnet(ctx, vpcID, subnetID)
		},
		DependencyWalk: vpcSubnetDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("VPCSubnet"),
	})
}

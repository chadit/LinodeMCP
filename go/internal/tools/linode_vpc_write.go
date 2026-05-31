package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeVPCCreateTool creates a tool for creating a new VPC.
func NewLinodeVPCCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_vpc_create",
		"Creates a new VPC. WARNING: This creates a billable resource. "+
			"Use linode_region_list to find valid region values.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the VPC (1-64 characters)")),
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region for the VPC (e.g. us-east). Use linode_region_list to find valid values.")),
			mcp.WithString("description",
				mcp.Description("Description for the VPC (optional)")),
			mcp.WithString("subnets",
				mcp.Description("JSON array of subnets to create: [{\"label\": \"my-subnet\", \"ipv4\": \"10.0.0.0/24\"}] (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm VPC creation. This creates a billable resource. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleVPCCreateRequest,
	)

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

	return MarshalToolResponse(response)
}

// NewLinodeVPCUpdateTool creates a tool for updating an existing VPC.
func NewLinodeVPCUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
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
				mcp.Description("Must be true to confirm VPC update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleVPCUpdateRequest,
	)

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

	vpc, err := client.UpdateVPC(ctx, vpcID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify VPC %d: %v", vpcID, err)), nil
	}

	response := struct {
		Message string      `json:"message"`
		VPC     *linode.VPC `json:"vpc"`
	}{
		Message: fmt.Sprintf("VPC %d modified successfully", vpcID),
		VPC:     vpc,
	}

	return MarshalToolResponse(response)
}

// NewLinodeVPCDeleteTool creates a tool for deleting a VPC.
func NewLinodeVPCDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_vpc_delete",
		"Deletes a VPC. WARNING: This is irreversible. All subnets within the VPC will also be deleted."+
			" Pass dry_run=true to preview without deleting.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deletion. This action is irreversible and deletes all subnets. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleVPCDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleVPCDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_vpc_delete",
		IDParam:        "vpc_id",
		Method:         httpMethodDelete,
		PathPattern:    "/vpcs/%d",
		ConfirmMessage: "This is irreversible. All subnets in the VPC will also be deleted. Set confirm=true to proceed.",
		SuccessFormat:  "VPC %d removed successfully",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetVPC(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteVPC(ctx, id)
		},
		DependencyWalk: vpcDeleteDependencyWalk,
	})
}

// NewLinodeVPCSubnetCreateTool creates a tool for creating a subnet within a VPC.
func NewLinodeVPCSubnetCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
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
				mcp.Description("Must be true to confirm subnet creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleVPCSubnetCreateRequest,
	)

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

	return MarshalToolResponse(response)
}

// NewLinodeVPCSubnetUpdateTool creates a tool for updating a subnet within a VPC.
func NewLinodeVPCSubnetUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
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
				mcp.Description("Must be true to confirm subnet update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleVPCSubnetUpdateRequest,
	)

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

	subnet, err := client.UpdateVPCSubnet(ctx, vpcID, subnetID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify subnet %d in VPC %d: %v", subnetID, vpcID, err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		Subnet  *linode.VPCSubnet `json:"subnet"`
	}{
		Message: fmt.Sprintf("Subnet %d in VPC %d modified successfully", subnetID, vpcID),
		Subnet:  subnet,
	}

	return MarshalToolResponse(response)
}

// NewLinodeVPCSubnetDeleteTool creates a tool for deleting a subnet from a VPC.
func NewLinodeVPCSubnetDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_vpc_subnet_delete",
		"Deletes a subnet from a VPC. WARNING: This is irreversible."+
			" Pass dry_run=true to preview without deleting.",
		[]mcp.ToolOption{
			mcp.WithNumber("vpc_id", mcp.Required(),
				mcp.Description("The ID of the VPC containing the subnet")),
			mcp.WithNumber("subnet_id", mcp.Required(),
				mcp.Description("The ID of the subnet to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm subnet deletion. This action is irreversible. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleVPCSubnetDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleVPCSubnetDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_vpc_subnet_delete",
		OuterIDParam:   "vpc_id",
		InnerIDParam:   "subnet_id",
		Method:         httpMethodDelete,
		PathPattern:    "/vpcs/%d/subnets/%d",
		ConfirmMessage: "This is irreversible. The subnet will be permanently deleted. Set confirm=true to proceed.",
		SuccessFormat:  "Subnet %d deleted from VPC %d successfully",
		FetchState: func(ctx context.Context, c *linode.Client, vpcID, subnetID int) (any, error) {
			return c.GetVPCSubnet(ctx, vpcID, subnetID)
		},
		Execute: func(ctx context.Context, c *linode.Client, vpcID, subnetID int) error {
			return c.DeleteVPCSubnet(ctx, vpcID, subnetID)
		},
		DependencyWalk: vpcSubnetDeleteDependencyWalk,
	})
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	paramSkipIPv6RDNS = "skip_ipv6_rdns"
	paramIPs          = "ips"
)

// NewLinodeNetworkingIPListTool creates a tool for listing account IP addresses.
func NewLinodeNetworkingIPListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ips_list",
		"Lists IP addresses on the account. Set skip_ipv6_rdns to true to skip IPv6 reverse DNS lookups.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramSkipIPv6RDNS, mcp.Description("Skip IPv6 reverse DNS lookups (optional).")),
		},
		handleLinodeNetworkingIPListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeNetworkingIPListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	skipIPv6RDNS, validationMessage := optionalNetworkingBoolArg(args, paramSkipIPv6RDNS)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ips, err := client.ListNetworkingIPs(ctx, skipIPv6RDNS)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve networking IPs: %v", err)), nil
	}

	return MarshalToolResponse(ips)
}

// NewLinodeNetworkingIPAllocateTool creates a tool for allocating an account-level IP address.
func NewLinodeNetworkingIPAllocateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ip_allocate",
		"Allocates an account-level IP address. WARNING: Additional IPs may incur charges.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode that receives the new IP address.")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("The type of IP address to allocate, for example ipv4.")),
			mcp.WithBoolean("public", mcp.Required(),
				mcp.Description("Whether the IP address should be public.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IP allocation. Additional IPs may incur charges.")),
		},
		handleLinodeNetworkingIPAllocateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPAllocateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This allocates a new IP address which may incur charges. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := networkingIPAllocateRequestFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipAddr, err := client.AllocateNetworkingIP(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to allocate networking IP: %v", err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		IP      *linode.IPAddress `json:"ip"`
	}{
		Message: fmt.Sprintf("IP %s allocated for Linode %d", ipAddr.Address, req.LinodeID),
		IP:      ipAddr,
	}

	return MarshalToolResponse(response)
}

// NewLinodeNetworkingIPAssignTool creates a tool for assigning IP addresses to Linodes.
func NewLinodeNetworkingIPAssignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ips_assign",
		"Assigns IP addresses to Linodes in a region. WARNING: This changes IP ownership assignments.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("The region for the IP assignments.")),
			mcp.WithString("assignments", mcp.Required(),
				mcp.Description("JSON array of assignments, each with address and linode_id.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IP reassignment.")),
		},
		handleLinodeNetworkingIPAssignRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPAssignRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This assigns IP addresses to Linodes. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := networkingIPAssignRequestFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	response, err := client.AssignNetworkingIPs(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to assign networking IPs: %v", err)), nil
	}

	return MarshalToolResponse(struct {
		Message  string         `json:"message"`
		Response map[string]any `json:"response"`
	}{
		Message:  "Networking IP assignments updated",
		Response: response,
	})
}

// NewLinodeNetworkingIPShareTool creates a tool for sharing IP addresses with a primary Linode.
func NewLinodeNetworkingIPShareTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ips_share",
		"Shares IP addresses with a primary Linode. Set ips to a JSON string array; an empty array removes all shared IP addresses.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the primary Linode that receives the shared IP addresses.")),
			mcp.WithString(paramIPs, mcp.Required(),
				mcp.Description("JSON array of IP addresses or IPv6 ranges to share. Use [] to remove all shared IP addresses.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm changing shared IP assignments.")),
		},
		handleLinodeNetworkingIPShareRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPShareRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This changes shared IP assignments. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := networkingIPShareRequestFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	response, err := client.ShareNetworkingIPs(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to share networking IPs: %v", err)), nil
	}

	return MarshalToolResponse(struct {
		Message  string         `json:"message"`
		Response map[string]any `json:"response"`
	}{
		Message:  "Networking IP sharing updated",
		Response: response,
	})
}

func networkingIPAssignRequestFromTool(args map[string]any) (linode.AssignNetworkingIPsRequest, string) {
	region, validationMessage := requiredStringArg(args, "region")
	if validationMessage != "" {
		return linode.AssignNetworkingIPsRequest{}, validationMessage
	}

	assignmentsJSON, validationMessage := requiredStringArg(args, "assignments")
	if validationMessage != "" {
		return linode.AssignNetworkingIPsRequest{}, validationMessage
	}

	var assignments []linode.IPAssignment
	if err := json.Unmarshal([]byte(assignmentsJSON), &assignments); err != nil {
		return linode.AssignNetworkingIPsRequest{}, "assignments must be a JSON array of objects with address and linode_id"
	}

	if len(assignments) == 0 {
		return linode.AssignNetworkingIPsRequest{}, "assignments must include at least one assignment"
	}

	for _, assignment := range assignments {
		if assignment.Address == "" {
			return linode.AssignNetworkingIPsRequest{}, "assignment address is required"
		}

		if assignment.LinodeID <= 0 {
			return linode.AssignNetworkingIPsRequest{}, "assignment linode_id must be a positive integer"
		}
	}

	return linode.AssignNetworkingIPsRequest{Region: region, Assignments: assignments}, ""
}

func networkingIPShareRequestFromTool(args map[string]any) (linode.ShareNetworkingIPsRequest, string) {
	linodeID, ok := numberArgToInt(args["linode_id"])
	if !ok || linodeID <= 0 {
		return linode.ShareNetworkingIPsRequest{}, "linode_id must be a positive integer"
	}

	ipsJSON, validationMessage := requiredStringArg(args, paramIPs)
	if validationMessage != "" {
		return linode.ShareNetworkingIPsRequest{}, validationMessage
	}

	var ips []string
	if err := json.Unmarshal([]byte(ipsJSON), &ips); err != nil || ips == nil {
		return linode.ShareNetworkingIPsRequest{}, "ips must be a JSON array of strings"
	}

	if slices.Contains(ips, "") {
		return linode.ShareNetworkingIPsRequest{}, "ips must not include blank IP addresses"
	}

	return linode.ShareNetworkingIPsRequest{LinodeID: linodeID, IPs: ips}, ""
}

func networkingIPAllocateRequestFromTool(args map[string]any) (linode.AllocateNetworkingIPRequest, string) {
	linodeID, ok := numberArgToInt(args["linode_id"])
	if !ok || linodeID <= 0 {
		return linode.AllocateNetworkingIPRequest{}, "linode_id must be a positive integer"
	}

	ipType, validationMessage := requiredStringArg(args, "type")
	if validationMessage != "" {
		return linode.AllocateNetworkingIPRequest{}, validationMessage
	}

	public, validationMessage := requiredNetworkingBoolArg(args, "public")
	if validationMessage != "" {
		return linode.AllocateNetworkingIPRequest{}, validationMessage
	}

	return linode.AllocateNetworkingIPRequest{LinodeID: linodeID, Public: public, Type: ipType}, ""
}

func optionalNetworkingBoolArg(args map[string]any, key string) (bool, string) {
	raw, rawFound := args[key]
	if !rawFound {
		return false, ""
	}

	value, valueIsBool := raw.(bool)
	if !valueIsBool {
		return false, key + " must be a boolean"
	}

	return value, ""
}

func requiredNetworkingBoolArg(args map[string]any, key string) (bool, string) {
	raw, rawFound := args[key]
	if !rawFound {
		return false, key + " is required"
	}

	value, valueIsBool := raw.(bool)
	if !valueIsBool {
		return false, key + " must be a boolean"
	}

	return value, ""
}

package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const paramSkipIPv6RDNS = "skip_ipv6_rdns"

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

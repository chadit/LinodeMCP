package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
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

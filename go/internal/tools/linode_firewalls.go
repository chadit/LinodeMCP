package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeFirewallListTool creates a tool for listing firewalls.
func NewLinodeFirewallListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newListTool(
		cfg,
		"linode_firewall_list",
		"Lists all Cloud Firewalls on your account. Can filter by status or label.",
		func(ctx context.Context, client *linode.Client) ([]linode.Firewall, error) {
			return client.ListFirewalls(ctx)
		},
		[]listFilterParam[linode.Firewall]{
			fieldFilter("status", "Filter by firewall status (enabled, disabled, deleted)",
				func(f linode.Firewall) string { return f.Status }),
			containsFilter("label_contains", "Filter firewalls by label containing this string (case-insensitive)",
				func(f linode.Firewall) string { return f.Label }),
		},
		"firewalls",
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeFirewallSettingsListTool creates a tool for listing default firewall assignments.
func NewLinodeFirewallSettingsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_settings_list",
		"Lists default Cloud Firewall assignments for Linodes, NodeBalancers, public interfaces, and VPC interfaces.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeFirewallSettingsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallSettingsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, err := client.ListFirewallSettings(ctx, request.GetInt("page", 0), request.GetInt("page_size", 0))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_settings_list: %v", err)), nil
	}

	return MarshalToolResponse(settings)
}

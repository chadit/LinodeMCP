package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeFirewallsListTool creates a tool for listing firewalls.
func NewLinodeFirewallsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_firewalls_list",
		"Lists all Cloud Firewalls on your account. Can filter by status or label.",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.Firewall, error) {
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
}

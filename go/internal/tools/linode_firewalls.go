//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeFirewallsListTool creates a tool for listing firewalls.
func NewLinodeFirewallsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_firewalls_list",
		mcp.WithDescription("Lists all Cloud Firewalls on your account. Can filter by status or label."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("status",
			mcp.Description("Filter by firewall status (enabled, disabled, deleted)"),
		),
		mcp.WithString("label_contains",
			mcp.Description("Filter firewalls by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeFirewallsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	statusFilter := request.GetString("status", "")
	labelContains := request.GetString("label_contains", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	firewalls, err := client.ListFirewalls(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve firewalls: %v", err)), nil
	}

	if statusFilter != "" {
		firewalls = filterFirewallsByStatus(firewalls, statusFilter)
	}

	if labelContains != "" {
		firewalls = filterFirewallsByLabel(firewalls, labelContains)
	}

	return formatFirewallsResponse(firewalls, statusFilter, labelContains)
}

func filterFirewallsByStatus(firewalls []linode.Firewall, statusFilter string) []linode.Firewall {
	filtered := make([]linode.Firewall, 0, len(firewalls))

	statusFilter = strings.ToLower(statusFilter)

	for _, firewall := range firewalls {
		if strings.ToLower(firewall.Status) == statusFilter {
			filtered = append(filtered, firewall)
		}
	}

	return filtered
}

func filterFirewallsByLabel(firewalls []linode.Firewall, labelContains string) []linode.Firewall {
	filtered := make([]linode.Firewall, 0, len(firewalls))

	labelContains = strings.ToLower(labelContains)

	for _, firewall := range firewalls {
		if strings.Contains(strings.ToLower(firewall.Label), labelContains) {
			filtered = append(filtered, firewall)
		}
	}

	return filtered
}

func formatFirewallsResponse(firewalls []linode.Firewall, statusFilter, labelContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count     int               `json:"count"`
		Filter    string            `json:"filter,omitempty"`
		Firewalls []linode.Firewall `json:"firewalls"`
	}{
		Count:     len(firewalls),
		Firewalls: firewalls,
	}

	var filters []string
	if statusFilter != "" {
		filters = append(filters, "status="+statusFilter)
	}

	if labelContains != "" {
		filters = append(filters, "label_contains="+labelContains)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return marshalToolResponse(response)
}

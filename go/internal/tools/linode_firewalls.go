package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	firewallDefaultLinodeKey = "linode"
	paramDefaultFirewallIDs  = "default_firewall_ids"
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

// NewLinodeFirewallSettingsUpdateTool creates a tool for updating default firewall assignments.
func NewLinodeFirewallSettingsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_settings_update",
		"Updates default Cloud Firewall assignments for Linodes, NodeBalancers, public interfaces, and VPC interfaces.",
		[]mcp.ToolOption{
			mcp.WithObject(paramDefaultFirewallIDs, mcp.Required(),
				mcp.Description("Object of positive firewall IDs keyed by linode, nodebalancer, public_interface, or vpc_interface.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm default firewall settings update.")),
		},
		handleLinodeFirewallSettingsUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeFirewallSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates default Cloud Firewall assignments. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := firewallSettingsUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, failureMessage := updateFirewallSettings(ctx, client, req)
	if failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message  string                   `json:"message"`
		Settings *linode.FirewallSettings `json:"settings"`
	}{
		Message:  "Default firewall settings updated successfully",
		Settings: settings,
	})
}

func updateFirewallSettings(
	ctx context.Context,
	client *linode.Client,
	req *linode.UpdateFirewallSettingsRequest,
) (*linode.FirewallSettings, string) {
	settings, err := client.UpdateFirewallSettings(ctx, req)
	if err != nil {
		return nil, "Failed to update linode_firewall_settings_update: " + err.Error()
	}

	return settings, ""
}

func firewallSettingsUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateFirewallSettingsRequest, string) {
	rawDefaultIDs, foundDefaultIDs := request.GetArguments()[paramDefaultFirewallIDs]
	if !foundDefaultIDs {
		return nil, "default_firewall_ids is required"
	}

	ids, validDefaultIDs := rawDefaultIDs.(map[string]any)
	if !validDefaultIDs || len(ids) == 0 {
		return nil, "default_firewall_ids must be a non-empty object"
	}

	var seen int

	req := &linode.UpdateFirewallSettingsRequest{}

	for key, rawValue := range ids {
		value, ok := positiveFirewallID(rawValue)
		if !ok {
			return nil, "default_firewall_ids." + key + " must be a positive integer"
		}

		switch key {
		case firewallDefaultLinodeKey:
			req.DefaultFirewallIDs.Linode = &value
		case "nodebalancer":
			req.DefaultFirewallIDs.NodeBalancer = &value
		case "public_interface":
			req.DefaultFirewallIDs.PublicInterface = &value
		case "vpc_interface":
			req.DefaultFirewallIDs.VPCInterface = &value
		default:
			return nil, "default_firewall_ids contains unsupported key: " + key
		}

		seen++
	}

	if seen == 0 {
		return nil, "default_firewall_ids must include at least one firewall ID"
	}

	return req, ""
}

func positiveFirewallID(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, value > 0
	case float64:
		intValue := int(value)

		return intValue, value == float64(intValue) && intValue > 0
	default:
		return 0, false
	}
}

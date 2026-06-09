package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

// NewLinodeFirewallCreateTool creates a tool for creating a firewall.
func NewLinodeFirewallCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_create",
		"Creates a new Cloud Firewall. The firewall is created with no rules by default.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(),
				mcp.Description("A label for the firewall (must be unique)")),
			mcp.WithString("inbound_policy",
				mcp.Description("Default policy for inbound traffic: 'ACCEPT' or 'DROP' (optional, default: 'ACCEPT')")),
			mcp.WithString("outbound_policy",
				mcp.Description("Default policy for outbound traffic: 'ACCEPT' or 'DROP' (optional, default: 'ACCEPT')")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm firewall creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeFirewallCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateFirewallCreateArgs validates the firewall create args, returning an
// error message or "". Shared by the real create path and the dry-run preview.
func validateFirewallCreateArgs(label, inboundPolicy, outboundPolicy string) string {
	if label == "" {
		return errLabelRequired
	}

	if err := validateFirewallPolicy(inboundPolicy); err != nil {
		return fmt.Sprintf("inbound_policy: %v", err)
	}

	if err := validateFirewallPolicy(outboundPolicy); err != nil {
		return fmt.Sprintf("outbound_policy: %v", err)
	}

	return ""
}

func handleLinodeFirewallCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label := request.GetString("label", "")
	inboundPolicy := request.GetString("inbound_policy", "ACCEPT")
	outboundPolicy := request.GetString("outbound_policy", "ACCEPT")

	if IsDryRun(request) {
		if msg := validateFirewallCreateArgs(label, inboundPolicy, outboundPolicy); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_firewall_create", httpMethodPost, "/networking/firewalls", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return firewallCreateSideEffects(ctx, label, inboundPolicy, outboundPolicy)
			})
	}

	if result := RequireConfirm(request, "This creates a Cloud Firewall. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateFirewallCreateArgs(label, inboundPolicy, outboundPolicy); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateFirewallRequest{
		Label: label,
		Rules: &linode.FirewallRules{
			InboundPolicy:  inboundPolicy,
			OutboundPolicy: outboundPolicy,
		},
	}

	firewall, err := client.CreateFirewall(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create firewall: %v", err)), nil
	}

	response := struct {
		Message  string           `json:"message"`
		Firewall *linode.Firewall `json:"firewall"`
	}{
		Message:  fmt.Sprintf("Firewall '%s' (ID: %d) created successfully", firewall.Label, firewall.ID),
		Firewall: firewall,
	}

	return MarshalToolResponse(response)
}

// NewLinodeFirewallUpdateTool creates a tool for updating a firewall.
func NewLinodeFirewallUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_update",
		"Updates an existing Cloud Firewall. Can modify label, status, and default policies.",
		[]mcp.ToolOption{
			mcp.WithNumber("firewall_id", mcp.Required(), mcp.Description("The ID of the firewall to update")),
			mcp.WithString("label", mcp.Description("New label for the firewall (optional)")),
			mcp.WithString("status", mcp.Description("New status: 'enabled' or 'disabled' (optional)")),
			mcp.WithString("inbound_policy", mcp.Description("Default policy for inbound traffic: 'ACCEPT' or 'DROP' (optional)")),
			mcp.WithString("outbound_policy", mcp.Description("Default policy for outbound traffic: 'ACCEPT' or 'DROP' (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm firewall update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeFirewallUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateFirewallUpdateArgs validates the firewall update args, returning an
// error message or "". Shared by the real update path and the dry-run preview.
func validateFirewallUpdateArgs(firewallID int, inboundPolicy, outboundPolicy string) string {
	if firewallID == 0 {
		return "firewall_id is required"
	}

	if inboundPolicy != "" {
		if err := validateFirewallPolicy(inboundPolicy); err != nil {
			return fmt.Sprintf("inbound_policy: %v", err)
		}
	}

	if outboundPolicy != "" {
		if err := validateFirewallPolicy(outboundPolicy); err != nil {
			return fmt.Sprintf("outbound_policy: %v", err)
		}
	}

	return ""
}

func handleLinodeFirewallUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt("firewall_id", 0)
	label := request.GetString("label", "")
	status := request.GetString("status", "")
	inboundPolicy := request.GetString("inbound_policy", "")
	outboundPolicy := request.GetString("outbound_policy", "")

	if IsDryRun(request) {
		if msg := validateFirewallUpdateArgs(firewallID, inboundPolicy, outboundPolicy); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_firewall_update", "PUT",
			fmt.Sprintf("/networking/firewalls/%d", firewallID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetFirewall(ctx, firewallID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return firewallUpdateSideEffects(ctx, state, label, status)
			})
	}

	if result := RequireConfirm(request, "This updates a Cloud Firewall. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateFirewallUpdateArgs(firewallID, inboundPolicy, outboundPolicy); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateFirewallRequest{
		Label:  label,
		Status: status,
	}

	if inboundPolicy != "" || outboundPolicy != "" {
		req.Rules = &linode.FirewallRules{
			InboundPolicy:  inboundPolicy,
			OutboundPolicy: outboundPolicy,
		}
	}

	firewall, err := client.UpdateFirewall(ctx, firewallID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify firewall %d: %v", firewallID, err)), nil
	}

	response := struct {
		Message  string           `json:"message"`
		Firewall *linode.Firewall `json:"firewall"`
	}{
		Message:  fmt.Sprintf("Firewall %d modified successfully", firewallID),
		Firewall: firewall,
	}

	return MarshalToolResponse(response)
}

// NewLinodeFirewallDeleteTool creates a tool for deleting a firewall.
func NewLinodeFirewallDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_delete",
		"Deletes a Cloud Firewall. WARNING: This will remove all firewall rules and unassign all attached devices."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithNumber("firewall_id", mcp.Required(), mcp.Description("The ID of the firewall to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleLinodeFirewallDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeFirewallDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_firewall_delete",
		IDParam:        "firewall_id",
		Method:         httpMethodDelete,
		PathPattern:    "/networking/firewalls/%d",
		ConfirmMessage: destroyConfirmMessage,
		SuccessFormat:  "Firewall %d removed successfully",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetFirewall(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteFirewall(ctx, id)
		},
		DependencyWalk: firewallDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("Firewall"),
	})
}

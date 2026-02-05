//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeFirewallCreateTool creates a tool for creating a firewall.
func NewLinodeFirewallCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_firewall_create",
		mcp.WithDescription("Creates a new Cloud Firewall. The firewall is created with no rules by default."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("A label for the firewall (must be unique)"),
		),
		mcp.WithString("inbound_policy",
			mcp.Description("Default policy for inbound traffic: 'ACCEPT' or 'DROP' (optional, default: 'ACCEPT')"),
		),
		mcp.WithString("outbound_policy",
			mcp.Description("Default policy for outbound traffic: 'ACCEPT' or 'DROP' (optional, default: 'ACCEPT')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeFirewallCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	label := request.GetString("label", "")
	inboundPolicy := request.GetString("inbound_policy", "ACCEPT")
	outboundPolicy := request.GetString("outbound_policy", "ACCEPT")

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeFirewallUpdateTool creates a tool for updating a firewall.
func NewLinodeFirewallUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_firewall_update",
		mcp.WithDescription("Updates an existing Cloud Firewall. Can modify label, status, and default policies."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("firewall_id",
			mcp.Required(),
			mcp.Description("The ID of the firewall to update"),
		),
		mcp.WithString("label",
			mcp.Description("New label for the firewall (optional)"),
		),
		mcp.WithString("status",
			mcp.Description("New status: 'enabled' or 'disabled' (optional)"),
		),
		mcp.WithString("inbound_policy",
			mcp.Description("Default policy for inbound traffic: 'ACCEPT' or 'DROP' (optional)"),
		),
		mcp.WithString("outbound_policy",
			mcp.Description("Default policy for outbound traffic: 'ACCEPT' or 'DROP' (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallUpdateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeFirewallUpdateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	firewallID := request.GetInt("firewall_id", 0)
	label := request.GetString("label", "")
	status := request.GetString("status", "")
	inboundPolicy := request.GetString("inbound_policy", "")
	outboundPolicy := request.GetString("outbound_policy", "")

	if firewallID == 0 {
		return mcp.NewToolResultError("firewall_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update firewall %d: %v", firewallID, err)), nil
	}

	response := struct {
		Message  string           `json:"message"`
		Firewall *linode.Firewall `json:"firewall"`
	}{
		Message:  fmt.Sprintf("Firewall %d updated successfully", firewallID),
		Firewall: firewall,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeFirewallDeleteTool creates a tool for deleting a firewall.
func NewLinodeFirewallDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_firewall_delete",
		mcp.WithDescription("Deletes a Cloud Firewall. WARNING: This will remove all firewall rules and unassign all attached devices."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("firewall_id",
			mcp.Required(),
			mcp.Description("The ID of the firewall to delete"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeFirewallDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	firewallID := request.GetInt("firewall_id", 0)
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation is destructive. Set confirm=true to proceed."), nil
	}

	if firewallID == 0 {
		return mcp.NewToolResultError("firewall_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DeleteFirewall(ctx, firewallID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete firewall %d: %v", firewallID, err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		FirewallID int    `json:"firewall_id"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message:    fmt.Sprintf("Firewall %d deleted successfully", firewallID),
		FirewallID: firewallID,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

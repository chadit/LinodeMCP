package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeFirewallCreateTool creates a tool for creating a firewall.
func NewLinodeFirewallCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_create",
		"Creates a new Cloud Firewall. The firewall is created with no rules by default.",
		toolschemas.Schema("linode.mcp.v1.FirewallCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateFirewallCreateArgs validates the firewall create args, returning an
// error message or "". Shared by the real create path and the dry-run preview.
func validateFirewallCreateArgs(label, inboundPolicy, outboundPolicy string) string {
	if label == "" {
		return errLabelRequired
	}

	if msg := enumChoiceError(inboundPolicy, "inbound_policy", linodev1.FirewallPolicy_Value_value); msg != "" {
		return msg
	}

	if msg := enumChoiceError(outboundPolicy, "outbound_policy", linodev1.FirewallPolicy_Value_value); msg != "" {
		return msg
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

	firewall, err := client.CreateFirewallProto(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create firewall: %v", err)), nil
	}

	response := &linodev1.FirewallWriteResponse{
		Message:  fmt.Sprintf("Firewall '%s' (ID: %d) created successfully", firewall.GetLabel(), firewall.GetId()),
		Firewall: firewall,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeFirewallUpdateTool creates a tool for updating a firewall.
func NewLinodeFirewallUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_update",
		"Updates an existing Cloud Firewall. Can modify label, status, and default policies.",
		toolschemas.Schema("linode.mcp.v1.FirewallUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateFirewallUpdateArgs validates the firewall update args, returning an
// error message or "". Shared by the real update path and the dry-run preview.
func validateFirewallUpdateArgs(firewallID int, inboundPolicy, outboundPolicy string) string {
	if firewallID == 0 {
		return "firewall_id is required"
	}

	if msg := enumChoiceError(inboundPolicy, "inbound_policy", linodev1.FirewallPolicy_Value_value); msg != "" {
		return msg
	}

	if msg := enumChoiceError(outboundPolicy, "outbound_policy", linodev1.FirewallPolicy_Value_value); msg != "" {
		return msg
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

	firewall, err := client.UpdateFirewallProto(ctx, firewallID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify firewall %d: %v", firewallID, err)), nil
	}

	response := &linodev1.FirewallWriteResponse{
		Message:  fmt.Sprintf("Firewall %d modified successfully", firewallID),
		Firewall: firewall,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeFirewallDeleteTool creates a tool for deleting a firewall.
func NewLinodeFirewallDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_delete",
		"Deletes a Cloud Firewall. WARNING: This will remove all firewall rules and unassign all attached devices."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.FirewallDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// firewallDeleteProto builds the proto-canonical id-echo body for a successful
// firewall delete, keeping the proto literal off the handler's struct literal
// so the delete handlers stay below the dupl threshold.
func firewallDeleteProto(id int) proto.Message {
	return &linodev1.FirewallDeleteResponse{
		Message:    fmt.Sprintf("Firewall %d removed successfully", id),
		FirewallId: linodeIDToInt32(id),
	}
}

func handleLinodeFirewallDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_firewall_delete",
		IDParam:        "firewall_id",
		Method:         httpMethodDelete,
		PathPattern:    "/networking/firewalls/%d",
		ConfirmMessage: destroyConfirmMessage,
		SuccessProto:   firewallDeleteProto,
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

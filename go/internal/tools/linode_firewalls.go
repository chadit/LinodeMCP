package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const (
	firewallDefaultLinodeKey  = "linode"
	paramDefaultFirewallIDs   = "default_firewall_ids"
	paramDeviceID             = "id"
	paramDeviceType           = "type"
	paramFirewallDeviceID     = "device_id"
	paramFirewallID           = "firewall_id"
	paramFirewallRuleInbound  = "inbound"
	paramFirewallRuleOutbound = "outbound"
	paramFirewallRuleVersion  = "version"
	paramSlug                 = "slug"
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

// NewLinodeFirewallGetTool creates a tool for retrieving a single Cloud Firewall by ID.
func NewLinodeFirewallGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_get",
		"Gets a Cloud Firewall by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall to retrieve (required)")),
		},
		handleLinodeFirewallGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt(paramFirewallID, 0)
	if firewallID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	firewall, err := client.GetFirewall(ctx, firewallID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve firewall %d: %v", firewallID, err)), nil
	}

	// Summary shape (rule counts instead of full rule bodies) matches the
	// Python implementation's response for this tool; the full rules are
	// available via linode_firewall_rules_get.
	return MarshalToolResponse(firewallGetResponse{
		Firewall: firewallSummary{
			ID:                 firewall.ID,
			Label:              firewall.Label,
			Status:             firewall.Status,
			RulesInboundCount:  len(firewall.Rules.Inbound),
			RulesOutboundCount: len(firewall.Rules.Outbound),
			Created:            firewall.Created,
			Updated:            firewall.Updated,
			Tags:               firewall.Tags,
		},
	})
}

// firewallGetResponse wraps the firewall summary under a "firewall" key to
// match the Python implementation's response shape for linode_firewall_get.
type firewallGetResponse struct {
	Firewall firewallSummary `json:"firewall"`
}

// firewallSummary is the condensed firewall view linode_firewall_get returns:
// identity, status, rule counts, and timestamps without the full rule bodies.
type firewallSummary struct {
	ID                 int      `json:"id"`
	Label              string   `json:"label"`
	Status             string   `json:"status"`
	RulesInboundCount  int      `json:"rules_inbound_count"`
	RulesOutboundCount int      `json:"rules_outbound_count"`
	Created            string   `json:"created"`
	Updated            string   `json:"updated"`
	Tags               []string `json:"tags"`
}

// NewLinodeVLANsListTool creates a tool for listing VLANs.
func NewLinodeVLANsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newLinodeIPv6ListTool(
		cfg,
		"linode_vlan_list",
		"Lists VLANs on the account with optional pagination.",
		func(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.VLAN], string) {
			vlans, err := client.ListVLANs(ctx, page, pageSize)
			if err != nil {
				return nil, err.Error()
			}

			return vlans, ""
		},
	)
}

// NewLinodeVLANDeleteTool creates a tool for deleting one VLAN.
func NewLinodeVLANDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_vlan_delete",
		"Deletes one VLAN by region and label. Pass dry_run=true to preview without deleting."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithString("region_id", mcp.Required(),
				mcp.Description("The region ID for the VLAN, for example us-east.")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("The VLAN label.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm deleting the VLAN. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleLinodeVLANDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeVLANDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	regionID, validationMessage := vlanRegionPathParamFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	label, validationMessage := vlanPathParamFromTool(request, "label")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_vlan_delete",
		Method:         httpMethodDelete,
		Path:           "/networking/vlans/" + regionID + "/" + label,
		ConfirmMessage: "This deletes a VLAN. Set confirm=true to proceed.",
		// VLANs have no single-GET endpoint, only a paginated list.
		// The dry-run fetch lists and filters to the matching
		// region+label. A VLAN paged out beyond the first 500 results
		// (extreme edge case) would read as "not found".
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return findVLAN(ctx, c, regionID, label)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteVLAN(ctx, regionID, label)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: "VLAN " + label + " deleted successfully from region " + regionID,
				"region_id":        regionID,
				"label":            label,
			}
		},
		// A VLAN carries no cosmetic timestamp, so the whole state is hashed;
		// the unknown "VLAN" key returns nil.
		HashIgnore: twostage.HashIgnoreFields("VLAN"),
	})
}

// findVLAN resolves a single VLAN by region+label for the dry-run
// preview. VLANs expose only a paginated list endpoint, so this filters
// the first page (max page size) rather than issuing a single-resource
// GET. Returns ErrVLANNotFound when no VLAN matches.
func findVLAN(ctx context.Context, client *linode.Client, regionID, label string) (any, error) {
	const maxVLANPageSize = 500

	vlans, err := client.ListVLANs(ctx, 1, maxVLANPageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list VLANs: %w", err)
	}

	for i := range vlans.Data {
		vlan := vlans.Data[i]
		if vlan.Region == regionID && vlan.Label == label {
			return vlan, nil
		}
	}

	return nil, fmt.Errorf("%w: %s in region %s", ErrVLANNotFound, label, regionID)
}

func vlanRegionPathParamFromTool(request *mcp.CallToolRequest) (string, string) {
	regionID := request.GetString("region_id", "")
	if regionID == "" {
		return "", "region_id is required"
	}

	if err := validateRegionSlug(regionID); err != nil {
		return "", "region_id must be a lowercase region slug"
	}

	return regionID, ""
}

func vlanPathParamFromTool(request *mcp.CallToolRequest, name string) (string, string) {
	value := request.GetString(name, "")
	if value == "" {
		return "", name + " is required"
	}

	if value != strings.TrimSpace(value) || strings.ContainsAny(value, "/?#") || strings.Contains(value, "..") {
		return "", name + " must not contain path separators, query separators, or traversal segments"
	}

	return value, ""
}

// NewLinodeFirewallRulesListTool creates a tool for listing rules for a Cloud Firewall.
func NewLinodeFirewallRulesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_rules_get",
		"Lists rules for a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose rules should be listed.")),
		},
		handleLinodeFirewallRulesListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallRulesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredPositiveIntArgument(
		request,
		paramFirewallID,
		linode.ErrFirewallIDPositive.Error(),
		linode.ErrFirewallIDPositive.Error(),
	)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rules, err := client.ListFirewallRules(ctx, firewallID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_rules_get: %v", err)), nil
	}

	return MarshalToolResponse(rules)
}

// NewLinodeFirewallRulesUpdateTool creates a tool for replacing rules for a Cloud Firewall.
func NewLinodeFirewallRulesUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_rules_update",
		"Replaces inbound and outbound rules for a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose rules should be replaced.")),
			mcp.WithArray(paramFirewallRuleInbound, mcp.Required(),
				mcp.Description("Array of inbound firewall rule objects.")),
			mcp.WithArray(paramFirewallRuleOutbound, mcp.Required(),
				mcp.Description("Array of outbound firewall rule objects.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm replacing firewall rules. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeFirewallRulesUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeFirewallRulesUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return handleLinodeFirewallRulesUpdateDryRun(ctx, request, cfg)
	}

	if result := RequireConfirm(request, "This replaces Cloud Firewall rules. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	firewallID, validationMessage := requiredPositiveIntArgument(
		request,
		paramFirewallID,
		linode.ErrFirewallIDPositive.Error(),
		linode.ErrFirewallIDPositive.Error(),
	)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	inbound, validationMessage := firewallRuleSetFromTool(request, paramFirewallRuleInbound)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	outbound, validationMessage := firewallRuleSetFromTool(request, paramFirewallRuleOutbound)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.FirewallRules{Inbound: inbound, Outbound: outbound}

	rules, err := client.UpdateFirewallRules(ctx, firewallID, &req)
	if err != nil {
		return mcp.NewToolResultError(formatFirewallRulesUpdateError(err)), nil
	}

	response := struct {
		Message    string                `json:"message"`
		FirewallID int                   `json:"firewall_id"`
		Rules      *linode.FirewallRules `json:"rules"`
	}{
		Message:    fmt.Sprintf("Firewall %d rules updated successfully", firewallID),
		FirewallID: firewallID,
		Rules:      rules,
	}

	return MarshalToolResponse(response)
}

func handleLinodeFirewallRulesUpdateDryRun(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredPositiveIntArgument(
		request,
		paramFirewallID,
		linode.ErrFirewallIDPositive.Error(),
		linode.ErrFirewallIDPositive.Error(),
	)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if _, msg := firewallRuleSetFromTool(request, paramFirewallRuleInbound); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	if _, msg := firewallRuleSetFromTool(request, paramFirewallRuleOutbound); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	return RunDryRunPreview(ctx, request, cfg, "linode_firewall_rules_update", "PUT",
		fmt.Sprintf("/networking/firewalls/%d/rules", firewallID),
		func(ctx context.Context, c *linode.Client) (any, error) { return c.ListFirewallRules(ctx, firewallID) })
}

func formatFirewallRulesUpdateError(err error) string {
	return "Failed to update linode_firewall_rules_update: " + err.Error()
}

// firewallRuleSetFromTool reads a required native array of firewall-rule objects.
// An empty array is valid (it clears that direction's rules); an absent, null, or
// malformed value is rejected.
func firewallRuleSetFromTool(request *mcp.CallToolRequest, name string) ([]linode.FirewallRule, string) {
	raw, present := request.GetArguments()[name]
	if !present {
		return nil, name + " is required"
	}

	rules, validationMessage := objectSliceFromToolArg[linode.FirewallRule](raw, name)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if rules == nil {
		return nil, name + " must be an array of objects"
	}

	return rules, ""
}

// NewLinodeFirewallRuleVersionsListTool creates a tool for retrieving rule-version history for a Cloud Firewall.
func NewLinodeFirewallRuleVersionsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_rule_version_list",
		"Retrieves the rule-version history payload for a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose rule versions should be listed.")),
		},
		handleLinodeFirewallRuleVersionsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallRuleVersionsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt(paramFirewallID, 0)
	if firewallID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	firewall, err := client.ListFirewallRuleVersions(ctx, firewallID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_rule_version_list: %v", err)), nil
	}

	return MarshalToolResponse(firewall)
}

// NewLinodeFirewallRuleVersionGetTool creates a tool for retrieving one Cloud Firewall rule version.
func NewLinodeFirewallRuleVersionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_rule_version_get",
		"Retrieves one rule version for a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose rule version should be retrieved.")),
			mcp.WithNumber(paramFirewallRuleVersion, mcp.Required(),
				mcp.Description("The firewall rule version number to retrieve.")),
		},
		handleLinodeFirewallRuleVersionGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallRuleVersionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredPositiveIntArgument(
		request,
		paramFirewallID,
		linode.ErrFirewallIDPositive.Error(),
		linode.ErrFirewallIDPositive.Error(),
	)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	version, validationMessage := requiredPositiveIntArgument(
		request,
		paramFirewallRuleVersion,
		linode.ErrFirewallRuleVersionPositive.Error(),
		linode.ErrFirewallRuleVersionPositive.Error(),
	)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rule, err := client.GetFirewallRuleVersion(ctx, firewallID, version)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_rule_version_get: %v", err)), nil
	}

	return MarshalToolResponse(rule)
}

// NewLinodeFirewallDevicesListTool creates a tool for listing devices assigned to a Cloud Firewall.
func NewLinodeFirewallDevicesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_device_list",
		"Lists devices assigned to a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose assigned devices should be listed.")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeFirewallDevicesListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallDevicesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt(paramFirewallID, 0)
	if firewallID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	devices, err := client.ListFirewallDevices(ctx, firewallID, request.GetInt("page", 0), request.GetInt("page_size", 0))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_device_list: %v", err)), nil
	}

	return MarshalToolResponse(devices)
}

// NewLinodeFirewallDeviceGetTool creates a tool for retrieving one device assigned to a Cloud Firewall.
func NewLinodeFirewallDeviceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_device_get",
		"Gets one device assigned to a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose assigned device should be retrieved.")),
			mcp.WithNumber(paramFirewallDeviceID, mcp.Required(),
				mcp.Description("The ID of the firewall device assignment to retrieve.")),
		},
		handleLinodeFirewallDeviceGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallDeviceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt(paramFirewallID, 0)
	if firewallID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
	}

	deviceID := request.GetInt(paramFirewallDeviceID, 0)
	if deviceID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallDeviceIDPositive.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	device, err := client.GetFirewallDevice(ctx, firewallID, deviceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_device_get: %v", err)), nil
	}

	return MarshalToolResponse(device)
}

// NewLinodeFirewallDeviceCreateTool creates a tool for assigning a device to a Cloud Firewall.
func NewLinodeFirewallDeviceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_device_create",
		"Assigns a Linode, Linode interface, or NodeBalancer device to a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall to assign the device to.")),
			mcp.WithNumber(paramDeviceID, mcp.Required(),
				mcp.Description("The positive ID of the Linode, Linode interface, or NodeBalancer to assign.")),
			mcp.WithString(paramDeviceType, mcp.Required(),
				mcp.Description("Device type. Must be linode, nodebalancer, or linode_interface.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm firewall device assignment. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeFirewallDeviceCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeFirewallDeviceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt(paramFirewallID, 0)

	if IsDryRun(request) {
		if firewallID <= 0 {
			return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
		}

		if _, validationMessage := firewallDeviceCreateRequestFromTool(request); validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_firewall_device_create", httpMethodPost,
			fmt.Sprintf("/networking/firewalls/%d/devices", firewallID), nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return firewallDeviceCreateSideEffects(ctx,
					request.GetString(paramDeviceType, ""), request.GetInt(paramDeviceID, 0), firewallID)
			})
	}

	if result := RequireConfirm(request, "This assigns a device to a Cloud Firewall. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if firewallID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
	}

	req, validationMessage := firewallDeviceCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	device, failureMessage := createFirewallDevice(ctx, client, firewallID, req)
	if failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message string                 `json:"message"`
		Device  *linode.FirewallDevice `json:"device"`
	}{
		Message: "Firewall device assigned successfully",
		Device:  device,
	})
}

func firewallDeviceCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateFirewallDeviceRequest, string) {
	deviceID := request.GetInt(paramDeviceID, 0)
	if deviceID <= 0 {
		return nil, linode.ErrFirewallDeviceIDPositive.Error()
	}

	deviceType := request.GetString(paramDeviceType, "")
	if deviceType == "" {
		return nil, linode.ErrFirewallDeviceTypeRequired.Error()
	}

	if validationMessage := validateFirewallDeviceType(deviceType); validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.CreateFirewallDeviceRequest{ID: deviceID, Type: deviceType}, ""
}

func validateFirewallDeviceType(deviceType string) string {
	switch deviceType {
	case firewallDefaultLinodeKey, "nodebalancer", "linode_interface":
		return ""
	default:
		return linode.ErrInvalidFirewallDeviceType.Error()
	}
}

func createFirewallDevice(
	ctx context.Context,
	client *linode.Client,
	firewallID int,
	req *linode.CreateFirewallDeviceRequest,
) (*linode.FirewallDevice, string) {
	device, err := client.CreateFirewallDevice(ctx, firewallID, req)
	if err != nil {
		return nil, "Failed to create linode_firewall_device_create: " + err.Error()
	}

	return device, ""
}

// NewLinodeFirewallDeviceDeleteTool creates a tool for removing a device assignment from a Cloud Firewall.
func NewLinodeFirewallDeviceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_device_delete",
		"Removes one device assignment from a Cloud Firewall."+
			" Pass dry_run=true to preview without removing."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose device assignment should be removed.")),
			mcp.WithNumber(paramFirewallDeviceID, mcp.Required(),
				mcp.Description("The ID of the firewall device assignment to remove.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm firewall device removal. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleLinodeFirewallDeviceDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeFirewallDeviceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	// Pre-validation: existing tests assert specific sentinel error
	// messages for non-positive IDs. The shared two-int helper only
	// rejects `id == 0`; this guard catches negatives before either
	// branch (dry-run or real) runs. Same pattern as the negative-ID
	// guard on object_storage_key_delete (Phase 1b.2).
	if firewallID := request.GetInt(paramFirewallID, 0); firewallID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallIDPositive.Error()), nil
	}

	if deviceID := request.GetInt(paramFirewallDeviceID, 0); deviceID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallDeviceIDPositive.Error()), nil
	}

	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_firewall_device_delete",
		OuterIDParam:   paramFirewallID,
		InnerIDParam:   paramFirewallDeviceID,
		Method:         httpMethodDelete,
		PathPattern:    "/networking/firewalls/%d/devices/%d",
		ConfirmMessage: "This removes a device assignment from a Cloud Firewall. Set confirm=true to proceed.",
		SuccessFormat:  "Firewall device removed successfully",
		FetchState: func(ctx context.Context, c *linode.Client, firewallID, deviceID int) (any, error) {
			return c.GetFirewallDevice(ctx, firewallID, deviceID)
		},
		Execute: func(ctx context.Context, c *linode.Client, firewallID, deviceID int) error {
			return c.DeleteFirewallDevice(ctx, firewallID, deviceID)
		},
		HashIgnore: twostage.HashIgnoreFields("FirewallDevice"),
	})
}

// NewLinodeFirewallSettingsListTool creates a tool for listing default firewall assignments.
func NewLinodeFirewallSettingsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_settings_get",
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_settings_get: %v", err)), nil
	}

	return MarshalToolResponse(settings)
}

// NewLinodeFirewallTemplatesListTool creates a tool for listing reusable firewall templates.
func NewLinodeFirewallTemplatesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_template_list",
		"Lists reusable Cloud Firewall templates for VPC and public interfaces.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeFirewallTemplatesListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallTemplatesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	templates, err := client.ListFirewallTemplates(ctx, request.GetInt("page", 0), request.GetInt("page_size", 0))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_template_list: %v", err)), nil
	}

	return MarshalToolResponse(templates)
}

// NewLinodeFirewallTemplateGetTool creates a tool for retrieving a reusable firewall template by slug.
func NewLinodeFirewallTemplateGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_template_get",
		"Gets a reusable Cloud Firewall template for VPC or public interfaces.",
		[]mcp.ToolOption{
			mcp.WithString(paramSlug, mcp.Required(),
				mcp.Description("Firewall template slug to retrieve. Must be public or vpc.")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeFirewallTemplateGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallTemplateGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	slug := request.GetString(paramSlug, "")
	if validationMessage := validateFirewallTemplateSlug(slug); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	template, err := client.GetFirewallTemplate(ctx, slug, request.GetInt("page", 0), request.GetInt("page_size", 0))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_template_get: %v", err)), nil
	}

	return MarshalToolResponse(template)
}

func validateFirewallTemplateSlug(slug string) string {
	switch slug {
	case interfaceFieldPublic, "vpc":
		return ""
	case "":
		return "slug is required"
	default:
		return "slug must be one of public or vpc"
	}
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
				mcp.Description("Must be true to confirm default firewall settings update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeFirewallSettingsUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeFirewallSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if _, validationMessage := firewallSettingsUpdateRequestFromTool(request); validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_firewall_settings_update", "PUT",
			"/networking/firewalls/settings",
			func(ctx context.Context, c *linode.Client) (any, error) { return c.ListFirewallSettings(ctx, 0, 0) })
	}

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

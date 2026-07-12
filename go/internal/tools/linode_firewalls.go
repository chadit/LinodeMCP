package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// firewallDeviceDeleteProto builds the proto-canonical id-echo body for a
// successful firewall-device removal, keeping the proto literal off the
// handler's struct literal so the delete handlers stay below the dupl
// threshold.
func firewallDeviceDeleteProto(firewallID, deviceID int) proto.Message {
	return &linodev1.FirewallDeviceDeleteResponse{
		Message:    "Firewall device removed successfully",
		FirewallId: linodeIDToInt32(firewallID),
		DeviceId:   linodeIDToInt32(deviceID),
	}
}

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
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_firewall_list",
		"Lists all Cloud Firewalls on your account. Can filter by status or label.",
		"linode.mcp.v1.FirewallListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.Firewall, error) {
			return client.ListFirewallsProto(ctx)
		},
		[]listFilterParam[*linodev1.Firewall]{
			fieldFilter("status", "Filter by firewall status (enabled, disabled, deleted)",
				func(f *linodev1.Firewall) string { return f.GetStatus() }),
			containsFilter("label_contains", "Filter firewalls by label containing this string (case-insensitive)",
				func(f *linodev1.Firewall) string { return f.GetLabel() }),
		},
		firewallListResponse,
	)

	return tool, profiles.CapRead, handler
}

func firewallListResponse(items []*linodev1.Firewall, count int32, filter *string) *linodev1.FirewallListResponse {
	return &linodev1.FirewallListResponse{Count: count, Filter: filter, Firewalls: items}
}

// NewLinodeFirewallGetTool creates a tool for retrieving a single Cloud Firewall by ID.
func NewLinodeFirewallGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_get",
		"Gets a Cloud Firewall by ID.",
		toolschemas.Schema("linode.mcp.v1.FirewallGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredIDArgument(request, paramFirewallID)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	firewall, err := client.GetFirewallProto(ctx, firewallID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve firewall %d: %v", firewallID, err)), nil
	}

	return MarshalProtoToolResponse(firewall)
}

// NewLinodeVLANsListTool creates a tool for listing VLANs.
func NewLinodeVLANsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_vlan_list",
		"Lists VLANs on the account with optional pagination.",
		"linode.mcp.v1.VLANListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.VLAN, error) {
			return client.ListVLANsProto(ctx, page, pageSize)
		},
		ipv6ListPaginationFromTool,
		nil,
		vlanListResponse,
	)

	return tool, profiles.CapRead, handler
}

func vlanListResponse(items []*linodev1.VLAN, count int32, filter *string) *linodev1.VLANListResponse {
	return &linodev1.VLANListResponse{Count: count, Filter: filter, Vlans: items}
}

// NewLinodeVLANDeleteTool creates a tool for deleting one VLAN.
func NewLinodeVLANDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_vlan_delete",
		"Deletes one VLAN by region and label. Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.VLANDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVLANDeleteRequest(ctx, &request, cfg)
	}

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
		Success: func() proto.Message {
			return &linodev1.VLANDeleteResponse{
				Message:  "VLAN " + label + " deleted successfully from region " + regionID,
				RegionId: regionID,
				Label:    label,
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
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_rules_get",
		"Lists rules for a Cloud Firewall.",
		toolschemas.Schema("linode.mcp.v1.FirewallRulesGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallRulesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallRulesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredIDArgument(request, paramFirewallID)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rules, err := client.ListFirewallRulesProto(ctx, firewallID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_rules_get: %v", err)), nil
	}

	return MarshalProtoToolResponse(rules)
}

// NewLinodeFirewallRulesUpdateTool creates a tool for replacing rules for a Cloud Firewall.
func NewLinodeFirewallRulesUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_rules_update",
		"Replaces inbound and outbound rules for a Cloud Firewall.",
		toolschemas.Schema("linode.mcp.v1.FirewallRulesUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallRulesUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeFirewallRulesUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return handleLinodeFirewallRulesUpdateDryRun(ctx, request, cfg)
	}

	if result := RequireConfirm(request, "This replaces Cloud Firewall rules. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	firewallID, validationMessage := requiredIDArgument(request, paramFirewallID)
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

	req := linode.FirewallRulesReplaceRequest{Inbound: inbound, Outbound: outbound}

	rules, err := client.UpdateFirewallRulesProto(ctx, firewallID, &req)
	if err != nil {
		return mcp.NewToolResultError(formatFirewallRulesUpdateError(err)), nil
	}

	var firewallID32 int32
	if firewallID >= math.MinInt32 && firewallID <= math.MaxInt32 {
		firewallID32 = int32(firewallID)
	}

	response := &linodev1.FirewallRulesWriteResponse{
		Message:    fmt.Sprintf("Firewall %d rules updated successfully", firewallID),
		FirewallId: firewallID32,
		Rules:      rules,
	}

	return MarshalProtoToolResponse(response)
}

func handleLinodeFirewallRulesUpdateDryRun(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredIDArgument(request, paramFirewallID)
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
// malformed value is rejected. Rules are returned as raw maps, not typed
// linode.FirewallRule, so the caller's exact keys survive to the wire: decoding
// into FirewallRule would pad each rule with empty action/protocol/ports/label/
// description and a null ipv6 the caller never sent, drifting from the Python
// client and breaking the wire-defaults ruling.
func firewallRuleSetFromTool(request *mcp.CallToolRequest, name string) ([]map[string]any, string) {
	raw, present := request.GetArguments()[name]
	if !present {
		return nil, name + " is required"
	}

	rules, validationMessage := objectSliceFromToolArg[map[string]any](raw, name)
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
	tool, handler := newProtoListToolSubresourceRawSchema(
		cfg,
		"linode_firewall_rule_version_list",
		"Retrieves the rule-version history payload for a Cloud Firewall.",
		"linode.mcp.v1.FirewallRuleVersionListInput",
		protoListPathID{
			option: mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose rule versions should be listed.")),
			parse: firewallDeviceListFirewallIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, firewallID int) ([]*linodev1.FirewallRuleVersion, error) {
			return client.ListFirewallRuleVersionsProto(ctx, firewallID)
		},
		nil,
		firewallRuleVersionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func firewallRuleVersionListResponse(items []*linodev1.FirewallRuleVersion, count int32, filter *string) *linodev1.FirewallRuleVersionListResponse {
	return &linodev1.FirewallRuleVersionListResponse{Count: count, Filter: filter, FirewallRuleVersions: items}
}

// NewLinodeFirewallRuleVersionGetTool creates a tool for retrieving one Cloud Firewall rule version.
func NewLinodeFirewallRuleVersionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_rule_version_get",
		"Retrieves one rule version for a Cloud Firewall.",
		toolschemas.Schema("linode.mcp.v1.FirewallRuleVersionGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallRuleVersionGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallRuleVersionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredIDArgument(request, paramFirewallID)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	version, validationMessage := requiredIDArgument(request, paramFirewallRuleVersion)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ruleVersion, err := client.GetFirewallRuleVersionProto(ctx, firewallID, version)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_rule_version_get: %v", err)), nil
	}

	return MarshalProtoToolResponse(ruleVersion)
}

// NewLinodeFirewallDevicesListTool creates a tool for listing devices assigned to a Cloud Firewall.
func NewLinodeFirewallDevicesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourcePaginatedRawSchema(
		cfg,
		"linode_firewall_device_list",
		"Lists devices assigned to a Cloud Firewall.",
		"linode.mcp.v1.FirewallDeviceListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose assigned devices should be listed.")),
			parse: firewallDeviceListFirewallIDFromTool,
		},
		firewallDeviceListPaginationFromTool,
		func(ctx context.Context, client *linode.Client, firewallID, page, pageSize int) ([]*linodev1.FirewallDevice, error) {
			return client.ListFirewallDevicesProto(ctx, firewallID, page, pageSize)
		},
		nil,
		firewallDeviceListResponse,
	)

	return tool, profiles.CapRead, handler
}

// firewallDeviceListFirewallIDFromTool validates the firewall_id path param the
// same way the non-proto handler did (a non-positive id returns
// ErrFirewallIDPositive).
func firewallDeviceListFirewallIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredIDArgument(request, paramFirewallID)
}

// firewallDeviceListPaginationFromTool reads page/page_size the same way the
// non-proto handler did: a plain GetInt defaulting to 0, with no bounds
// validation, so the runtime request matches the previous behavior exactly.
func firewallDeviceListPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	return request.GetInt("page", 0), request.GetInt("page_size", 0), ""
}

func firewallDeviceListResponse(items []*linodev1.FirewallDevice, count int32, filter *string) *linodev1.FirewallDeviceListResponse {
	return &linodev1.FirewallDeviceListResponse{Count: count, Filter: filter, Devices: items}
}

// NewLinodeFirewallDeviceGetTool creates a tool for retrieving one device assigned to a Cloud Firewall.
func NewLinodeFirewallDeviceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_device_get",
		"Gets one device assigned to a Cloud Firewall.",
		toolschemas.Schema("linode.mcp.v1.FirewallDeviceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallDeviceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallDeviceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID, validationMessage := requiredIDArgument(request, paramFirewallID)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	deviceID := request.GetInt(paramFirewallDeviceID, 0)
	if deviceID <= 0 {
		return mcp.NewToolResultError(linode.ErrFirewallDeviceIDPositive.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	device, err := client.GetFirewallDeviceProto(ctx, firewallID, deviceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_device_get: %v", err)), nil
	}

	return MarshalProtoToolResponse(device)
}

// NewLinodeFirewallDeviceCreateTool creates a tool for assigning a device to a Cloud Firewall.
func NewLinodeFirewallDeviceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_device_create",
		"Assigns a Linode, Linode interface, or NodeBalancer device to a Cloud Firewall.",
		toolschemas.Schema("linode.mcp.v1.FirewallDeviceCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallDeviceCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeFirewallDeviceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	firewallID := request.GetInt(paramFirewallID, 0)

	if IsDryRun(request) {
		if _, validationMessage := requiredIDArgument(request, paramFirewallID); validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
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

	if _, validationMessage := requiredIDArgument(request, paramFirewallID); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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

	return MarshalProtoToolResponse(&linodev1.FirewallDeviceWriteResponse{
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
	return enumChoiceError(deviceType, paramDeviceType, linodev1.FirewallDeviceType_Value_value)
}

func createFirewallDevice(
	ctx context.Context,
	client *linode.Client,
	firewallID int,
	req *linode.CreateFirewallDeviceRequest,
) (*linodev1.FirewallDevice, string) {
	device, err := client.CreateFirewallDeviceProto(ctx, firewallID, req)
	if err != nil {
		return nil, "Failed to create linode_firewall_device_create: " + err.Error()
	}

	return device, ""
}

// NewLinodeFirewallDeviceDeleteTool creates a tool for removing a device assignment from a Cloud Firewall.
func NewLinodeFirewallDeviceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_device_delete",
		"Removes one device assignment from a Cloud Firewall."+
			" Pass dry_run=true to preview without removing."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.FirewallDeviceDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallDeviceDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLinodeFirewallDeviceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	// Pre-validation before the shared two-int destroy helper runs, so each
	// id path emits its own Option-B message rather than the helper's generic
	// one. Same pattern as the negative-ID guard on object_storage_key_delete.
	if _, validationMessage := requiredIDArgument(request, paramFirewallID); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
		SuccessProto:   firewallDeviceDeleteProto,
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
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_settings_get",
		"Lists default Cloud Firewall assignments for Linodes, NodeBalancers, public interfaces, and VPC interfaces.",
		toolschemas.Schema("linode.mcp.v1.FirewallSettingsGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallSettingsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeFirewallSettingsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, err := client.ListFirewallSettingsProto(ctx, request.GetInt("page", 0), request.GetInt("page_size", 0))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_settings_get: %v", err)), nil
	}

	return MarshalProtoToolResponse(settings)
}

// NewLinodeFirewallTemplatesListTool creates a tool for listing reusable firewall templates.
func NewLinodeFirewallTemplatesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_firewall_template_list",
		"Lists reusable Cloud Firewall templates for VPC and public interfaces.",
		"linode.mcp.v1.FirewallTemplateListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.FirewallTemplate, error) {
			return client.ListFirewallTemplatesProto(ctx, page, pageSize)
		},
		firewallTemplateListPaginationFromTool,
		nil,
		firewallTemplateListResponse,
	)

	return tool, profiles.CapRead, handler
}

// firewallTemplateListPaginationFromTool reads page/page_size the same way the
// non-proto handler did: a plain GetInt defaulting to 0, with no bounds
// validation, so the runtime request matches the previous behavior exactly.
func firewallTemplateListPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	return request.GetInt("page", 0), request.GetInt("page_size", 0), ""
}

func firewallTemplateListResponse(items []*linodev1.FirewallTemplate, count int32, filter *string) *linodev1.FirewallTemplateListResponse {
	return &linodev1.FirewallTemplateListResponse{Count: count, Filter: filter, FirewallTemplates: items}
}

// NewLinodeFirewallTemplateGetTool creates a tool for retrieving a reusable firewall template by slug.
func NewLinodeFirewallTemplateGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_template_get",
		"Gets a reusable Cloud Firewall template for VPC or public interfaces.",
		toolschemas.Schema("linode.mcp.v1.FirewallTemplateGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallTemplateGetRequest(ctx, &request, cfg)
	}

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

	template, err := client.GetFirewallTemplateProto(ctx, slug, request.GetInt("page", 0), request.GetInt("page_size", 0))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_template_get: %v", err)), nil
	}

	return MarshalProtoToolResponse(template)
}

func validateFirewallTemplateSlug(slug string) string {
	if slug == "" {
		return "slug is required"
	}

	return enumChoiceError(slug, paramSlug, linodev1.FirewallTemplateSlug_Value_value)
}

// NewLinodeFirewallSettingsUpdateTool creates a tool for updating default firewall assignments.
func NewLinodeFirewallSettingsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_firewall_settings_update",
		"Updates default Cloud Firewall assignments for Linodes, NodeBalancers, public interfaces, and VPC interfaces.",
		toolschemas.Schema("linode.mcp.v1.FirewallSettingsUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeFirewallSettingsUpdateRequest(ctx, &request, cfg)
	}

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

	return MarshalProtoToolResponse(&linodev1.FirewallSettingsWriteResponse{
		Message:  "Default firewall settings updated successfully",
		Settings: settings,
	})
}

func updateFirewallSettings(
	ctx context.Context,
	client *linode.Client,
	req *linode.UpdateFirewallSettingsRequest,
) (*linodev1.FirewallSettings, string) {
	settings, err := client.UpdateFirewallSettingsProto(ctx, req)
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

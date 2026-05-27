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
	paramDeviceID            = "id"
	paramDeviceType          = "type"
	paramFirewallDeviceID    = "device_id"
	paramFirewallID          = "firewall_id"
	paramFirewallRuleVersion = "version"
	paramSlug                = "slug"
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

// NewLinodeFirewallRuleVersionsListTool creates a tool for retrieving rule-version history for a Cloud Firewall.
func NewLinodeFirewallRuleVersionsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_rule_versions_list",
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_rule_versions_list: %v", err)), nil
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
		"linode_firewall_devices_list",
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_devices_list: %v", err)), nil
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
				mcp.Description("Must be true to confirm firewall device assignment.")),
		},
		handleLinodeFirewallDeviceCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeFirewallDeviceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This assigns a device to a Cloud Firewall. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	firewallID := request.GetInt(paramFirewallID, 0)
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
		"Removes one device assignment from a Cloud Firewall.",
		[]mcp.ToolOption{
			mcp.WithNumber(paramFirewallID, mcp.Required(),
				mcp.Description("The ID of the firewall whose device assignment should be removed.")),
			mcp.WithNumber(paramFirewallDeviceID, mcp.Required(),
				mcp.Description("The ID of the firewall device assignment to remove.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm firewall device removal.")),
		},
		handleLinodeFirewallDeviceDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeFirewallDeviceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This removes a device assignment from a Cloud Firewall. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

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

	if failureMessage := deleteFirewallDevice(ctx, client, firewallID, deviceID); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message    string `json:"message"`
		FirewallID int    `json:"firewall_id"`
		DeviceID   int    `json:"device_id"`
	}{
		Message:    "Firewall device removed successfully",
		FirewallID: firewallID,
		DeviceID:   deviceID,
	})
}

func deleteFirewallDevice(ctx context.Context, client *linode.Client, firewallID, deviceID int) string {
	if err := client.DeleteFirewallDevice(ctx, firewallID, deviceID); err != nil {
		return "Failed to delete linode_firewall_device_delete: " + err.Error()
	}

	return ""
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

// NewLinodeFirewallTemplatesListTool creates a tool for listing reusable firewall templates.
func NewLinodeFirewallTemplatesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_firewall_templates_list",
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve linode_firewall_templates_list: %v", err)), nil
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
	case "public", "vpc":
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

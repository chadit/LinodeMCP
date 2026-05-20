package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeInstanceBootTool creates a tool for booting a Linode instance.
func NewLinodeInstanceBootTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_boot",
		mcp.WithDescription("Boots a Linode instance that is currently offline. If the instance is already running, this has no effect."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"instance_id",
			mcp.Required(),
			mcp.Description("The ID of the Linode instance to boot"),
		),
		mcp.WithNumber(
			"config_id",
			mcp.Description("The ID of the configuration profile to boot with (optional)"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm booting the instance."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceBootRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// handleInstancePowerAction is shared by the Boot and Reboot handlers,
// which differ only by which client method they invoke and the verb in
// status messages. Centralizing the flow keeps the dupl linter happy and
// keeps the confirm/instance_id validation in one place.
func handleInstancePowerAction(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	verb, confirmMsg string,
	action func(ctx context.Context, client *linode.Client, instanceID int, configID *int) error,
) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)
	configID := request.GetInt("config_id", 0)

	if result := RequireConfirm(request, confirmMsg); result != nil {
		return result, nil
	}

	if instanceID == 0 {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var configIDPtr *int
	if configID != 0 {
		configIDPtr = &configID
	}

	if err := action(ctx, client, instanceID, configIDPtr); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to %s instance %d: %v", verb, instanceID, err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		InstanceID int    `json:"instance_id"`
	}{
		Message:    fmt.Sprintf("Instance %d %s initiated successfully", instanceID, verb),
		InstanceID: instanceID,
	}

	return MarshalToolResponse(response)
}

func handleLinodeInstanceBootRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleInstancePowerAction(
		ctx, request, cfg,
		"boot",
		"This boots a Linode instance. Set confirm=true to proceed.",
		func(ctx context.Context, client *linode.Client, instanceID int, configID *int) error {
			return client.BootInstance(ctx, instanceID, configID)
		},
	)
}

// NewLinodeInstanceRebootTool creates a tool for rebooting a Linode instance.
func NewLinodeInstanceRebootTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_reboot",
		mcp.WithDescription("Reboots a running Linode instance. This is equivalent to pressing the reset button on a physical computer."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"instance_id",
			mcp.Required(),
			mcp.Description("The ID of the Linode instance to reboot"),
		),
		mcp.WithNumber(
			"config_id",
			mcp.Description("The ID of the configuration profile to boot with after reboot (optional)"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm rebooting the instance. This causes a brief outage."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceRebootRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeInstanceRebootRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleInstancePowerAction(
		ctx, request, cfg,
		"reboot",
		"This reboots a Linode instance and causes a brief outage. Set confirm=true to proceed.",
		func(ctx context.Context, client *linode.Client, instanceID int, configID *int) error {
			return client.RebootInstance(ctx, instanceID, configID)
		},
	)
}

// NewLinodeInstanceShutdownTool creates a tool for shutting down a Linode instance.
func NewLinodeInstanceShutdownTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_shutdown",
		mcp.WithDescription("Gracefully shuts down a running Linode instance. The instance will attempt to shut down cleanly."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"instance_id",
			mcp.Required(),
			mcp.Description("The ID of the Linode instance to shut down"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm shutting down the instance."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceShutdownRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeInstanceShutdownRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)

	if result := RequireConfirm(request, "This shuts down a Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if instanceID == 0 {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.ShutdownInstance(ctx, instanceID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to shut down instance %d: %v", instanceID, err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		InstanceID int    `json:"instance_id"`
	}{
		Message:    fmt.Sprintf("Instance %d shutdown initiated successfully", instanceID),
		InstanceID: instanceID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceCreateTool creates a tool for creating a new Linode instance.
func NewLinodeInstanceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_create",
		mcp.WithDescription("Creates a new Linode instance under the current Linode Interfaces generation. WARNING: Billing starts immediately upon creation. Requires firewall_id (get one from linode_firewall_list or create with linode_firewall_create). Note: VPC attachment via the current interface model is not yet supported by this tool; use linode_vpc_* tools after create."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"region",
			mcp.Required(),
			mcp.Description("The region where the instance will be created (e.g., 'us-east')"),
		),
		mcp.WithString(
			"type",
			mcp.Required(),
			mcp.Description("The Linode plan type (e.g., 'g6-nanode-1')"),
		),
		mcp.WithString(
			"label",
			mcp.Description("A label for the instance (optional)"),
		),
		mcp.WithString(
			"image",
			mcp.Description("The image ID to deploy (e.g., 'linode/debian11'). Required for provisioned instances."),
		),
		mcp.WithString(
			"root_pass",
			mcp.Description("The root password for the instance. Required if image is provided."),
		),
		mcp.WithNumber(
			"firewall_id",
			mcp.Required(),
			mcp.Description("Cloud Firewall ID to attach to the public interface. Required under the current Linode Interfaces generation."),
		),
		mcp.WithBoolean(
			"route_ipv4",
			mcp.Description("Whether the public interface owns the IPv4 default route (optional, default: true)"),
		),
		mcp.WithBoolean(
			"route_ipv6",
			mcp.Description("Whether the public interface owns the IPv6 default route (optional, default: true)"),
		),
		mcp.WithBoolean(
			"backups_enabled",
			mcp.Description("Enable backups for this instance (optional, default: false)"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm instance creation. This operation incurs billing charges."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

const errFirewallIDRequired = "firewall_id is required for instance creation. Get a firewall ID from linode_firewall_list, or create one with linode_firewall_create."

func handleLinodeInstanceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	instanceType := request.GetString("type", "")
	label := request.GetString("label", "")
	image := request.GetString("image", "")
	rootPass := request.GetString("root_pass", "")
	backupsEnabled := request.GetBool("backups_enabled", false)
	firewallID := request.GetInt("firewall_id", 0)
	routeIPv4 := request.GetBool("route_ipv4", true)
	routeIPv6 := request.GetBool("route_ipv6", true)

	if result := RequireConfirm(request, "This operation creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if instanceType == "" {
		return mcp.NewToolResultError("type is required"), nil
	}

	if firewallID <= 0 {
		return mcp.NewToolResultError(errFirewallIDRequired), nil
	}

	if err := validateRootPassword(rootPass); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateInstanceRequest{
		Region:              region,
		Type:                instanceType,
		Label:               label,
		Image:               image,
		RootPass:            rootPass,
		BackupsEnabled:      backupsEnabled,
		InterfaceGeneration: linode.CurrentInterfaceGeneration,
		Interfaces: []linode.InstanceInterface{
			{
				Public:       &linode.InterfacePublicConfig{},
				DefaultRoute: buildDefaultRoute(routeIPv4, routeIPv6),
				FirewallID:   &firewallID,
			},
		},
	}

	instance, err := client.CreateInstance(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create instance: %v", err)), nil
	}

	response := struct {
		Message  string           `json:"message"`
		Instance *linode.Instance `json:"instance"`
	}{
		Message:  fmt.Sprintf("Instance '%s' (ID: %d) created successfully in %s", instance.Label, instance.ID, instance.Region),
		Instance: instance,
	}

	return MarshalToolResponse(response)
}

// buildDefaultRoute returns a default-route struct only when at least one
// family is selected. When neither is true, returns nil so the field is omitted
// from the wire entirely rather than sent as an empty object. When only one is
// true, the other key is omitted by the omitempty tag on the bool field.
func buildDefaultRoute(ipv4, ipv6 bool) *linode.InterfaceDefaultRoute {
	if !ipv4 && !ipv6 {
		return nil
	}

	return &linode.InterfaceDefaultRoute{IPv4: ipv4, IPv6: ipv6}
}

// NewLinodeInstanceDeleteTool creates a tool for deleting a Linode instance.
func NewLinodeInstanceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_delete",
		mcp.WithDescription("Deletes a Linode instance. WARNING: This action is irreversible and all data will be permanently lost."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"instance_id",
			mcp.Required(),
			mcp.Description("The ID of the Linode instance to delete"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion. This action is irreversible."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLinodeInstanceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)

	if result := RequireConfirm(request, "This operation is destructive and irreversible. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if instanceID == 0 {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteInstance(ctx, instanceID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove instance %d: %v", instanceID, err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		InstanceID int    `json:"instance_id"`
	}{
		Message:    fmt.Sprintf("Instance %d removed successfully", instanceID),
		InstanceID: instanceID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceResizeTool creates a tool for resizing a Linode instance.
func NewLinodeInstanceResizeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_resize",
		"Resizes a Linode instance to a new plan. WARNING: This causes downtime during the migration process and may affect billing.",
		[]mcp.ToolOption{
			mcp.WithNumber("instance_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to resize")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("The new Linode plan type (e.g., 'g6-standard-1')")),
			mcp.WithBoolean("allow_auto_disk",
				mcp.Description("Automatically resize disks when resizing to a larger plan (optional, default: false)")),
			mcp.WithString("migration_type",
				mcp.Description("Migration type: 'cold' (default) or 'warm' (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm resize. This operation causes downtime.")),
		},
		handleLinodeInstanceResizeRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeInstanceResizeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)
	instanceType := request.GetString("type", "")
	allowAutoDisk := request.GetBool("allow_auto_disk", false)
	migrationType := request.GetString("migration_type", "")

	if result := RequireConfirm(request, "This operation causes downtime and may affect billing. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if instanceID == 0 {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	if instanceType == "" {
		return mcp.NewToolResultError("type is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.ResizeInstanceRequest{
		Type:          instanceType,
		AllowAutoDisk: allowAutoDisk,
		MigrationType: migrationType,
	}

	if err := client.ResizeInstance(ctx, instanceID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resize instance %d: %v", instanceID, err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		InstanceID int    `json:"instance_id"`
		NewType    string `json:"new_type"`
	}{
		Message:    fmt.Sprintf("Instance %d resize to %s initiated successfully", instanceID, instanceType),
		InstanceID: instanceID,
		NewType:    instanceType,
	}

	return MarshalToolResponse(response)
}

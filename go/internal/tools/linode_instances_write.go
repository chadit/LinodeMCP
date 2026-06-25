package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
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
			mcp.Description("Must be set to true to confirm booting the instance. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
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

	if IsDryRun(request) {
		if instanceID == 0 {
			return mcp.NewToolResultError("instance_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_"+verb, httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/%s", instanceID, verb),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, instanceID) })
	}

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
			mcp.Description("Must be set to true to confirm rebooting the instance. This causes a brief outage. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
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
			mcp.Description("Must be set to true to confirm shutting down the instance. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceShutdownRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeInstanceShutdownRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	// Routes through the shared power-action flow (verb "shutdown" yields the
	// same "Instance N shutdown initiated successfully" message). The action
	// closure ignores configID since shutdown takes no config profile.
	return handleInstancePowerAction(
		ctx, request, cfg,
		"shutdown",
		"This shuts down a Linode instance. Set confirm=true to proceed.",
		func(ctx context.Context, client *linode.Client, instanceID int, _ *int) error {
			return client.ShutdownInstance(ctx, instanceID)
		},
	)
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
			mcp.Description("Must be set to true to confirm instance creation. This operation incurs billing charges. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

const errFirewallIDRequired = "firewall_id is required for instance creation. Get a firewall ID from linode_firewall_list, or create one with linode_firewall_create."

// validateInstanceCreateArgs validates the instance create args, returning an
// error message or "". Shared by the real create path and the dry-run preview.
func validateInstanceCreateArgs(region, instanceType, rootPass string, firewallID int) string {
	if region == "" {
		return errRegionRequired
	}

	if instanceType == "" {
		return "type is required"
	}

	if firewallID <= 0 {
		return errFirewallIDRequired
	}

	if err := validateRootPassword(rootPass); err != nil {
		return err.Error()
	}

	return ""
}

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

	if IsDryRun(request) {
		if msg := validateInstanceCreateArgs(region, instanceType, rootPass, firewallID); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_create", httpMethodPost, "/linode/instances", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return instanceCreateSideEffects(ctx, instanceType, region, image)
			})
	}

	if result := RequireConfirm(request, "This operation creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateInstanceCreateArgs(region, instanceType, rootPass, firewallID); msg != "" {
		return mcp.NewToolResultError(msg), nil
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

// toolInstanceUpdate is the update tool's name, shared by the constructor and
// the dry-run preview branch.
const toolInstanceUpdate = "linode_instance_update"

// NewLinodeInstanceUpdateTool creates a tool for updating editable fields on a Linode instance.
func NewLinodeInstanceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		toolInstanceUpdate,
		"Updates editable fields on a Linode instance. Pass dry_run=true to preview without updating.",
		[]mcp.ToolOption{
			mcp.WithNumber("instance_id", mcp.Required(),
				mcp.Description("The ID of the instance to update (required)")),
			mcp.WithString("label",
				mcp.Description("New Linode label (optional)")),
			mcp.WithString("group",
				mcp.Description("Deprecated group label (optional)")),
			mcp.WithArray("tags",
				mcp.Description("Tags to assign to the Linode (optional)")),
			mcp.WithObject("alerts",
				mcp.Description("Alert threshold settings (optional)")),
			mcp.WithString("maintenance_policy",
				mcp.Description("Maintenance policy, such as linode/migrate (optional)")),
			mcp.WithBoolean("watchdog_enabled",
				mcp.Description("Whether Lassie shutdown watchdog is enabled (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm the update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeInstanceUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeInstanceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)

	if IsDryRun(request) {
		if instanceID == 0 {
			return mcp.NewToolResultError("instance_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, toolInstanceUpdate, httpMethodPut,
			fmt.Sprintf("/linode/instances/%d", instanceID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, instanceID) })
	}

	if result := RequireConfirm(request, "This updates a Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if instanceID == 0 {
		return mcp.NewToolResultError("instance_id is required"), nil
	}

	req, validationMessage := instanceUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.UpdateInstance(ctx, instanceID, req)
	if err != nil {
		msg := fmt.Sprint("instance ", instanceID, " update failed: ", err)

		return mcp.NewToolResultError(msg), nil
	}

	response := struct {
		Message  string                `json:"message"`
		Instance instanceUpdateSummary `json:"instance"`
	}{
		Message: fmt.Sprintf("Instance %d updated successfully", instance.ID),
		Instance: instanceUpdateSummary{
			ID:              instance.ID,
			Label:           instance.Label,
			Status:          instance.Status,
			Type:            instance.Type,
			Region:          instance.Region,
			Tags:            instance.Tags,
			WatchdogEnabled: instance.WatchdogEnabled,
		},
	}

	return MarshalToolResponse(response)
}

// instanceUpdateSummary is the condensed instance view linode_instance_update
// returns, matching the Python implementation's response shape for this tool.
type instanceUpdateSummary struct {
	ID              int      `json:"id"`
	Label           string   `json:"label"`
	Status          string   `json:"status"`
	Type            string   `json:"type"`
	Region          string   `json:"region"`
	Tags            []string `json:"tags"`
	WatchdogEnabled bool     `json:"watchdog_enabled"`
}

// instanceUpdateRequestFromTool builds the UpdateInstanceRequest from the tool
// args. Returns a validation message when an arg has the wrong shape or no
// updatable field was provided at all.
func instanceUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateInstanceRequest, string) {
	args := request.GetArguments()
	req := &linode.UpdateInstanceRequest{}

	var hasField bool

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
		hasField = true
	}

	if group := request.GetString("group", ""); group != "" {
		req.Group = group
		hasField = true
	}

	if policy := request.GetString("maintenance_policy", ""); policy != "" {
		req.MaintenancePolicy = policy
		hasField = true
	}

	if raw, exists := args["tags"]; exists {
		tags, validationMessage := instanceUpdateTagsFromArg(raw)
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.Tags = tags
		hasField = true
	}

	if raw, exists := args["alerts"]; exists {
		alerts, validationMessage := instanceUpdateAlertsFromArg(raw)
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.Alerts = alerts
		hasField = true
	}

	if raw, exists := args["watchdog_enabled"]; exists {
		enabled, ok := raw.(bool)
		if !ok {
			return nil, "watchdog_enabled must be a boolean"
		}

		req.WatchdogEnabled = &enabled
		hasField = true
	}

	if !hasField {
		return nil, "at least one update field is required: label, group, tags, alerts, maintenance_policy, or watchdog_enabled"
	}

	return req, ""
}

// instanceUpdateTagsFromArg converts the raw tags argument into a string
// slice, rejecting non-array values and non-string entries.
func instanceUpdateTagsFromArg(raw any) ([]string, string) {
	rawList, ok := raw.([]any)
	if !ok {
		return nil, "tags must be an array of strings"
	}

	tags := make([]string, 0, len(rawList))

	for _, item := range rawList {
		tag, ok := item.(string)
		if !ok {
			return nil, "tags entries must be strings"
		}

		tags = append(tags, tag)
	}

	return tags, ""
}

// errAlertsMustBeObject is the validation message every malformed-alerts
// branch below shares.
const errAlertsMustBeObject = "alerts must be an object of alert thresholds"

// instanceUpdateAlertsFromArg decodes the raw alerts object into the typed
// alert-threshold struct via a JSON round trip so field names and numeric
// types validate against the API shape.
func instanceUpdateAlertsFromArg(raw any) (*linode.Alerts, string) {
	if _, ok := raw.(map[string]any); !ok {
		return nil, errAlertsMustBeObject
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, errAlertsMustBeObject
	}

	var alerts linode.Alerts
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, errAlertsMustBeObject
	}

	return &alerts, ""
}

// NewLinodeInstanceDeleteTool creates a tool for deleting a Linode instance.
func NewLinodeInstanceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := newDeleteByIDTool(
		"linode_instance_delete",
		"Deletes a Linode instance. WARNING: This action is irreversible and all data will be permanently lost. Pass dry_run=true to preview without deleting.",
		"instance_id",
		"The ID of the Linode instance to delete",
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLinodeInstanceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_instance_delete",
		IDParam:        "instance_id",
		Method:         httpMethodDelete,
		PathPattern:    "/linode/instances/%d",
		ConfirmMessage: "This operation is destructive and irreversible. Set confirm=true to proceed.",
		SuccessFormat:  "Instance %d removed successfully",
		FetchState:     func(ctx context.Context, c *linode.Client, id int) (any, error) { return c.GetInstance(ctx, id) },
		Execute:        func(ctx context.Context, c *linode.Client, id int) error { return c.DeleteInstance(ctx, id) },
		DependencyWalk: instanceDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("Instance"),
	})
}

// toolInstanceResize is the resize tool's name, referenced by the constructor,
// the two-stage action, and the dry-run preview, so it lives in one place.
const toolInstanceResize = "linode_instance_resize"

// NewLinodeInstanceResizeTool creates a tool for resizing a Linode instance.
func NewLinodeInstanceResizeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		toolInstanceResize,
		"Resizes a Linode instance to a new plan. WARNING: This causes downtime during the migration process and may affect billing."+
			" Pass dry_run=true to preview without resizing."+twoStageOptInNote,
		[]mcp.ToolOption{
			mcp.WithNumber("instance_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to resize")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("The new Linode plan type (e.g., 'g6-standard-1')")),
			mcp.WithBoolean("allow_auto_disk_resize",
				mcp.Description("Automatically resize disks when resizing to a larger plan (optional, default: false)")),
			mcp.WithString("migration_type",
				mcp.Description("Migration type: 'cold' (default) or 'warm' (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm resize. This operation causes downtime. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleLinodeInstanceResizeRequest,
	)

	return tool, profiles.CapWrite, handler
}

// instanceResizeState is the drift-relevant projection a resize plan hashes:
// the instance's current type plus each disk's id, size, and filesystem. A real
// change to any of these between plan and apply refuses the apply. Cosmetic
// instance fields (updated, status, last_seen_ipv4) are excluded by
// construction, so resize needs no hash-ignore list.
type instanceResizeState struct {
	Type  string                   `json:"type"`
	Disks []instanceResizeDiskInfo `json:"disks"`
}

// instanceResizeDiskInfo is one disk's drift-relevant fields. Disk sizes matter
// because allow_auto_disk_resize resizes them as part of the plan change.
type instanceResizeDiskInfo struct {
	ID         int    `json:"id"`
	Size       int    `json:"size"`
	Filesystem string `json:"filesystem"`
}

// fetchInstanceResizeState builds the composite resize projection: the instance
// plus its disks. Resize affects both, so the drift hash must cover both.
func fetchInstanceResizeState(ctx context.Context, client *linode.Client, instanceID int) (any, error) {
	instance, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance for resize plan: %w", err)
	}

	disks, err := client.ListInstanceDisks(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list disks for resize plan: %w", err)
	}

	snapshot := make([]instanceResizeDiskInfo, len(disks))
	for i := range disks {
		snapshot[i] = instanceResizeDiskInfo{
			ID:         disks[i].ID,
			Size:       disks[i].Size,
			Filesystem: disks[i].Filesystem,
		}
	}

	return &instanceResizeState{Type: instance.Type, Disks: snapshot}, nil
}

// resizeStateType reads the from-type out of whichever state shape the resize
// walk receives: the composite projection on the two-stage path, the bare
// instance on the dry-run path.
func resizeStateType(state any) string {
	switch typed := state.(type) {
	case *instanceResizeState:
		if typed != nil {
			return typed.Type
		}
	case *linode.Instance:
		if typed != nil {
			return typed.Type
		}
	}

	return ""
}

// newInstanceResizeAction packages resize as a two-stage action. Capability is
// CapWrite, so the flow stays opt-in: a plan/apply call resizes only when an
// operator enables linode_instance_resize via the two_stage config block.
func newInstanceResizeAction(instanceID int, instanceType string, req linode.ResizeInstanceRequest) *DestructiveAction {
	return &DestructiveAction{
		ToolName:   toolInstanceResize,
		Capability: profiles.CapWrite,
		Method:     httpMethodPost,
		Path:       fmt.Sprintf("/linode/instances/%d/resize", instanceID),
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return fetchInstanceResizeState(ctx, c, instanceID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.ResizeInstance(ctx, instanceID, req)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: fmt.Sprintf("Instance %d resize to %s initiated successfully", instanceID, instanceType),
				"instance_id":      instanceID,
				"new_type":         instanceType,
			}
		},
		DependencyWalk: func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
			return instanceResizeSideEffects(ctx, resizeStateType(state), instanceType)
		},
	}
}

func handleLinodeInstanceResizeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)
	instanceType := request.GetString("type", "")
	allowAutoDiskResize := request.GetBool("allow_auto_disk_resize", false)
	migrationType := request.GetString("migration_type", "")

	req := linode.ResizeInstanceRequest{
		Type:                instanceType,
		AllowAutoDiskResize: allowAutoDiskResize,
		MigrationType:       migrationType,
	}

	if instanceID != 0 && instanceType != "" {
		action := newInstanceResizeAction(instanceID, instanceType, req)
		if result, handled := runTwoStageBranch(ctx, request, cfg, action); handled {
			return result, nil
		}
	}

	if IsDryRun(request) {
		if instanceID == 0 {
			return mcp.NewToolResultError("instance_id is required"), nil
		}

		if instanceType == "" {
			return mcp.NewToolResultError("type is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, toolInstanceResize, httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/resize", instanceID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, instanceID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return instanceResizeSideEffects(ctx, resizeStateType(state), instanceType)
			})
	}

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

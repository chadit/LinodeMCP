package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeInstanceCloneTool creates a tool for cloning a Linode instance.
func NewLinodeInstanceCloneTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_clone",
		"Clones a Linode instance. WARNING: This creates a billable resource.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to clone")),
			mcp.WithString("region",
				mcp.Description("Region for the cloned instance (optional, defaults to same region)")),
			mcp.WithString("type",
				mcp.Description("Instance type for the clone (optional, defaults to same type)")),
			mcp.WithString("label",
				mcp.Description("Label for the cloned instance (optional)")),
			mcp.WithBoolean("backups_enabled",
				mcp.Description("Enable backups on the cloned instance (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm instance cloning. This creates a billable resource. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceCloneRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceCloneRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError("linode_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_clone", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/clone", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This clones a Linode instance and creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	req := &linode.CloneInstanceRequest{}

	if region := request.GetString("region", ""); region != "" {
		req.Region = region
	}

	if instanceType := request.GetString("type", ""); instanceType != "" {
		req.Type = instanceType
	}

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
	}

	if request.GetBool("backups_enabled", false) {
		req.BackupsEnabled = true
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.CloneInstance(ctx, linodeID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to clone instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string           `json:"message"`
		Instance *linode.Instance `json:"instance"`
	}{
		Message:  fmt.Sprintf("Instance %d cloned as '%s' (ID: %d) in %s", linodeID, instance.Label, instance.ID, instance.Region),
		Instance: instance,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceMigrateTool creates a tool for migrating a Linode instance to a new region.
func NewLinodeInstanceMigrateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_migrate",
		"Migrates a Linode instance to a new region. If no region is specified, Linode picks the destination.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to migrate")),
			mcp.WithString("region",
				mcp.Description("Target region for migration (optional, Linode picks if omitted)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm migration. The instance will be shut down during migration. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceMigrateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceMigrateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError("linode_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_migrate", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/migrate", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This migrates the instance and causes downtime during migration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	region := request.GetString("region", "")

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.MigrateInstance(ctx, linodeID, region); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to migrate instance %d: %v", linodeID, err)), nil
	}

	msg := fmt.Sprintf("Migration initiated for instance %d", linodeID)
	if region != "" {
		msg = fmt.Sprintf("Migration initiated for instance %d to region %s", linodeID, region)
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		Region   string `json:"region,omitempty"`
	}{
		Message:  msg,
		LinodeID: linodeID,
		Region:   region,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceMutateTool creates a tool for upgrading a Linode instance to the latest generation type.
func NewLinodeInstanceMutateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_mutate",
		"Upgrades a Linode instance to the latest generation type. WARNING: This changes instance state and may cause downtime.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to upgrade")),
			mcp.WithBoolean("allow_auto_disk_resize",
				mcp.Description("Automatically resize disks during the upgrade when possible (optional, API default: true)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm the instance upgrade. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceMutateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceMutateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID <= 0 {
			return mcp.NewToolResultError("linode_id is required and must be positive"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_mutate", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/mutate", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This upgrades the instance and may cause downtime. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID <= 0 {
		return mcp.NewToolResultError("linode_id is required and must be positive"), nil
	}

	mutateReq := &linode.MutateInstanceRequest{}

	if raw, exists := request.GetArguments()["allow_auto_disk_resize"]; exists {
		allowAutoDiskResize, ok := raw.(bool)
		if !ok {
			return mcp.NewToolResultError("allow_auto_disk_resize must be a boolean"), nil
		}

		mutateReq.AllowAutoDiskResize = &allowAutoDiskResize
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.MutateInstance(ctx, linodeID, mutateReq); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to upgrade instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Upgrade initiated for instance %d", linodeID),
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceRebuildTool creates a tool for rebuilding a Linode instance with a new image.
func NewLinodeInstanceRebuildTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_rebuild",
		"Rebuilds a Linode instance with a new image. WARNING: This destroys all existing data on the instance."+
			" Pass dry_run=true to preview without rebuilding.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to rebuild")),
			mcp.WithString("image", mcp.Required(),
				mcp.Description("The image to rebuild with (e.g. 'linode/ubuntu24.04'). Use linode_image_list to find valid values.")),
			mcp.WithString("root_pass", mcp.Required(),
				mcp.Description("Root password for the rebuilt instance (min 12 chars, must include upper, lower, and digits)")),
			mcp.WithString("authorized_keys",
				mcp.Description("Comma-separated SSH public keys to add to root's authorized_keys (optional)")),
			mcp.WithString("authorized_users",
				mcp.Description("Comma-separated Linode usernames whose SSH keys to add (optional)")),
			mcp.WithBoolean("booted",
				mcp.Description("Whether to boot the instance after rebuild (optional, defaults to true)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm rebuild. WARNING: This destroys ALL existing data. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceRebuildRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceRebuildRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt(paramLinodeID, 0)
	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	image := request.GetString("image", "")
	if image == "" {
		return mcp.NewToolResultError("image is required"), nil
	}

	rootPass := request.GetString("root_pass", "")
	if rootPass == "" {
		return mcp.NewToolResultError("root_pass is required"), nil
	}

	if err := validateRootPassword(rootPass); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := &linode.RebuildInstanceRequest{
		Image:    image,
		RootPass: rootPass,
	}

	if keys := request.GetString("authorized_keys", ""); keys != "" {
		req.AuthorizedKeys = splitCommaSeparated(keys)
	}

	if users := request.GetString("authorized_users", ""); users != "" {
		req.AuthorizedUsers = splitCommaSeparated(users)
	}

	// Only set Booted when the caller explicitly passed the parameter.
	// The MCP schema delivers booleans as false by default, so we check
	// the raw arguments map to distinguish "not provided" from "false".
	if _, ok := request.GetArguments()["booted"]; ok {
		booted := request.GetBool("booted", true)
		req.Booted = &booted
	}

	// Captured by the Execute and Success closures: Execute assigns the
	// rebuilt instance, Success returns it. The destroy helper's Execute
	// returns only an error, so the result is threaded through this var.
	var rebuilt *linode.Instance

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_instance_rebuild",
		Method:         httpMethodPost,
		Path:           fmt.Sprintf("/linode/instances/%d/rebuild", linodeID),
		ConfirmMessage: "This DESTROYS ALL DATA on the instance and rebuilds it. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetInstance(ctx, linodeID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			instance, execErr := c.RebuildInstance(ctx, linodeID, req)
			if execErr != nil {
				return fmt.Errorf("rebuild instance %d: %w", linodeID, execErr)
			}

			rebuilt = instance

			return nil
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: fmt.Sprintf("Instance %d rebuilt with image %s", linodeID, image),
				"instance":         rebuilt,
			}
		},
	})
}

// NewLinodeInstanceRescueTool creates a tool for booting a Linode instance into rescue mode.
func NewLinodeInstanceRescueTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_rescue",
		"Boots a Linode instance into rescue mode for recovery operations.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to boot into rescue mode")),
			mcp.WithString("devices",
				mcp.Description("JSON object mapping device slots to disk/volume IDs, e.g. "+
					"{\"sda\":{\"disk_id\":123},\"sdb\":{\"volume_id\":456}} (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm booting into rescue mode. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceRescueRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceRescueRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError("linode_id is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_rescue", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/rescue", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This reboots the instance into rescue mode. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	req := linode.RescueInstanceRequest{
		Devices: make(map[string]*linode.RescueDeviceAssignment),
	}

	if devicesJSON := request.GetString("devices", ""); devicesJSON != "" {
		if err := json.Unmarshal([]byte(devicesJSON), &req.Devices); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid devices JSON: %v", err)), nil
		}
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.RescueInstance(ctx, linodeID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to rescue instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Instance %d is booting into rescue mode", linodeID),
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstancePasswordResetTool creates a tool for resetting the root password on a Linode instance.
func NewLinodeInstancePasswordResetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_password_reset",
		"Resets the root password on a Linode instance. The instance must be powered off."+
			" Pass dry_run=true to preview without resetting.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithString("root_pass", mcp.Required(),
				mcp.Description("New root password (min 12 chars, must include upper, lower, and digits)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm password reset. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstancePasswordResetRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstancePasswordResetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt(paramLinodeID, 0)
	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	rootPass := request.GetString("root_pass", "")
	if rootPass == "" {
		return mcp.NewToolResultError("root_pass is required"), nil
	}

	if err := validateRootPassword(rootPass); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_instance_password_reset",
		IDParam:        paramLinodeID,
		Method:         httpMethodPost,
		PathPattern:    "/linode/instances/%d/password",
		ConfirmMessage: "This resets the root password on the instance. Set confirm=true to proceed.",
		SuccessFormat:  "Root password reset for instance %d",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.ResetInstancePassword(ctx, id, rootPass)
		},
	})
}

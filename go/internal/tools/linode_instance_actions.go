package tools

import (
	"context"
	"encoding/json"
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

// NewLinodeInstanceCloneTool creates a tool for cloning a Linode instance.
func NewLinodeInstanceCloneTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_clone",
		"Clones a Linode instance. WARNING: This creates a billable resource.",
		toolschemas.Schema("linode.mcp.v1.InstanceCloneInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceCloneRequest(ctx, &request, cfg)
	}

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

	if raw, exists := request.GetArguments()["configs"]; exists {
		configs, validationMessage := intSliceFromToolArg(raw, "configs")
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		req.Configs = configs
	}

	if raw, exists := request.GetArguments()["disks"]; exists {
		disks, validationMessage := intSliceFromToolArg(raw, "disks")
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		req.Disks = disks
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.CloneInstanceProto(ctx, linodeID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to clone instance %d: %v", linodeID, err)), nil
	}

	response := &linodev1.InstanceWriteResponse{
		Message:  fmt.Sprintf("Instance %d cloned as '%s' (ID: %d) in %s", linodeID, instance.GetLabel(), instance.GetId(), instance.GetRegion()),
		Instance: instance,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeInstanceMigrateTool creates a tool for migrating a Linode instance to a new region.
func NewLinodeInstanceMigrateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_migrate",
		"Migrates a Linode instance to a new region. If no region is specified, Linode picks the destination.",
		toolschemas.Schema("linode.mcp.v1.InstanceMigrateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceMigrateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleInstanceMigrateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError("linode_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_migrate", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/migrate", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return instanceMigrateSideEffects(ctx, state, request.GetString("region", ""))
			})
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

	response := &linodev1.InstanceMigrateWriteResponse{
		Message:  msg,
		LinodeId: linodeIDToInt32(linodeID),
	}

	// Echo the target region only when the caller picked one; an omitted
	// region (Linode picks the destination) leaves the field unset so it
	// stays absent from the output, matching the legacy omitempty shape.
	if region != "" {
		response.Message = fmt.Sprintf("Migration initiated for instance %d to region %s", linodeID, region)
		response.Region = &region
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeInstanceMutateTool creates a tool for upgrading a Linode instance to the latest generation type.
func NewLinodeInstanceMutateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_mutate",
		"Upgrades a Linode instance to the latest generation type. WARNING: This changes instance state and may cause downtime.",
		toolschemas.Schema("linode.mcp.v1.InstanceMutateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceMutateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleInstanceMutateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID <= 0 {
			return mcp.NewToolResultError("linode_id is required and must be positive"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_mutate", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/mutate", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return instanceMutateSideEffects(ctx, state)
			})
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

	return MarshalProtoToolResponse(&linodev1.InstanceActionWriteResponse{
		Message:  fmt.Sprintf("Upgrade initiated for instance %d", linodeID),
		LinodeId: linodeIDToInt32(linodeID),
	})
}

// NewLinodeInstanceRebuildTool creates a tool for rebuilding a Linode instance with a new image.
func NewLinodeInstanceRebuildTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_rebuild",
		"Rebuilds a Linode instance with a new image. WARNING: This destroys all existing data on the instance."+
			" Pass dry_run=true to preview without rebuilding."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.InstanceRebuildInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceRebuildRequest(ctx, &request, cfg)
	}

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

	rootPass, validationMessage := stringArgument(request, "root_pass", false)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	var authorizedKeys []string
	if raw, exists := request.GetArguments()["authorized_keys"]; exists {
		authorizedKeys, validationMessage = stringSliceFromToolArg(raw, "authorized_keys")
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}
	}

	var authorizedUsers []string
	if raw, exists := request.GetArguments()["authorized_users"]; exists {
		authorizedUsers, validationMessage = stringSliceFromToolArg(raw, "authorized_users")
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}
	}

	if validationMessage := validateProvisioningAuth(rootPass, authorizedKeys, authorizedUsers); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req := &linode.RebuildInstanceRequest{
		Image:           image,
		RootPass:        rootPass,
		AuthorizedKeys:  authorizedKeys,
		AuthorizedUsers: authorizedUsers,
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
	var rebuilt *linodev1.Instance

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_instance_rebuild",
		Method:         httpMethodPost,
		Path:           fmt.Sprintf("/linode/instances/%d/rebuild", linodeID),
		ConfirmMessage: "This DESTROYS ALL DATA on the instance and rebuilds it. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetInstance(ctx, linodeID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			instance, execErr := c.RebuildInstanceProto(ctx, linodeID, req)
			if execErr != nil {
				return fmt.Errorf("rebuild instance %d: %w", linodeID, execErr)
			}

			rebuilt = instance

			return nil
		},
		// Success returns a proto.Message so marshalDestroySuccess routes it
		// through the proto-canonical marshaller, matching the Python side.
		Success: func() proto.Message {
			return &linodev1.InstanceWriteResponse{
				Message:  fmt.Sprintf("Instance %d rebuilt with image %s", linodeID, image),
				Instance: rebuilt,
			}
		},
		DependencyWalk: instanceRebuildSideEffectsWalk,
		HashIgnore:     twostage.HashIgnoreFields("Instance"),
	})
}

// NewLinodeInstanceRescueTool creates a tool for booting a Linode instance into rescue mode.
func NewLinodeInstanceRescueTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_rescue",
		"Boots a Linode instance into rescue mode for recovery operations.",
		toolschemas.Schema("linode.mcp.v1.InstanceRescueInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceRescueRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleInstanceRescueRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError("linode_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_rescue", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/rescue", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) },
			instanceRescueSideEffectsWalk)
	}

	if result := RequireConfirm(request, "This reboots the instance into rescue mode. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	var req linode.RescueInstanceRequest

	if rawDevices, present := request.GetArguments()["devices"]; present {
		devicesJSON, validationMessage := objectJSONFromToolArg(rawDevices, "devices")
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if devicesJSON != "" {
			if err := json.Unmarshal([]byte(devicesJSON), &req.Devices); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid devices JSON: %v", err)), nil
			}
		}
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.RescueInstance(ctx, linodeID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to rescue instance %d: %v", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceActionWriteResponse{
		Message:  fmt.Sprintf("Instance %d is booting into rescue mode", linodeID),
		LinodeId: linodeIDToInt32(linodeID),
	})
}

// NewLinodeInstancePasswordResetTool creates a tool for resetting the root password on a Linode instance.
func NewLinodeInstancePasswordResetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_password_reset",
		"Resets the root password on a Linode instance. The instance must be powered off."+
			" Pass dry_run=true to preview without resetting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.InstancePasswordResetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstancePasswordResetRequest(ctx, &request, cfg)
	}

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

	// Full DestructiveAction form (rather than the *WithID wrapper) so the
	// Success closure returns a proto message, routing both the single-step and
	// two-stage apply paths through the proto-canonical marshaller to match the
	// Python side.
	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_instance_password_reset",
		Method:         httpMethodPost,
		Path:           fmt.Sprintf("/linode/instances/%d/password", linodeID),
		ConfirmMessage: "This resets the root password on the instance. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetInstance(ctx, linodeID)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.ResetInstancePassword(ctx, linodeID, rootPass)
		},
		Success: func() proto.Message {
			return &linodev1.InstanceActionWriteResponse{
				Message:  fmt.Sprintf("Root password reset for instance %d", linodeID),
				LinodeId: linodeIDToInt32(linodeID),
			}
		},
		DependencyWalk: func(ctx context.Context, c *linode.Client, state any) (DryRunDetails, error) {
			return instancePasswordResetSideEffectsWalk(ctx, c, linodeID, state)
		},
		HashIgnore: twostage.HashIgnoreFields("Instance"),
	})
}

package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeInstanceBackupListTool creates a tool for listing all backups for a Linode instance.
func NewLinodeInstanceBackupListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_backup_list",
		"Lists all backups for a Linode instance, including automatic backups and manual snapshots.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceBackupsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceBackupsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	backups, err := client.ListInstanceBackups(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list backups for instance %d: %v", linodeID, err)), nil
	}

	return MarshalToolResponse(backups)
}

// NewLinodeInstanceBackupGetTool creates a tool for retrieving a specific backup for a Linode instance.
func NewLinodeInstanceBackupGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_backup_get",
		"Retrieves details of a specific backup for a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceBackupGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceBackupGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleInstanceBackupGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	backupID := request.GetInt("backup_id", 0)
	if backupID == 0 {
		return mcp.NewToolResultError(ErrBackupIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	backup, err := client.GetInstanceBackupProto(ctx, linodeID, backupID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve backup %d for instance %d: %v", backupID, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(backup)
}

// NewLinodeInstanceBackupCreateTool creates a tool for taking a manual snapshot of a Linode instance.
func NewLinodeInstanceBackupCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_backup_create",
		"Creates a manual snapshot of a Linode instance. "+
			"WARNING: This overwrites any existing manual snapshot for the instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to snapshot")),
			mcp.WithString("label",
				mcp.Description("Label for the manual snapshot (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm snapshot creation. This overwrites any existing manual snapshot. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceBackupCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceBackupCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_backup_create", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/backups", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This creates a manual snapshot and overwrites any existing one. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	backup, err := client.CreateInstanceBackup(ctx, linodeID, request.GetString("label", ""))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create snapshot for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message string                 `json:"message"`
		Backup  *linode.InstanceBackup `json:"backup"`
	}{
		Message: fmt.Sprintf("Snapshot created for instance %d (backup ID: %d)", linodeID, backup.ID),
		Backup:  backup,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceBackupRestoreTool creates a tool for restoring a backup to a Linode instance.
func NewLinodeInstanceBackupRestoreTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_backup_restore",
		"Restores a backup to a Linode instance. "+
			"WARNING: When overwrite is true, this destroys all disks and configs on the target instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance that owns the backup")),
			mcp.WithNumber("backup_id", mcp.Required(),
				mcp.Description("The ID of the backup to restore")),
			mcp.WithNumber("target_linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to restore the backup to")),
			mcp.WithBoolean("overwrite",
				mcp.Description("If true, deletes all disks and configs on the target before restoring. Defaults to false.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm restore. With overwrite=true, all existing data on the target is destroyed. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceBackupRestoreRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateBackupRestoreArgs validates the restore IDs, returning an error
// message or "". Shared by the real restore path and the dry-run preview.
func validateBackupRestoreArgs(linodeID, backupID, targetLinodeID int) string {
	if linodeID == 0 {
		return ErrLinodeIDRequired.Error()
	}

	if backupID == 0 {
		return ErrBackupIDRequired.Error()
	}

	if targetLinodeID == 0 {
		return "target_linode_id is required"
	}

	return ""
}

func handleInstanceBackupRestoreRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	overwrite := request.GetBool("overwrite", false)
	linodeID := request.GetInt("linode_id", 0)
	backupID := request.GetInt("backup_id", 0)
	targetLinodeID := request.GetInt("target_linode_id", 0)

	if IsDryRun(request) {
		if msg := validateBackupRestoreArgs(linodeID, backupID, targetLinodeID); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_backup_restore", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/backups/%d/restore", linodeID, backupID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceBackup(ctx, linodeID, backupID)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return backupRestoreSideEffects(ctx, targetLinodeID, overwrite)
			})
	}

	confirmMsg := "This restores a backup to the target instance. Set confirm=true to proceed."
	if overwrite {
		confirmMsg = "This restores a backup and DESTROYS all existing disks and configs on the target instance. Set confirm=true to proceed."
	}

	if result := RequireConfirm(request, confirmMsg); result != nil {
		return result, nil
	}

	if msg := validateBackupRestoreArgs(linodeID, backupID, targetLinodeID); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.RestoreBackupRequest{
		LinodeID:  targetLinodeID,
		Overwrite: overwrite,
	}

	if err := client.RestoreInstanceBackup(ctx, linodeID, backupID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to restore backup %d to instance %d: %v", backupID, targetLinodeID, err)), nil
	}

	response := struct {
		Message        string `json:"message"`
		BackupID       int    `json:"backup_id"`
		TargetLinodeID int    `json:"target_linode_id"`
		Overwrite      bool   `json:"overwrite"`
	}{
		Message:        fmt.Sprintf("Backup %d restore initiated to instance %d (overwrite=%t)", backupID, targetLinodeID, overwrite),
		BackupID:       backupID,
		TargetLinodeID: targetLinodeID,
		Overwrite:      overwrite,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceBackupsEnableTool creates a tool for enabling the backup service on a Linode instance.
func NewLinodeInstanceBackupsEnableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_backups_enable",
		"Enables the backup service for a Linode instance. "+
			"WARNING: This adds a recurring charge to your account based on the instance plan.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm enabling backups. This adds a recurring billing charge. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceBackupsEnableRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceBackupsEnableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_backups_enable", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/backups/enable", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "Enabling backups adds a recurring charge to your account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.EnableInstanceBackups(ctx, linodeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to enable backups for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Backup service enabled for instance %d", linodeID),
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceFirewallsApplyTool creates a tool for reapplying assigned firewalls to a Linode instance.
func NewLinodeInstanceFirewallsApplyTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_firewall_apply",
		"Reapplies assigned firewalls to a Linode instance. Use this if firewall assignment was not applied successfully.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm reapplying firewalls to this Linode. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceFirewallsApplyRequest,
	)

	return tool, profiles.CapWrite, handler
}

// firewallsApplyLinodeIDFromTool validates and parses the linode_id, returning
// the ID or an error message. Shared by the real path and the dry-run preview.
func firewallsApplyLinodeIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["linode_id"]; !exists {
		return 0, ErrLinodeIDRequired.Error()
	}

	return optionalPaginationInt(args, "linode_id", 1, 0)
}

func handleInstanceFirewallsApplyRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		linodeID, validationMessage := firewallsApplyLinodeIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_firewall_apply", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/firewalls/apply", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This reapplies assigned firewalls to the Linode. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, validationMessage := firewallsApplyLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.ApplyInstanceFirewalls(ctx, linodeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to apply firewalls for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Firewall apply initiated for instance %d", linodeID),
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceBackupsCancelTool creates a tool for canceling the backup service on a Linode instance.
func NewLinodeInstanceBackupsCancelTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_backups_cancel",
		"Cancels the backup service for a Linode instance. "+
			"WARNING: This permanently deletes all existing backups for the instance."+
			" Pass dry_run=true to preview without canceling."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm cancellation. All existing backups will be permanently deleted. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleInstanceBackupsCancelRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceBackupsCancelRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_instance_backups_cancel",
		IDParam:        paramLinodeID,
		Method:         httpMethodPost,
		PathPattern:    "/linode/instances/%d/backups/cancel",
		ConfirmMessage: "This permanently deletes all backups for the instance. Set confirm=true to proceed.",
		SuccessFormat:  "Backup service canceled for instance %d. All backups have been deleted.",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.CancelInstanceBackups(ctx, id)
		},
		// The plan fetches the parent instance, so reuse the Instance
		// hash-ignore list to drop its self-moving telemetry fields.
		HashIgnore: twostage.HashIgnoreFields("Instance"),
	})
}

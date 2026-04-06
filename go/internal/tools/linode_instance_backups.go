package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeInstanceBackupsListTool creates a tool for listing all backups for a Linode instance.
func NewLinodeInstanceBackupsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_backups_list",
		"Lists all backups for a Linode instance, including automatic backups and manual snapshots.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceBackupsListRequest,
	)
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
func NewLinodeInstanceBackupGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_backup_get",
		"Retrieves details of a specific backup for a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("backup_id", mcp.Required(),
				mcp.Description("The ID of the backup to retrieve")),
		},
		handleInstanceBackupGetRequest,
	)
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

	backup, err := client.GetInstanceBackup(ctx, linodeID, backupID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve backup %d for instance %d: %v", backupID, linodeID, err)), nil
	}

	return MarshalToolResponse(backup)
}

// NewLinodeInstanceBackupCreateTool creates a tool for taking a manual snapshot of a Linode instance.
func NewLinodeInstanceBackupCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_backup_create",
		"Creates a manual snapshot of a Linode instance. "+
			"WARNING: This overwrites any existing manual snapshot for the instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance to snapshot")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm snapshot creation. This overwrites any existing manual snapshot.")),
		},
		handleInstanceBackupCreateRequest,
	)
}

func handleInstanceBackupCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a manual snapshot and overwrites any existing one. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	backup, err := client.CreateInstanceBackup(ctx, linodeID)
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
func NewLinodeInstanceBackupRestoreTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
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
				mcp.Description("Must be true to confirm restore. With overwrite=true, all existing data on the target is destroyed.")),
		},
		handleInstanceBackupRestoreRequest,
	)
}

func handleInstanceBackupRestoreRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	overwrite := request.GetBool("overwrite", false)

	confirmMsg := "This restores a backup to the target instance. Set confirm=true to proceed."
	if overwrite {
		confirmMsg = "This restores a backup and DESTROYS all existing disks and configs on the target instance. Set confirm=true to proceed."
	}

	if result := RequireConfirm(request, confirmMsg); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	backupID := request.GetInt("backup_id", 0)
	if backupID == 0 {
		return mcp.NewToolResultError(ErrBackupIDRequired.Error()), nil
	}

	targetLinodeID := request.GetInt("target_linode_id", 0)
	if targetLinodeID == 0 {
		return mcp.NewToolResultError("target_linode_id is required"), nil
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
func NewLinodeInstanceBackupsEnableTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_backups_enable",
		"Enables the backup service for a Linode instance. "+
			"WARNING: This adds a recurring charge to your account based on the instance plan.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm enabling backups. This adds a recurring billing charge.")),
		},
		handleInstanceBackupsEnableRequest,
	)
}

func handleInstanceBackupsEnableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "Enabling backups adds a recurring charge to your account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
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

// NewLinodeInstanceBackupsCancelTool creates a tool for canceling the backup service on a Linode instance.
func NewLinodeInstanceBackupsCancelTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_backups_cancel",
		"Cancels the backup service for a Linode instance. "+
			"WARNING: This permanently deletes all existing backups for the instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm cancellation. All existing backups will be permanently deleted.")),
		},
		handleInstanceBackupsCancelRequest,
	)
}

func handleInstanceBackupsCancelRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This permanently deletes all backups for the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.CancelInstanceBackups(ctx, linodeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to cancel backups for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Backup service canceled for instance %d. All backups have been deleted.", linodeID),
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
}

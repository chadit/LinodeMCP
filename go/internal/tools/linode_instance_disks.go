package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeInstanceDiskListTool creates a tool for listing all disks on a Linode instance.
func NewLinodeInstanceDiskListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresource(
		cfg,
		"linode_instance_disk_list",
		"Lists all disks attached to a Linode instance.",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: parseInstanceDiskListPathID,
		},
		func(ctx context.Context, client *linode.Client, linodeID int) ([]*linodev1.InstanceDisk, error) {
			return client.ListInstanceDisksProto(ctx, linodeID)
		},
		nil,
		instanceDiskListResponse,
	)

	return tool, profiles.CapRead, handler
}

// parseInstanceDiskListPathID validates the linode_id path param the same way the
// non-proto handler did (a missing or zero id returns ErrLinodeIDRequired).
func parseInstanceDiskListPathID(request *mcp.CallToolRequest) (int, string) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return 0, ErrLinodeIDRequired.Error()
	}

	return linodeID, ""
}

func instanceDiskListResponse(items []*linodev1.InstanceDisk, count int32, filter *string) *linodev1.InstanceDiskListResponse {
	return &linodev1.InstanceDiskListResponse{Count: count, Filter: filter, Disks: items}
}

// NewLinodeInstanceDiskGetTool creates a tool for retrieving a specific disk on a Linode instance.
func NewLinodeInstanceDiskGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_get",
		"Retrieves details of a specific disk on a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleInstanceDiskGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	diskID := request.GetInt("disk_id", 0)
	if diskID == 0 {
		return mcp.NewToolResultError(ErrDiskIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	disk, err := client.GetInstanceDiskProto(ctx, linodeID, diskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve disk %d for instance %d: %v", diskID, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(disk)
}

// NewLinodeInstanceDiskCreateTool creates a tool for adding a new disk to a Linode instance.
func NewLinodeInstanceDiskCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_disk_create",
		"Creates a new disk on a Linode instance. WARNING: This modifies the instance's storage allocation.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the disk")),
			mcp.WithNumber("size", mcp.Required(),
				mcp.Description("Size of the disk in MB")),
			mcp.WithString("filesystem",
				mcp.Description("Filesystem type: raw, swap, ext3, ext4, initrd (defaults to ext4)")),
			mcp.WithString("image",
				mcp.Description("Image ID to deploy to the disk (e.g. linode/ubuntu22.04)")),
			mcp.WithString("root_pass",
				mcp.Description("Root password for the disk (required when deploying an image, min 12 chars with upper/lower/digits)")),
			mcp.WithString("authorized_keys",
				mcp.Description("Comma-separated list of SSH public keys to install")),
			mcp.WithString("authorized_users",
				mcp.Description("Comma-separated list of Linode usernames whose SSH keys to install")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disk creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceDiskCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateDiskCreateArgs validates the disk create args, returning an error
// message or "". Shared by the real create path and the dry-run preview.
func validateDiskCreateArgs(linodeID int, label string, size int) string {
	if linodeID == 0 {
		return ErrLinodeIDRequired.Error()
	}

	if label == "" {
		return errLabelRequired
	}

	if size == 0 {
		return "size is required and must be greater than 0"
	}

	return ""
}

func handleInstanceDiskCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	label := request.GetString("label", "")
	size := request.GetInt("size", 0)

	if IsDryRun(request) {
		if msg := validateDiskCreateArgs(linodeID, label, size); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_disk_create", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/disks", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) },
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return instanceDiskCreateSideEffects(ctx, label, size, linodeID)
			})
	}

	if result := RequireConfirm(request, "This creates a new disk on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateDiskCreateArgs(linodeID, label, size); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	req := linode.CreateDiskRequest{
		Label: label,
		Size:  size,
	}

	if fs := request.GetString("filesystem", ""); fs != "" {
		req.Filesystem = fs
	}

	if image := request.GetString("image", ""); image != "" {
		req.Image = image
	}

	if rootPass := request.GetString("root_pass", ""); rootPass != "" {
		if err := validateRootPassword(rootPass); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		req.RootPass = rootPass
	}

	if keysStr := request.GetString("authorized_keys", ""); keysStr != "" {
		req.AuthorizedKeys = splitCommaSeparated(keysStr)
	}

	if usersStr := request.GetString("authorized_users", ""); usersStr != "" {
		req.AuthorizedUsers = splitCommaSeparated(usersStr)
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	disk, err := client.CreateInstanceDiskProto(ctx, linodeID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create disk for instance %d: %v", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceDiskWriteResponse{
		Message: fmt.Sprintf("Disk '%s' (ID: %d) created on instance %d", disk.GetLabel(), disk.GetId(), linodeID),
		Disk:    disk,
	})
}

// NewLinodeInstanceDiskUpdateTool creates a tool for updating a disk's label on a Linode instance.
func NewLinodeInstanceDiskUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_disk_update",
		"Updates the label of a disk on a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to update")),
			mcp.WithString("label",
				mcp.Description("New label for the disk")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disk update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceDiskUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateInstanceDiskIDs validates the linode_id + disk_id pair, returning an
// error message or "". Shared by the disk update/clone/resize/password-reset
// real paths and their dry-run previews.
func validateInstanceDiskIDs(linodeID, diskID int) string {
	if linodeID == 0 {
		return ErrLinodeIDRequired.Error()
	}

	if diskID == 0 {
		return ErrDiskIDRequired.Error()
	}

	return ""
}

func handleInstanceDiskUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	diskID := request.GetInt("disk_id", 0)

	if IsDryRun(request) {
		if msg := validateInstanceDiskIDs(linodeID, diskID); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_disk_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/disks/%d", linodeID, diskID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceDisk(ctx, linodeID, diskID)
			})
	}

	if result := RequireConfirm(request, "This modifies the disk configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateInstanceDiskIDs(linodeID, diskID); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	req := linode.UpdateDiskRequest{}

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	disk, err := client.UpdateInstanceDiskProto(ctx, linodeID, diskID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify disk %d on instance %d: %v", diskID, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceDiskWriteResponse{
		Message: fmt.Sprintf("Disk %d on instance %d modified successfully", diskID, linodeID),
		Disk:    disk,
	})
}

// NewLinodeInstanceDiskDeleteTool creates a tool for deleting a disk from a Linode instance.
func NewLinodeInstanceDiskDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_disk_delete",
		"Deletes a disk from a Linode instance. WARNING: This is irreversible and all data on the disk will be lost."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deletion. This action is irreversible and all disk data will be lost. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleInstanceDiskDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceDiskDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	// The two-int helper emits "linode_id is required" / "disk_id is
	// required", which matches ErrLinodeIDRequired / ErrDiskIDRequired
	// verbatim, so no per-tool pre-validation guard is needed here.
	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_instance_disk_delete",
		OuterIDParam:   paramLinodeID,
		InnerIDParam:   "disk_id",
		Method:         httpMethodDelete,
		PathPattern:    "/linode/instances/%d/disks/%d",
		ConfirmMessage: "This is irreversible. All data on the disk will be permanently deleted. Set confirm=true to proceed.",
		SuccessFormat:  "Disk %d deleted from instance %d successfully",
		FetchState: func(ctx context.Context, c *linode.Client, linodeID, diskID int) (any, error) {
			return c.GetInstanceDisk(ctx, linodeID, diskID)
		},
		Execute: func(ctx context.Context, c *linode.Client, linodeID, diskID int) error {
			return c.DeleteInstanceDisk(ctx, linodeID, diskID)
		},
		DependencyWalk: instanceDiskDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("Disk"),
	})
}

// NewLinodeInstanceDiskCloneTool creates a tool for cloning a disk on a Linode instance.
func NewLinodeInstanceDiskCloneTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_disk_clone",
		"Clones a disk on a Linode instance. The instance must have enough unallocated storage for the clone.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to clone")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disk clone. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceDiskCloneRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceDiskCloneRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	diskID := request.GetInt("disk_id", 0)

	if IsDryRun(request) {
		if msg := validateInstanceDiskIDs(linodeID, diskID); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_disk_clone", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/disks/%d/clone", linodeID, diskID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceDisk(ctx, linodeID, diskID)
			},
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return instanceDiskCloneSideEffects(ctx, state)
			})
	}

	if result := RequireConfirm(request, "This clones a disk, consuming additional storage on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateInstanceDiskIDs(linodeID, diskID); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	clonedDisk, err := client.CloneInstanceDiskProto(ctx, linodeID, diskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to clone disk %d on instance %d: %v", diskID, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceDiskWriteResponse{
		Message: fmt.Sprintf("Disk %d cloned to new disk %d on instance %d", diskID, clonedDisk.GetId(), linodeID),
		Disk:    clonedDisk,
	})
}

// NewLinodeInstanceDiskResizeTool creates a tool for resizing a disk on a Linode instance.
func NewLinodeInstanceDiskResizeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_disk_resize",
		"Resizes a disk on a Linode instance. The instance must be powered off to resize. "+
			"Growing a disk requires sufficient unallocated storage. Shrinking requires the data to fit in the new size.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to resize")),
			mcp.WithNumber("size", mcp.Required(),
				mcp.Description("New size for the disk in MB")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disk resize. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceDiskResizeRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceDiskResizeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	diskID := request.GetInt("disk_id", 0)
	size := request.GetInt("size", 0)

	if IsDryRun(request) {
		if msg := validateInstanceDiskIDs(linodeID, diskID); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		if size == 0 {
			return mcp.NewToolResultError("size is required and must be greater than 0"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_disk_resize", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/disks/%d/resize", linodeID, diskID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceDisk(ctx, linodeID, diskID)
			},
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return instanceDiskResizeSideEffects(ctx, state, size)
			})
	}

	if result := RequireConfirm(request, "This resizes the disk. The instance must be powered off. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateInstanceDiskIDs(linodeID, diskID); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	if size == 0 {
		return mcp.NewToolResultError("size is required and must be greater than 0"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.ResizeDiskRequest{Size: size}

	if err := client.ResizeInstanceDisk(ctx, linodeID, diskID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resize disk %d on instance %d: %v", diskID, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceDiskResizeWriteResponse{
		Message:   fmt.Sprintf("Disk %d on instance %d resize initiated to %d MB", diskID, linodeID, size),
		LinodeId:  linodeIDToInt32(linodeID),
		DiskId:    linodeIDToInt32(diskID),
		NewSizeMb: linodeIDToInt32(size),
	})
}

// NewLinodeInstanceDiskPasswordResetTool creates a tool for resetting a disk root password.
func NewLinodeInstanceDiskPasswordResetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_disk_password_reset",
		"Resets the root password for a disk on a Linode instance. The instance must be powered off.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk")),
			mcp.WithString("password", mcp.Required(),
				mcp.Description("New disk root password (min 12 chars, must include upper, lower, and digits)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disk password reset.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceDiskPasswordResetRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceDiskPasswordResetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	diskID := request.GetInt("disk_id", 0)
	if diskID == 0 {
		return mcp.NewToolResultError(ErrDiskIDRequired.Error()), nil
	}

	// Preview fetches the disk metadata, never the secret. The reset payload
	// itself is write-only, so there is nothing sensitive to surface.
	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_instance_disk_password_reset", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/disks/%d/password", linodeID, diskID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceDisk(ctx, linodeID, diskID)
			})
	}

	if result := RequireConfirm(request, "This resets the root password for a disk. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	password := request.GetString("password", "")
	if password == "" {
		return mcp.NewToolResultError("password is required"), nil
	}

	if err := validateRootPassword(password); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.ResetInstanceDiskPassword(ctx, linodeID, diskID, password); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to reset password for disk %d on instance %d: %v", diskID, linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		DiskID   int    `json:"disk_id"`
	}{
		Message:  fmt.Sprintf("Password reset for disk %d on instance %d", diskID, linodeID),
		LinodeID: linodeID,
		DiskID:   diskID,
	}

	return MarshalToolResponse(response)
}

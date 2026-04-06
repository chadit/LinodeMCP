package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeInstanceDisksListTool creates a tool for listing all disks on a Linode instance.
func NewLinodeInstanceDisksListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_disks_list",
		"Lists all disks attached to a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceDisksListRequest,
	)
}

func handleInstanceDisksListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	disks, err := client.ListInstanceDisks(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list disks for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count int                   `json:"count"`
		Disks []linode.InstanceDisk `json:"disks"`
	}{
		Count: len(disks),
		Disks: disks,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceDiskGetTool creates a tool for retrieving a specific disk on a Linode instance.
func NewLinodeInstanceDiskGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_disk_get",
		"Retrieves details of a specific disk on a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to retrieve")),
		},
		handleInstanceDiskGetRequest,
	)
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

	disk, err := client.GetInstanceDisk(ctx, linodeID, diskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve disk %d for instance %d: %v", diskID, linodeID, err)), nil
	}

	return MarshalToolResponse(disk)
}

// NewLinodeInstanceDiskCreateTool creates a tool for adding a new disk to a Linode instance.
func NewLinodeInstanceDiskCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
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
				mcp.Description("Must be true to confirm disk creation.")),
		},
		handleInstanceDiskCreateRequest,
	)
}

func handleInstanceDiskCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a new disk on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	label := request.GetString("label", "")
	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	size := request.GetInt("size", 0)
	if size == 0 {
		return mcp.NewToolResultError("size is required and must be greater than 0"), nil
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

	disk, err := client.CreateInstanceDisk(ctx, linodeID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create disk for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message string               `json:"message"`
		Disk    *linode.InstanceDisk `json:"disk"`
	}{
		Message: fmt.Sprintf("Disk '%s' (ID: %d) created on instance %d", disk.Label, disk.ID, linodeID),
		Disk:    disk,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceDiskUpdateTool creates a tool for updating a disk's label on a Linode instance.
func NewLinodeInstanceDiskUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
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
				mcp.Description("Must be true to confirm disk update.")),
		},
		handleInstanceDiskUpdateRequest,
	)
}

func handleInstanceDiskUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This modifies the disk configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	diskID := request.GetInt("disk_id", 0)
	if diskID == 0 {
		return mcp.NewToolResultError(ErrDiskIDRequired.Error()), nil
	}

	req := linode.UpdateDiskRequest{}

	if label := request.GetString("label", ""); label != "" {
		req.Label = label
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	disk, err := client.UpdateInstanceDisk(ctx, linodeID, diskID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify disk %d on instance %d: %v", diskID, linodeID, err)), nil
	}

	response := struct {
		Message string               `json:"message"`
		Disk    *linode.InstanceDisk `json:"disk"`
	}{
		Message: fmt.Sprintf("Disk %d on instance %d modified successfully", diskID, linodeID),
		Disk:    disk,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceDiskDeleteTool creates a tool for deleting a disk from a Linode instance.
func NewLinodeInstanceDiskDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_disk_delete",
		"Deletes a disk from a Linode instance. WARNING: This is irreversible and all data on the disk will be lost.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deletion. This action is irreversible and all disk data will be lost.")),
		},
		handleInstanceDiskDeleteRequest,
	)
}

func handleInstanceDiskDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This is irreversible. All data on the disk will be permanently deleted. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

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

	if err := client.DeleteInstanceDisk(ctx, linodeID, diskID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove disk %d from instance %d: %v", diskID, linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		DiskID   int    `json:"disk_id"`
	}{
		Message:  fmt.Sprintf("Disk %d deleted from instance %d successfully", diskID, linodeID),
		LinodeID: linodeID,
		DiskID:   diskID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceDiskCloneTool creates a tool for cloning a disk on a Linode instance.
func NewLinodeInstanceDiskCloneTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_disk_clone",
		"Clones a disk on a Linode instance. The instance must have enough unallocated storage for the clone.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("disk_id", mcp.Required(),
				mcp.Description("The ID of the disk to clone")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disk clone.")),
		},
		handleInstanceDiskCloneRequest,
	)
}

func handleInstanceDiskCloneRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This clones a disk, consuming additional storage on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

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

	clonedDisk, err := client.CloneInstanceDisk(ctx, linodeID, diskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to clone disk %d on instance %d: %v", diskID, linodeID, err)), nil
	}

	response := struct {
		Message string               `json:"message"`
		Disk    *linode.InstanceDisk `json:"disk"`
	}{
		Message: fmt.Sprintf("Disk %d cloned to new disk %d on instance %d", diskID, clonedDisk.ID, linodeID),
		Disk:    clonedDisk,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceDiskResizeTool creates a tool for resizing a disk on a Linode instance.
func NewLinodeInstanceDiskResizeTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
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
				mcp.Description("Must be true to confirm disk resize.")),
		},
		handleInstanceDiskResizeRequest,
	)
}

func handleInstanceDiskResizeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This resizes the disk. The instance must be powered off. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	diskID := request.GetInt("disk_id", 0)
	if diskID == 0 {
		return mcp.NewToolResultError(ErrDiskIDRequired.Error()), nil
	}

	size := request.GetInt("size", 0)
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

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		DiskID   int    `json:"disk_id"`
		NewSize  int    `json:"new_size_mb"`
	}{
		Message:  fmt.Sprintf("Disk %d on instance %d resize initiated to %d MB", diskID, linodeID, size),
		LinodeID: linodeID,
		DiskID:   diskID,
		NewSize:  size,
	}

	return MarshalToolResponse(response)
}

package tools

import (
	"context"
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

// NewLinodeInstanceDiskListTool creates a tool for listing all disks on a Linode instance.
func NewLinodeInstanceDiskListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresource(
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

	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_list",
		"Lists all disks attached to a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskListInput"),
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_create",
		"Creates a new disk on a Linode instance. WARNING: This modifies the instance's storage allocation.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateDiskCreateArgs validates the disk create args, returning an error
// message or "". Shared by the real create path and the dry-run preview.
func validateDiskCreateArgs(linodeID int, label string, size int, rootPass string, authorizedKeys, authorizedUsers []string) string {
	if linodeID == 0 {
		return ErrLinodeIDRequired.Error()
	}

	if label == "" {
		return errLabelRequired
	}

	if size == 0 {
		return "size is required and must be greater than 0"
	}

	return validateProvisioningAuth(rootPass, authorizedKeys, authorizedUsers)
}

func handleInstanceDiskCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	label := request.GetString("label", "")
	size := request.GetInt("size", 0)
	image := request.GetString("image", "")

	rootPass, validationMessage := stringArgument(request, "root_pass", false)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	authorizedKeysRaw, validationMessage := stringArgument(request, "authorized_keys", false)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	authorizedUsersRaw, validationMessage := stringArgument(request, "authorized_users", false)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	authorizedKeys := splitCommaSeparated(authorizedKeysRaw)
	authorizedUsers := splitCommaSeparated(authorizedUsersRaw)

	if IsDryRun(request) {
		if msg := validateDiskCreateArgs(linodeID, label, size, rootPass, authorizedKeys, authorizedUsers); msg != "" {
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

	if msg := validateDiskCreateArgs(linodeID, label, size, rootPass, authorizedKeys, authorizedUsers); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	req := linode.CreateDiskRequest{
		Label:           label,
		Size:            size,
		Image:           image,
		RootPass:        rootPass,
		AuthorizedKeys:  authorizedKeys,
		AuthorizedUsers: authorizedUsers,
	}

	if fs := request.GetString("filesystem", ""); fs != "" {
		req.Filesystem = fs
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_update",
		"Updates the label of a disk on a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskUpdateRequest(ctx, &request, cfg)
	}

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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_delete",
		"Deletes a disk from a Linode instance. WARNING: This is irreversible and all data on the disk will be lost."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.InstanceDiskDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// instanceDiskDeleteProto builds the proto-canonical id-echo body for a
// successful disk delete, keeping the proto literal off the handler's struct
// literal so the delete handlers stay below the dupl threshold.
func instanceDiskDeleteProto(linodeID, diskID int) proto.Message {
	return &linodev1.InstanceDiskDeleteResponse{
		Message:  fmt.Sprintf("Disk %d deleted from instance %d successfully", diskID, linodeID),
		LinodeId: linodeIDToInt32(linodeID),
		DiskId:   linodeIDToInt32(diskID),
	}
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
		SuccessProto:   instanceDiskDeleteProto,
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_clone",
		"Clones a disk on a Linode instance. The instance must have enough unallocated storage for the clone.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskCloneInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskCloneRequest(ctx, &request, cfg)
	}

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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_resize",
		"Resizes a disk on a Linode instance. The instance must be powered off to resize. "+
			"Growing a disk requires sufficient unallocated storage. Shrinking requires the data to fit in the new size.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskResizeInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskResizeRequest(ctx, &request, cfg)
	}

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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_disk_password_reset",
		"Resets the root password for a disk on a Linode instance. The instance must be powered off.",
		toolschemas.Schema("linode.mcp.v1.InstanceDiskPasswordResetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceDiskPasswordResetRequest(ctx, &request, cfg)
	}

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

	return MarshalProtoToolResponse(&linodev1.InstanceDiskActionResponse{
		Message:  fmt.Sprintf("Password reset for disk %d on instance %d", diskID, linodeID),
		LinodeId: linodeIDToInt32(linodeID),
		DiskId:   linodeIDToInt32(diskID),
	})
}

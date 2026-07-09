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

// NewLinodeVolumeCreateTool creates a tool for creating a volume.
func NewLinodeVolumeCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_create",
		"Creates a new block storage volume. WARNING: Billing starts immediately. Use linode_region_list to find valid regions.",
		toolschemas.Schema("linode.mcp.v1.VolumeCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateVolumeCreateArgs validates the create args, returning an error
// message or "". Shared by the real create path and the dry-run preview.
func validateVolumeCreateArgs(label, region string, size, linodeID int) string {
	if label == "" {
		return errLabelRequired
	}

	if region == "" && linodeID == 0 {
		return "either region or linode_id is required"
	}

	if size > 0 {
		if err := validateVolumeSize(size); err != nil {
			return err.Error()
		}
	}

	return ""
}

func volumeCloneIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredIDArgument(request, "volume_id")
}

func validateVolumeCloneLabel(label string) string {
	if label == "" {
		return errLabelRequired
	}

	return ""
}

func handleLinodeVolumeCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label := request.GetString("label", "")
	region := request.GetString("region", "")
	size := request.GetInt("size", 0)
	linodeID := request.GetInt("linode_id", 0)

	if IsDryRun(request) {
		if msg := validateVolumeCreateArgs(label, region, size, linodeID); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_volume_create", httpMethodPost, "/volumes", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return volumeCreateSideEffects(ctx, label, region, size, linodeID)
			})
	}

	if result := RequireConfirm(request, "This operation creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateVolumeCreateArgs(label, region, size, linodeID); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateVolumeRequest{
		Label:  label,
		Region: region,
		Size:   size,
	}

	if linodeID != 0 {
		req.LinodeID = &linodeID
	}

	volume, err := client.CreateVolumeProto(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create volume: %v", err)), nil
	}

	response := &linodev1.VolumeWriteResponse{
		Message: fmt.Sprintf("Volume '%s' (ID: %d) created successfully in %s", volume.GetLabel(), volume.GetId(), volume.GetRegion()),
		Volume:  volume,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVolumeCloneTool creates a tool for cloning a volume.
func NewLinodeVolumeCloneTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_clone",
		"Clones an existing block storage volume. WARNING: Billing starts immediately for the cloned volume. Pass dry_run=true to preview without cloning.",
		toolschemas.Schema("linode.mcp.v1.VolumeCloneInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeCloneRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeVolumeCloneRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	volumeID, msg := volumeCloneIDFromTool(request)
	if msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	label := request.GetString("label", "")
	if msg := validateVolumeCloneLabel(label); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_volume_clone", httpMethodPost,
			fmt.Sprintf("/volumes/%d/clone", volumeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVolume(ctx, volumeID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return volumeCloneSideEffects(ctx, state, label)
			})
	}

	if result := RequireConfirm(request, "This operation creates a billable cloned volume. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	volume, err := client.CloneVolumeProto(ctx, volumeID, linode.CloneVolumeRequest{Label: label})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to clone volume %d: %v", volumeID, err)), nil
	}

	response := &linodev1.VolumeWriteResponse{
		Message: fmt.Sprintf("Volume %d cloned successfully as %q", volumeID, volume.GetLabel()),
		Volume:  volume,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVolumeAttachTool creates a tool for attaching a volume to a Linode.
func NewLinodeVolumeAttachTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_attach",
		"Attaches a block storage volume to a Linode instance. The volume and instance must be in the same region.",
		toolschemas.Schema("linode.mcp.v1.VolumeAttachInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeAttachRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeVolumeAttachRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	linodeID := request.GetInt("linode_id", 0)
	configID := request.GetInt("config_id", 0)

	if IsDryRun(request) {
		if volumeID == 0 {
			return mcp.NewToolResultError("volume_id is required"), nil
		}

		if linodeID == 0 {
			return mcp.NewToolResultError("linode_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_volume_attach", httpMethodPost,
			fmt.Sprintf("/volumes/%d/attach", volumeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVolume(ctx, volumeID) },
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return volumeAttachSideEffects(ctx, volumeID, linodeID)
			})
	}

	if result := RequireConfirm(request, "This attaches a block storage volume to an instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.AttachVolumeRequest{
		LinodeID:           linodeID,
		PersistAcrossBoots: request.GetBool("persist_across_boots", false),
	}

	if configID != 0 {
		req.ConfigID = &configID
	}

	volume, err := client.AttachVolumeProto(ctx, volumeID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to attach volume %d to Linode %d: %v", volumeID, linodeID, err)), nil
	}

	response := &linodev1.VolumeWriteResponse{
		Message: fmt.Sprintf("Volume %d attached to Linode %d successfully", volumeID, linodeID),
		Volume:  volume,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVolumeDetachTool creates a tool for detaching a volume from a Linode.
func NewLinodeVolumeDetachTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_detach",
		"Detaches a block storage volume from a Linode instance. The volume data is preserved. Pass dry_run=true to preview without detaching.",
		toolschemas.Schema("linode.mcp.v1.VolumeDetachInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeDetachRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeVolumeDetachRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)

	if IsDryRun(request) {
		if volumeID == 0 {
			return mcp.NewToolResultError("volume_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_volume_detach", httpMethodPost,
			fmt.Sprintf("/volumes/%d/detach", volumeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVolume(ctx, volumeID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return volumeDetachSideEffects(ctx, state)
			})
	}

	if result := RequireConfirm(request, "This detaches a block storage volume from an instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DetachVolume(ctx, volumeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to detach volume %d: %v", volumeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.VolumeDetachResponse{
		Message:  fmt.Sprintf("Volume %d detached successfully", volumeID),
		VolumeId: linodeIDToInt32(volumeID),
	})
}

// NewLinodeVolumeResizeTool creates a tool for resizing a volume.
func NewLinodeVolumeResizeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_resize",
		"Resizes a block storage volume. WARNING: Volumes can only be resized UP. This operation may incur additional billing.",
		toolschemas.Schema("linode.mcp.v1.VolumeResizeInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeResizeRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeVolumeResizeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	size := request.GetInt("size", 0)

	if IsDryRun(request) {
		if volumeID == 0 {
			return mcp.NewToolResultError("volume_id is required"), nil
		}

		if err := validateVolumeSize(size); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_volume_resize", httpMethodPost,
			fmt.Sprintf("/volumes/%d/resize", volumeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVolume(ctx, volumeID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return volumeResizeSideEffects(ctx, state, size)
			})
	}

	if result := RequireConfirm(request, "This operation may increase billing. Volumes cannot be downsized. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	if err := validateVolumeSize(size); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	volume, err := client.ResizeVolumeProto(ctx, volumeID, size)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resize volume %d: %v", volumeID, err)), nil
	}

	response := &linodev1.VolumeWriteResponse{
		Message: fmt.Sprintf("Volume %d resize to %d GB initiated successfully", volumeID, size),
		Volume:  volume,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVolumeUpdateTool creates a tool for updating a volume's label or tags.
func NewLinodeVolumeUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_update",
		"Updates a block storage volume's label or tags.",
		toolschemas.Schema("linode.mcp.v1.VolumeUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeVolumeUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	label := request.GetString("label", "")

	tags, hasTags, validationMessage := optionalTagsField(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		if volumeID == 0 {
			return mcp.NewToolResultError("volume_id is required"), nil
		}

		if label == "" && !hasTags {
			return mcp.NewToolResultError("at least one of label or tags is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_volume_update", "PUT",
			fmt.Sprintf("/volumes/%d", volumeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetVolume(ctx, volumeID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return volumeUpdateSideEffects(ctx, state, label, hasTags)
			})
	}

	if result := RequireConfirm(request, "This updates a block storage volume. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	if label == "" && !hasTags {
		return mcp.NewToolResultError("at least one of label or tags is required"), nil
	}

	req := linode.UpdateVolumeRequest{}

	if label != "" {
		req.Label = &label
	}

	if hasTags {
		req.Tags = tags
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	volume, err := client.UpdateVolumeProto(ctx, volumeID, &req)
	if err != nil {
		msg := fmt.Sprint("volume ", volumeID, " update failed: ", err)

		return mcp.NewToolResultError(msg), nil
	}

	response := &linodev1.VolumeWriteResponse{
		Message: fmt.Sprintf("Volume %d updated successfully", volumeID),
		Volume:  volume,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeVolumeDeleteTool creates a tool for deleting a volume.
func NewLinodeVolumeDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_delete",
		"Deletes a block storage volume. WARNING: This action is irreversible and all data will be permanently lost. The volume must be detached first. Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.VolumeDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// volumeDeleteProto builds the proto-canonical id-echo body for a successful
// volume delete, keeping the proto literal off the handler's struct literal so
// the delete handlers stay below the dupl threshold.
func volumeDeleteProto(id int) proto.Message {
	return &linodev1.VolumeDeleteResponse{
		Message:  fmt.Sprintf("Volume %d removed successfully", id),
		VolumeId: linodeIDToInt32(id),
	}
}

func handleLinodeVolumeDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_volume_delete",
		IDParam:        "volume_id",
		Method:         httpMethodDelete,
		PathPattern:    "/volumes/%d",
		ConfirmMessage: "This operation is destructive and irreversible. Set confirm=true to proceed.",
		SuccessProto:   volumeDeleteProto,
		FetchState:     func(ctx context.Context, c *linode.Client, id int) (any, error) { return c.GetVolume(ctx, id) },
		Execute:        func(ctx context.Context, c *linode.Client, id int) error { return c.DeleteVolume(ctx, id) },
		DependencyWalk: volumeDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("Volume"),
	})
}

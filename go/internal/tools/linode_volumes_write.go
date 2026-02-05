//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeVolumeCreateTool creates a tool for creating a volume.
func NewLinodeVolumeCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_volume_create",
		mcp.WithDescription("Creates a new block storage volume. WARNING: Billing starts immediately. Use linode_regions_list to find valid regions."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("A label for the volume (must be unique within your account)"),
		),
		mcp.WithString("region",
			mcp.Description("Region where the volume will be created. Required unless attaching to a Linode."),
		),
		mcp.WithNumber("size",
			mcp.Description("Size in GB (10-10240). Default is 20GB."),
		),
		mcp.WithNumber("linode_id",
			mcp.Description("Linode ID to attach the volume to (optional). If provided, region is inferred."),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm volume creation. This operation incurs billing charges."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeVolumeCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	label := request.GetString("label", "")
	region := request.GetString("region", "")
	size := request.GetInt("size", 0)
	linodeID := request.GetInt("linode_id", 0)
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation creates a billable resource. Set confirm=true to proceed."), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if region == "" && linodeID == 0 {
		return mcp.NewToolResultError("either region or linode_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.CreateVolumeRequest{
		Label:  label,
		Region: region,
		Size:   size,
	}

	if linodeID != 0 {
		req.LinodeID = &linodeID
	}

	volume, err := client.CreateVolume(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create volume: %v", err)), nil
	}

	response := struct {
		Message string         `json:"message"`
		Volume  *linode.Volume `json:"volume"`
	}{
		Message: fmt.Sprintf("Volume '%s' (ID: %d) created successfully in %s", volume.Label, volume.ID, volume.Region),
		Volume:  volume,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeVolumeAttachTool creates a tool for attaching a volume to a Linode.
func NewLinodeVolumeAttachTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_volume_attach",
		mcp.WithDescription("Attaches a block storage volume to a Linode instance. The volume and instance must be in the same region."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("volume_id",
			mcp.Required(),
			mcp.Description("The ID of the volume to attach"),
		),
		mcp.WithNumber("linode_id",
			mcp.Required(),
			mcp.Description("The ID of the Linode instance to attach the volume to"),
		),
		mcp.WithNumber("config_id",
			mcp.Description("The Linode config ID to attach to (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeAttachRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeVolumeAttachRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	volumeID := request.GetInt("volume_id", 0)
	linodeID := request.GetInt("linode_id", 0)
	configID := request.GetInt("config_id", 0)

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError("linode_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.AttachVolumeRequest{
		LinodeID: linodeID,
	}

	if configID != 0 {
		req.ConfigID = &configID
	}

	volume, err := client.AttachVolume(ctx, volumeID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to attach volume %d to Linode %d: %v", volumeID, linodeID, err)), nil
	}

	response := struct {
		Message  string         `json:"message"`
		LinodeID int            `json:"linode_id"` //nolint:tagliatelle // snake_case for consistent JSON
		Volume   *linode.Volume `json:"volume"`
	}{
		Message:  fmt.Sprintf("Volume %d attached to Linode %d successfully", volumeID, linodeID),
		LinodeID: linodeID,
		Volume:   volume,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeVolumeDetachTool creates a tool for detaching a volume from a Linode.
func NewLinodeVolumeDetachTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_volume_detach",
		mcp.WithDescription("Detaches a block storage volume from a Linode instance. The volume data is preserved."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("volume_id",
			mcp.Required(),
			mcp.Description("The ID of the volume to detach"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeDetachRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeVolumeDetachRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	volumeID := request.GetInt("volume_id", 0)

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DetachVolume(ctx, volumeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to detach volume %d: %v", volumeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		VolumeID int    `json:"volume_id"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message:  fmt.Sprintf("Volume %d detached successfully", volumeID),
		VolumeID: volumeID,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeVolumeResizeTool creates a tool for resizing a volume.
func NewLinodeVolumeResizeTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_volume_resize",
		mcp.WithDescription("Resizes a block storage volume. WARNING: Volumes can only be resized UP. This operation may incur additional billing."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("volume_id",
			mcp.Required(),
			mcp.Description("The ID of the volume to resize"),
		),
		mcp.WithNumber("size",
			mcp.Required(),
			mcp.Description("New size in GB (must be larger than current size)"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm resize. Volumes cannot be downsized."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeResizeRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeVolumeResizeRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	volumeID := request.GetInt("volume_id", 0)
	size := request.GetInt("size", 0)
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation may increase billing. Volumes cannot be downsized. Set confirm=true to proceed."), nil
	}

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	if size == 0 {
		return mcp.NewToolResultError("size is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	volume, err := client.ResizeVolume(ctx, volumeID, size)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resize volume %d: %v", volumeID, err)), nil
	}

	response := struct {
		Message string         `json:"message"`
		Volume  *linode.Volume `json:"volume"`
	}{
		Message: fmt.Sprintf("Volume %d resize to %d GB initiated successfully", volumeID, size),
		Volume:  volume,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeVolumeDeleteTool creates a tool for deleting a volume.
func NewLinodeVolumeDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_volume_delete",
		mcp.WithDescription("Deletes a block storage volume. WARNING: This action is irreversible and all data will be permanently lost. The volume must be detached first."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("volume_id",
			mcp.Required(),
			mcp.Description("The ID of the volume to delete"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion. This action is irreversible."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeVolumeDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	volumeID := request.GetInt("volume_id", 0)
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation is destructive and irreversible. Set confirm=true to proceed."), nil
	}

	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DeleteVolume(ctx, volumeID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete volume %d: %v", volumeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		VolumeID int    `json:"volume_id"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message:  fmt.Sprintf("Volume %d deleted successfully", volumeID),
		VolumeID: volumeID,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

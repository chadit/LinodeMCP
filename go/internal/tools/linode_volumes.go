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
)

// NewLinodeVolumeGetTool creates a tool for retrieving a single block storage volume by ID.
func NewLinodeVolumeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_volume_get",
		"Gets details for a single block storage volume by ID.",
		toolschemas.Schema("linode.mcp.v1.VolumeGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumeGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeVolumeGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	if volumeID == 0 {
		return mcp.NewToolResultError("volume_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	volume, err := client.GetVolumeProto(ctx, volumeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve volume %d: %v", volumeID, err)), nil
	}

	// Wrapped under a "volume" key to match the Python implementation's
	// response shape for this tool.
	return MarshalProtoToolResponse(&linodev1.VolumeGetResponse{Volume: volume})
}

// NewLinodeVolumeListTool creates a tool for listing Linode block storage volumes.
func NewLinodeVolumeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_volume_list",
		"Lists all block storage volumes for the authenticated user with optional filtering by region or label",
		"linode.mcp.v1.VolumeListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.Volume, error) {
			return client.ListVolumesProto(ctx)
		},
		[]listFilterParam[*linodev1.Volume]{
			fieldFilter("region", "Filter volumes by region (e.g., 'us-east', 'eu-west')",
				func(vol *linodev1.Volume) string { return vol.GetRegion() }),
			containsFilter("label_contains", "Filter volumes where label contains this string (case-insensitive)",
				func(vol *linodev1.Volume) string { return vol.GetLabel() }),
		},
		volumeListResponse,
	)

	return tool, profiles.CapRead, handler
}

func volumeListResponse(items []*linodev1.Volume, count int32, filter *string) *linodev1.VolumeListResponse {
	return &linodev1.VolumeListResponse{Count: count, Filter: filter, Volumes: items}
}

// NewLinodeVolumeTypeListTool creates a tool for listing Linode block storage volume types.
func NewLinodeVolumeTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_volume_type_list",
		"Lists available Linode block storage volume types and pricing",
		"linode.mcp.v1.VolumeTypeListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LinodeType, error) {
			return client.ListVolumeTypesProto(ctx)
		},
		nil,
		volumeTypeListResponse,
	)

	return tool, profiles.CapRead, handler
}

func volumeTypeListResponse(items []*linodev1.LinodeType, count int32, filter *string) *linodev1.VolumeTypeListResponse {
	return &linodev1.VolumeTypeListResponse{Count: count, Filter: filter, VolumeTypes: items}
}

package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeVolumeListTool creates a tool for listing Linode block storage volumes.
func NewLinodeVolumeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newListTool(
		cfg,
		"linode_volume_list",
		"Lists all block storage volumes for the authenticated user with optional filtering by region or label",
		func(ctx context.Context, client *linode.Client) ([]linode.Volume, error) {
			return client.ListVolumes(ctx)
		},
		[]listFilterParam[linode.Volume]{
			fieldFilter("region", "Filter volumes by region (e.g., 'us-east', 'eu-west')",
				func(vol linode.Volume) string { return vol.Region }),
			containsFilter("label_contains", "Filter volumes where label contains this string (case-insensitive)",
				func(vol linode.Volume) string { return vol.Label }),
		},
		"volumes",
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeVolumeTypeListTool creates a tool for listing Linode block storage volume types.
func NewLinodeVolumeTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newListTool(
		cfg,
		"linode_volume_type_list",
		"Lists available Linode block storage volume types and pricing",
		func(ctx context.Context, client *linode.Client) ([]linode.VolumeType, error) {
			return client.ListVolumeTypes(ctx)
		},
		[]listFilterParam[linode.VolumeType]{},
		"volume_types",
	)

	return tool, profiles.CapRead, handler
}

package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeVolumesListTool creates a tool for listing Linode block storage volumes.
func NewLinodeVolumesListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_volumes_list",
		"Lists all block storage volumes for the authenticated user with optional filtering by region or label",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.Volume, error) {
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
}

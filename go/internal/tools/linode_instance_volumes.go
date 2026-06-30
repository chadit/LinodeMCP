package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeInstanceVolumeListTool creates a tool for listing volumes attached to a Linode instance.
func NewLinodeInstanceVolumeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_instance_volume_list",
		"Lists volumes attached to a Linode instance with optional pagination.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		instanceConfigsPaginationFromTool,
		func(ctx context.Context, client *linode.Client, linodeID, page, pageSize int) ([]*linodev1.Volume, error) {
			return client.ListInstanceVolumesProto(ctx, linodeID, page, pageSize)
		},
		nil,
		instanceVolumeListResponse,
	)

	return tool, profiles.CapRead, handler
}

func instanceVolumeListResponse(items []*linodev1.Volume, count int32, filter *string) *linodev1.VolumeListResponse {
	return &linodev1.VolumeListResponse{Count: count, Filter: filter, Volumes: items}
}

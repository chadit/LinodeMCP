package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeInstanceVolumeListTool creates a tool for listing volumes attached to a Linode instance.
func NewLinodeInstanceVolumeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_volume_list",
		"Lists volumes attached to a Linode instance with optional pagination.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleInstanceVolumesListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceVolumesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := instanceConfigsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	volumes, err := client.ListInstanceVolumes(ctx, linodeID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list volumes for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count   int             `json:"count"`
		Volumes []linode.Volume `json:"volumes"`
	}{
		Count:   len(volumes),
		Volumes: volumes,
	}

	return MarshalToolResponse(response)
}

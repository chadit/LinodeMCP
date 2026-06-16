package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeInstanceStatsGetTool creates a tool for retrieving daily statistics for a Linode instance.
func NewLinodeInstanceStatsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_stats_get",
		"Gets daily CPU, IO, IPv4, and IPv6 statistics for a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceStatsGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceStatsGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	if _, exists := args["linode_id"]; !exists {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	linodeID, validationMessage := optionalPaginationInt(args, "linode_id", 1, 0)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	stats, err := client.GetInstanceStats(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get stats for instance %d: %v", linodeID, err)), nil
	}

	return MarshalToolResponse(stats)
}

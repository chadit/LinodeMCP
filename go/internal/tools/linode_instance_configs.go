package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	instanceConfigsPageSizeMin = 25
	instanceConfigsPageSizeMax = 500
)

// NewLinodeInstanceConfigListTool creates a tool for listing configuration profiles on a Linode instance.
func NewLinodeInstanceConfigListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_list",
		"Lists configuration profiles for a Linode instance with optional pagination.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleInstanceConfigsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceConfigsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	configs, err := client.ListInstanceConfigs(ctx, linodeID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list configuration profiles for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count   int                     `json:"count"`
		Configs []linode.InstanceConfig `json:"configs"`
	}{
		Count:   len(configs),
		Configs: configs,
	}

	return MarshalToolResponse(response)
}

func instanceConfigLinodeIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["linode_id"]; !exists {
		return 0, ErrLinodeIDRequired.Error()
	}

	linodeID, validationMessage := optionalPaginationInt(args, "linode_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return linodeID, ""
}

func instanceConfigsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", instanceConfigsPageSizeMin, instanceConfigsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

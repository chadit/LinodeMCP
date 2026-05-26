package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeMonitorAlertDefinitionsTool creates a tool for listing monitoring alert definitions.
func NewLinodeMonitorAlertDefinitionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_monitor_alert_definitions",
		"Lists monitoring alert definitions available to the user.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeMonitorAlertDefinitionsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorAlertDefinitionsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := monitorAlertDefinitionsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	definitions, listFailureMessage := listMonitorAlertDefinitions(ctx, client, page, pageSize)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_monitor_alert_definitions: " + listFailureMessage), nil
	}

	return MarshalToolResponse(definitions)
}

func listMonitorAlertDefinitions(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.AlertDefinition], string) {
	definitions, err := client.ListMonitorAlertDefinitions(ctx, page, pageSize)
	if err != nil {
		return nil, err.Error()
	}

	return definitions, ""
}

func monitorAlertDefinitionsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", monitorAlertChannelsPageSizeMin, monitorAlertChannelsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// NewLinodeMonitorAlertChannelsTool creates a tool for listing monitoring alert channels.
func NewLinodeMonitorAlertChannelsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_monitor_alert_channels",
		"Lists monitoring alert channels available to the user.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeMonitorAlertChannelsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorAlertChannelsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := monitorAlertChannelsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	channels, listFailureMessage := listMonitorAlertChannels(ctx, client, page, pageSize)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_monitor_alert_channels: " + listFailureMessage), nil
	}

	return MarshalToolResponse(channels)
}

func listMonitorAlertChannels(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.AlertChannel], string) {
	channels, err := client.ListMonitorAlertChannels(ctx, page, pageSize)
	if err != nil {
		return nil, err.Error()
	}

	return channels, ""
}

func monitorAlertChannelsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", monitorAlertChannelsPageSizeMin, monitorAlertChannelsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

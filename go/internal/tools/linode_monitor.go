package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	monitorServicesToolName       = "linode_monitor_services"
	monitorDashboardIDParam       = "dashboard_id"
	errMonitorDashboardIDMissing  = "dashboard_id is required"
	errMonitorDashboardIDPositive = "dashboard_id must be a positive integer"
)

// NewLinodeMonitorServicesTool creates a tool for listing supported monitoring service types.
func NewLinodeMonitorServicesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		monitorServicesToolName,
		"Lists supported monitoring service types.",
		nil,
		handleLinodeMonitorServicesRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorServicesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	services, listFailureMessage := listMonitorServices(ctx, client)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + monitorServicesToolName + ": " + listFailureMessage), nil
	}

	return MarshalToolResponse(services)
}

func listMonitorServices(ctx context.Context, client *linode.Client) (*linode.PaginatedResponse[linode.MonitorService], string) {
	services, err := client.ListMonitorServices(ctx)
	if err != nil {
		return nil, err.Error()
	}

	return services, ""
}

// NewLinodeMonitorDashboardsTool creates a tool for listing monitoring dashboards.
func NewLinodeMonitorDashboardsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_monitor_dashboards",
		"Lists monitoring dashboards available to the user.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeMonitorDashboardsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorDashboardsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := monitorDashboardsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dashboards, listFailureMessage := listMonitorDashboards(ctx, client, page, pageSize)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_monitor_dashboards: " + listFailureMessage), nil
	}

	return MarshalToolResponse(dashboards)
}

func listMonitorDashboards(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.MonitorDashboard], string) {
	dashboards, err := client.ListMonitorDashboards(ctx, page, pageSize)
	if err != nil {
		return nil, err.Error()
	}

	return dashboards, ""
}

func monitorDashboardsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
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

// NewLinodeMonitorDashboardGetTool creates a tool for retrieving one monitoring dashboard.
func NewLinodeMonitorDashboardGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_monitor_dashboard_get",
		"Gets one monitoring dashboard by dashboard_id.",
		[]mcp.ToolOption{
			mcp.WithNumber(monitorDashboardIDParam, mcp.Required(), mcp.Description("Monitoring dashboard ID to retrieve.")),
		},
		handleLinodeMonitorDashboardGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorDashboardGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	dashboardID, validationMessage := requiredPositiveIntArgument(request, monitorDashboardIDParam, errMonitorDashboardIDMissing, errMonitorDashboardIDPositive)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dashboard, getFailureMessage := getMonitorDashboard(ctx, client, dashboardID)
	if getFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_monitor_dashboard_get: " + getFailureMessage), nil
	}

	return MarshalToolResponse(dashboard)
}

func getMonitorDashboard(ctx context.Context, client *linode.Client, dashboardID int) (linode.MonitorDashboard, string) {
	dashboard, err := client.GetMonitorDashboard(ctx, dashboardID)
	if err != nil {
		return nil, err.Error()
	}

	return dashboard, ""
}

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

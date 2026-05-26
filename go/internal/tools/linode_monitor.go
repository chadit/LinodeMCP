package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	monitorServicesToolName                      = "linode_monitor_services"
	monitorServiceGetToolName                    = "linode_monitor_service_get"
	monitorServiceAlertDefinitionsToolName       = "linode_monitor_service_alert_definitions"
	monitorServiceAlertDefinitionCreateToolName  = "linode_monitor_service_alert_definition_create"
	monitorServiceAlertDefinitionGetToolName     = "linode_monitor_service_alert_definition_get"
	monitorServiceTypeParam                      = "service_type"
	monitorAlertDefinitionLabelParam             = "label"
	monitorAlertDefinitionSeverityParam          = "severity"
	monitorAlertDefinitionRuleCriteriaParam      = "rule_criteria"
	monitorAlertDefinitionTriggerConditionsParam = "trigger_conditions"
	monitorAlertDefinitionChannelIDsParam        = "channel_ids"
	monitorAlertDefinitionDescriptionParam       = "description"
	monitorAlertDefinitionEntityIDsParam         = "entity_ids"
	errMonitorServiceTypeInvalid                 = "service_type must be a single non-empty service type slug"
	monitorAlertIDParam                          = "alert_id"
	errMonitorAlertIDMissing                     = "alert_id is required"
	errMonitorAlertIDPositive                    = "alert_id must be a positive integer"
	errMonitorAlertDefinitionRequired            = "label, severity, rule_criteria, trigger_conditions, and channel_ids are required"
	errMonitorAlertDefinitionSeverity            = "severity must be an integer from 0 through 3"
	errMonitorAlertDefinitionChannels            = "channel_ids must be a non-empty array of positive integers"
	errMonitorAlertDefinitionEntityIDs           = "entity_ids must be an array of non-empty strings"
	monitorDashboardIDParam                      = "dashboard_id"
	errMonitorDashboardIDMissing                 = "dashboard_id is required"
	errMonitorDashboardIDPositive                = "dashboard_id must be a positive integer"
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

// NewLinodeMonitorServiceGetTool creates a tool for retrieving one supported monitoring service type.
func NewLinodeMonitorServiceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		monitorServiceGetToolName,
		"Gets details for one supported monitoring service type by service_type.",
		[]mcp.ToolOption{
			mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug to retrieve.")),
		},
		handleLinodeMonitorServiceGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorServiceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	service, getFailureMessage := getMonitorService(ctx, client, serviceType)
	if getFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + monitorServiceGetToolName + ": " + getFailureMessage), nil
	}

	return MarshalToolResponse(service)
}

func monitorServiceTypeFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, validationMessage := stringArgument(request, monitorServiceTypeParam, true)
	if validationMessage != "" {
		return "", validationMessage
	}

	value := strings.TrimSpace(raw)
	if value == "" || value != raw || !isMonitorServiceTypeSlug(value) {
		return "", errMonitorServiceTypeInvalid
	}

	return value, ""
}

func isMonitorServiceTypeSlug(value string) bool {
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			continue
		}

		if char == '-' && index != 0 && index != len(value)-1 {
			continue
		}

		return false
	}

	return true
}

func getMonitorService(ctx context.Context, client *linode.Client, serviceType string) (linode.MonitorService, string) {
	service, err := client.GetMonitorService(ctx, serviceType)
	if err != nil {
		return linode.MonitorService{}, err.Error()
	}

	return service, ""
}

// NewLinodeMonitorServiceAlertDefinitionsTool creates a tool for listing alert definitions for one monitoring service type.
func NewLinodeMonitorServiceAlertDefinitionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		monitorServiceAlertDefinitionsToolName,
		"Lists alert definitions for one supported monitoring service type by service_type.",
		[]mcp.ToolOption{
			mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug whose alert definitions should be listed.")),
		},
		handleLinodeMonitorServiceAlertDefinitionsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorServiceAlertDefinitionsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	definitions, listFailureMessage := listMonitorServiceAlertDefinitions(ctx, client, serviceType)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + monitorServiceAlertDefinitionsToolName + ": " + listFailureMessage), nil
	}

	return MarshalToolResponse(definitions)
}

func listMonitorServiceAlertDefinitions(ctx context.Context, client *linode.Client, serviceType string) (*linode.PaginatedResponse[linode.AlertDefinition], string) {
	definitions, err := client.ListMonitorServiceAlertDefinitions(ctx, serviceType)
	if err != nil {
		return nil, err.Error()
	}

	return definitions, ""
}

// NewLinodeMonitorServiceAlertDefinitionGetTool creates a tool for retrieving one alert definition for one monitoring service type.
func NewLinodeMonitorServiceAlertDefinitionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		monitorServiceAlertDefinitionGetToolName,
		"Gets one alert definition for one supported monitoring service type by service_type and alert_id.",
		[]mcp.ToolOption{
			mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug whose alert definition should be retrieved.")),
			mcp.WithNumber(monitorAlertIDParam, mcp.Required(), mcp.Description("Alert definition ID to retrieve.")),
		},
		handleLinodeMonitorServiceAlertDefinitionGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorServiceAlertDefinitionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	alertID, validationMessage := requiredPositiveIntArgument(request, monitorAlertIDParam, errMonitorAlertIDMissing, errMonitorAlertIDPositive)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	definition, getFailureMessage := getMonitorServiceAlertDefinition(ctx, client, serviceType, alertID)
	if getFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + monitorServiceAlertDefinitionGetToolName + ": " + getFailureMessage), nil
	}

	return MarshalToolResponse(definition)
}

func getMonitorServiceAlertDefinition(ctx context.Context, client *linode.Client, serviceType string, alertID int) (linode.AlertDefinition, string) {
	definition, err := client.GetMonitorServiceAlertDefinition(ctx, serviceType, alertID)
	if err != nil {
		return linode.AlertDefinition{}, err.Error()
	}

	return definition, ""
}

// NewLinodeMonitorServiceAlertDefinitionCreateTool creates a tool for creating one monitoring alert definition.
func NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		monitorServiceAlertDefinitionCreateToolName,
		"Creates an alert definition for one supported monitoring service type. Requires confirm=true.",
		[]mcp.ToolOption{
			mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug for the alert definition.")),
			mcp.WithString(monitorAlertDefinitionLabelParam, mcp.Required(), mcp.Description("Alert definition label.")),
			mcp.WithNumber(monitorAlertDefinitionSeverityParam, mcp.Required(), mcp.Description("Alert severity: 0 severe, 1 medium, 2 low, or 3 info.")),
			mcp.WithObject(monitorAlertDefinitionRuleCriteriaParam, mcp.Required(), mcp.Description("Alert rule criteria object.")),
			mcp.WithObject(monitorAlertDefinitionTriggerConditionsParam, mcp.Required(), mcp.Description("Alert trigger conditions object.")),
			mcp.WithArray(monitorAlertDefinitionChannelIDsParam, mcp.Required(), mcp.Description("Alert channel IDs.")),
			mcp.WithString(monitorAlertDefinitionDescriptionParam, mcp.Description("Optional alert definition description.")),
			mcp.WithArray(monitorAlertDefinitionEntityIDsParam, mcp.Description("Optional service entity IDs.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm creating an alert definition.")),
		},
		handleLinodeMonitorServiceAlertDefinitionCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeMonitorServiceAlertDefinitionCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a monitor alert definition. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	createRequest, validationMessage := monitorServiceAlertDefinitionCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	definition, createFailureMessage := createMonitorServiceAlertDefinition(ctx, client, serviceType, createRequest)
	if createFailureMessage != "" {
		return mcp.NewToolResultError("Failed to create " + monitorServiceAlertDefinitionCreateToolName + ": " + createFailureMessage), nil
	}

	return MarshalToolResponse(definition)
}

func monitorServiceAlertDefinitionCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateAlertDefinitionRequest, string) {
	args := request.GetArguments()

	label, validationMessage := stringArgument(request, monitorAlertDefinitionLabelParam, true)
	if validationMessage != "" {
		return nil, errMonitorAlertDefinitionRequired
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return nil, errMonitorAlertDefinitionRequired
	}

	severity, validationMessage := monitorAlertDefinitionSeverityFromArgs(args)
	if validationMessage != "" {
		return nil, validationMessage
	}

	ruleCriteria, validationMessage := objectArgument(args, monitorAlertDefinitionRuleCriteriaParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	triggerConditions, validationMessage := objectArgument(args, monitorAlertDefinitionTriggerConditionsParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	channelIDs, validationMessage := intArrayArgument(args, monitorAlertDefinitionChannelIDsParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	entityIDs, validationMessage := optionalStringArrayArgument(args, monitorAlertDefinitionEntityIDsParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	var description *string

	if rawDescription, exists := args[monitorAlertDefinitionDescriptionParam]; exists {
		descriptionString, ok := rawDescription.(string)
		if !ok {
			return nil, monitorAlertDefinitionDescriptionParam + " must be a string"
		}

		description = &descriptionString
	}

	return &linode.CreateAlertDefinitionRequest{
		ChannelIDs:        channelIDs,
		Description:       description,
		EntityIDs:         entityIDs,
		Label:             label,
		RuleCriteria:      ruleCriteria,
		Severity:          severity,
		TriggerConditions: triggerConditions,
	}, ""
}

func monitorAlertDefinitionSeverityFromArgs(args map[string]any) (int, string) {
	raw, exists := args[monitorAlertDefinitionSeverityParam]
	if !exists {
		return 0, errMonitorAlertDefinitionRequired
	}

	var severity int

	switch typed := raw.(type) {
	case int:
		severity = typed
	case int64:
		severity = int(typed)
	case float64:
		severity = int(typed)
		if typed != float64(severity) {
			return 0, errMonitorAlertDefinitionSeverity
		}
	default:
		return 0, errMonitorAlertDefinitionSeverity
	}

	if severity < 0 || severity > 3 {
		return 0, errMonitorAlertDefinitionSeverity
	}

	return severity, ""
}

func objectArgument(args map[string]any, name string) (map[string]any, string) {
	raw, exists := args[name]
	if !exists {
		return nil, errMonitorAlertDefinitionRequired
	}

	objectValue, ok := raw.(map[string]any)
	if !ok || len(objectValue) == 0 {
		return nil, name + " must be a non-empty object"
	}

	return objectValue, ""
}

func intArrayArgument(args map[string]any, name string) ([]int, string) {
	raw, exists := args[name]
	if !exists {
		return nil, errMonitorAlertDefinitionRequired
	}

	rawItems, ok := raw.([]any)
	if !ok || len(rawItems) == 0 {
		return nil, errMonitorAlertDefinitionChannels
	}

	items := make([]int, 0, len(rawItems))
	for _, rawItem := range rawItems {
		value, ok := intFromAny(rawItem)
		if !ok || value <= 0 {
			return nil, errMonitorAlertDefinitionChannels
		}

		items = append(items, value)
	}

	return items, ""
}

func optionalStringArrayArgument(args map[string]any, name string) ([]string, string) {
	raw, exists := args[name]
	if !exists {
		return nil, ""
	}

	rawItems, ok := raw.([]any)
	if !ok {
		return nil, errMonitorAlertDefinitionEntityIDs
	}

	items := make([]string, 0, len(rawItems))
	for _, rawItem := range rawItems {
		value, ok := rawItem.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, errMonitorAlertDefinitionEntityIDs
		}

		items = append(items, value)
	}

	return items, ""
}

func intFromAny(raw any) (int, bool) {
	switch typed := raw.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		value := int(typed)

		return value, typed == float64(value)
	default:
		return 0, false
	}
}

func createMonitorServiceAlertDefinition(ctx context.Context, client *linode.Client, serviceType string, request *linode.CreateAlertDefinitionRequest) (*linode.AlertDefinition, string) {
	definition, err := client.CreateMonitorServiceAlertDefinition(ctx, serviceType, request)
	if err != nil {
		return nil, err.Error()
	}

	return definition, ""
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

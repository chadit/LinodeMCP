package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	monitorServicesToolName                     = "linode_monitor_service_list"
	monitorServiceGetToolName                   = "linode_monitor_service_get"
	monitorServiceMetricDefinitionsToolName     = "linode_monitor_service_metric_definition_list"
	monitorServiceAlertDefinitionsToolName      = "linode_monitor_service_alert_definition_list"
	monitorServiceDashboardsToolName            = "linode_monitor_service_dashboard_list"
	monitorServiceMetricsToolName               = "linode_monitor_service_metric_query"
	monitorServiceCreateToolName                = "linode_monitor_service_" + "token_create"
	monitorServiceAlertDefinitionCreateToolName = "linode_monitor_service_alert_definition_create"
	monitorServiceAlertDefinitionGetToolName    = "linode_monitor_service_alert_definition_get"
	monitorServiceAlertDefinitionDeleteToolName = "linode_monitor_service_alert_definition_delete"

	monitorServiceAlertDefinitionUpdateToolName  = "linode_monitor_service_alert_definition_update"
	monitorServiceTypeParam                      = "service_type"
	monitorServicesPath                          = "/monitor/services"
	monitorAlertDefinitionLabelParam             = "label"
	monitorAlertDefinitionSeverityParam          = "severity"
	monitorAlertDefinitionRuleCriteriaParam      = "rule_criteria"
	monitorAlertDefinitionTriggerConditionsParam = "trigger_conditions"
	monitorAlertDefinitionChannelIDsParam        = "channel_ids"
	monitorAlertDefinitionDescriptionParam       = "description"
	monitorAlertDefinitionEntityIDsParam         = "entity_ids"
	monitorAlertDefinitionStatusParam            = "status"
	monitorAlertDefinitionStatusEnabled          = "enabled"
	monitorAlertDefinitionStatusDisabled         = "disabled"
	errMonitorServiceTypeInvalid                 = "service_type must be a single non-empty service type slug"
	errMonitorServiceCreateEntityIDs             = "entity_ids must be a non-empty array of positive integers"
	monitorAlertIDParam                          = "alert_id"
	errMonitorAlertIDMissing                     = "alert_id is required"
	errMonitorAlertIDPositive                    = "alert_id must be a positive integer"
	errMonitorAlertDefinitionRequired            = "label, severity, rule_criteria, trigger_conditions, and channel_ids are required"
	errMonitorAlertDefinitionSeverity            = "severity must be an integer from 0 through 3"
	errMonitorAlertDefinitionChannels            = "channel_ids must be a non-empty array of positive integers"
	errMonitorAlertDefinitionEntityIDs           = "entity_ids must be an array of non-empty strings"
	errMonitorAlertDefinitionUpdateEmpty         = "at least one alert definition field must be provided"
	errMonitorAlertDefinitionStatus              = "status must be enabled or disabled"
	monitorDashboardIDParam                      = "dashboard_id"
	errMonitorDashboardIDMissing                 = "dashboard_id is required"
	errMonitorDashboardIDPositive                = "dashboard_id must be a positive integer"
)

// NewLinodeMonitorServicesTool creates a tool for listing supported monitoring service types.
func NewLinodeMonitorServicesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolRawSchema(
		cfg,
		monitorServicesToolName,
		"Lists supported monitoring service types.",
		"linode.mcp.v1.MonitorServiceListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.MonitorService, error) {
			return client.ListMonitorServicesProto(ctx)
		},
		nil,
		monitorServiceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorServiceListResponse(items []*linodev1.MonitorService, count int32, filter *string) *linodev1.MonitorServiceListResponse {
	return &linodev1.MonitorServiceListResponse{Count: count, Filter: filter, Services: items}
}

// NewLinodeMonitorServiceGetTool creates a tool for retrieving one supported monitoring service type.
func NewLinodeMonitorServiceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceGetToolName,
		"Gets details for one supported monitoring service type by service_type.",
		toolschemas.Schema("linode.mcp.v1.MonitorServiceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceGetRequest(ctx, &request, cfg)
	}

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

	service, getFailure := client.GetMonitorServiceProto(ctx, serviceType)
	if getFailure == nil {
		return MarshalProtoToolResponse(service)
	}

	return mcp.NewToolResultError("Failed to retrieve " + monitorServiceGetToolName + ": " + getFailure.Error()), nil
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

// NewLinodeMonitorServiceMetricDefinitionsTool creates a tool for listing metric definitions for one supported monitoring service type.
func NewLinodeMonitorServiceMetricDefinitionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourceStringRawSchema(
		cfg,
		monitorServiceMetricDefinitionsToolName,
		"Lists metric definitions for one supported monitoring service type by service_type.",
		"linode.mcp.v1.MonitorServiceMetricDefinitionListInput",
		protoListPathIDString{
			option: mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug whose metric definitions should be listed.")),
			parse:  monitorServiceTypeFromTool,
		},
		func(ctx context.Context, client *linode.Client, serviceType string) ([]*linodev1.MonitorMetricDefinition, error) {
			return client.ListMonitorServiceMetricDefinitionsProto(ctx, serviceType)
		},
		nil,
		monitorServiceMetricDefinitionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorServiceMetricDefinitionListResponse(items []*linodev1.MonitorMetricDefinition, count int32, filter *string) *linodev1.MonitorServiceMetricDefinitionListResponse {
	return &linodev1.MonitorServiceMetricDefinitionListResponse{Count: count, Filter: filter, MetricDefinitions: items}
}

// NewLinodeMonitorServiceAlertDefinitionsTool creates a tool for listing alert definitions for one monitoring service type.
func NewLinodeMonitorServiceAlertDefinitionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourceStringRawSchema(
		cfg,
		monitorServiceAlertDefinitionsToolName,
		"Lists alert definitions for one supported monitoring service type by service_type.",
		"linode.mcp.v1.MonitorServiceAlertDefinitionListInput",
		protoListPathIDString{
			option: mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug whose alert definitions should be listed.")),
			parse:  monitorServiceTypeFromTool,
		},
		func(ctx context.Context, client *linode.Client, serviceType string) ([]*linodev1.MonitorAlertDefinition, error) {
			return client.ListMonitorServiceAlertDefinitionsProto(ctx, serviceType)
		},
		nil,
		monitorServiceAlertDefinitionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorServiceAlertDefinitionListResponse(items []*linodev1.MonitorAlertDefinition, count int32, filter *string) *linodev1.MonitorServiceAlertDefinitionListResponse {
	return &linodev1.MonitorServiceAlertDefinitionListResponse{Count: count, Filter: filter, AlertDefinitions: items}
}

// NewLinodeMonitorServiceDashboardsTool creates a tool for listing dashboards for one monitoring service type.
func NewLinodeMonitorServiceDashboardsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourceStringRawSchema(
		cfg,
		monitorServiceDashboardsToolName,
		"Lists dashboards for one supported monitoring service type by service_type.",
		"linode.mcp.v1.MonitorServiceDashboardListInput",
		protoListPathIDString{
			option: mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug whose dashboards should be listed.")),
			parse:  monitorServiceTypeFromTool,
		},
		func(ctx context.Context, client *linode.Client, serviceType string) ([]*linodev1.MonitorDashboard, error) {
			return client.ListMonitorServiceDashboardsProto(ctx, serviceType)
		},
		nil,
		monitorServiceDashboardListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorServiceDashboardListResponse(items []*linodev1.MonitorDashboard, count int32, filter *string) *linodev1.MonitorServiceDashboardListResponse {
	return &linodev1.MonitorServiceDashboardListResponse{Count: count, Filter: filter, Dashboards: items}
}

// NewLinodeMonitorServiceMetricsTool creates a tool for retrieving metrics for one monitoring service type.
func NewLinodeMonitorServiceMetricsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceMetricsToolName,
		"Gets metrics for one supported monitoring service type by service_type.",
		toolschemas.Schema("linode.mcp.v1.MonitorServiceMetricQueryInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceMetricsRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorServiceMetricsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	metricsStruct, getFailureMessage := getMonitorServiceMetrics(ctx, client, serviceType)
	if getFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + monitorServiceMetricsToolName + ": " + getFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.MonitorServiceMetricQueryResponse{
		Message:     fmt.Sprintf("Monitor service metrics read for '%s'", serviceType),
		ServiceType: serviceType,
		Metrics:     metricsStruct,
	})
}

// getMonitorServiceMetrics fetches the metrics map and converts it to the
// structpb.Struct the proto envelope needs, so the handler never returns a
// non-nil error as a tool result (which nilerr flags). Either failure surfaces
// as a string the caller wraps.
func getMonitorServiceMetrics(ctx context.Context, client *linode.Client, serviceType string) (*structpb.Struct, string) {
	metrics, err := client.GetMonitorServiceMetrics(ctx, serviceType)
	if err != nil {
		return nil, err.Error()
	}

	metricsStruct, structFailure := structpb.NewStruct(metrics)
	if structFailure != nil {
		return nil, structFailure.Error()
	}

	return metricsStruct, ""
}

// NewLinodeMonitorServiceTokenCreateTool creates a tool for creating a token for one monitoring service type.
func NewLinodeMonitorServiceTokenCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceCreateToolName,
		"Creates a token for one supported monitoring service type. Requires confirm=true. Pass dry_run=true to preview without creating.",
		toolschemas.Schema("linode.mcp.v1.MonitorServiceTokenCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceTokenCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// runMonitorServiceCreate validates the service type, runs the create-specific
// build/validation, previews on dry_run (nil-fetch POST, current_state null
// since the resource does not exist yet), then gates on confirm and executes.
// Shared by the token and alert-definition create handlers, which are
// otherwise identical and trip the dupl linter once both gain a dry-run branch.
// build returns the execute closure (capturing the parsed create request) plus
// a validation message; it runs after the service-type check so error
// precedence is preserved.
func runMonitorServiceCreate(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, verb, confirmMessage string,
	build func(*mcp.CallToolRequest) (func(context.Context, *linode.Client, string) (proto.Message, string), string),
) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	execute, validationMessage := build(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, toolName, httpMethodPost,
			fmt.Sprintf(monitorServicesPath+"/%s/"+verb, serviceType), nil)
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, createFailureMessage := execute(ctx, client, serviceType)
	if createFailureMessage != "" {
		return mcp.NewToolResultError("Failed to create " + toolName + ": " + createFailureMessage), nil
	}

	return MarshalProtoToolResponse(result)
}

func handleLinodeMonitorServiceTokenCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runMonitorServiceCreate(ctx, request, cfg, monitorServiceCreateToolName, "token",
		"This creates a monitor service token. Set confirm=true to proceed.",
		func(r *mcp.CallToolRequest) (func(context.Context, *linode.Client, string) (proto.Message, string), string) {
			createRequest, validationMessage := monitorServiceTokenCreateRequestFromTool(r)
			if validationMessage != "" {
				return nil, validationMessage
			}

			return func(ctx context.Context, client *linode.Client, serviceType string) (proto.Message, string) {
				return createMonitorServiceToken(ctx, client, serviceType, createRequest)
			}, ""
		})
}

func monitorServiceTokenCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateMonitorServiceTokenRequest, string) {
	entityIDs, validationMessage := monitorServiceTokenEntityIDsFromTool(request)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.CreateMonitorServiceTokenRequest{EntityIDs: entityIDs}, ""
}

func monitorServiceTokenEntityIDsFromTool(request *mcp.CallToolRequest) ([]int, string) {
	args := request.GetArguments()

	raw, exists := args[monitorAlertDefinitionEntityIDsParam]
	if !exists {
		return nil, errMonitorServiceCreateEntityIDs
	}

	rawItems, ok := raw.([]any)
	if !ok || len(rawItems) == 0 {
		return nil, errMonitorServiceCreateEntityIDs
	}

	items := make([]int, 0, len(rawItems))
	for _, rawItem := range rawItems {
		value, ok := intFromAny(rawItem)
		if !ok || value <= 0 {
			return nil, errMonitorServiceCreateEntityIDs
		}

		items = append(items, value)
	}

	return items, ""
}

func createMonitorServiceToken(ctx context.Context, client *linode.Client, serviceType string, request *linode.CreateMonitorServiceTokenRequest) (*linodev1.MonitorServiceTokenCreateResponse, string) {
	token, err := client.CreateMonitorServiceToken(ctx, serviceType, request)
	if err != nil {
		return nil, err.Error()
	}

	return token, ""
}

// NewLinodeMonitorServiceAlertDefinitionGetTool creates a tool for retrieving one alert definition for one monitoring service type.
func NewLinodeMonitorServiceAlertDefinitionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceAlertDefinitionGetToolName,
		"Gets one alert definition for one supported monitoring service type by service_type and alert_id.",
		toolschemas.Schema("linode.mcp.v1.MonitorAlertDefinitionGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceAlertDefinitionGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorServiceAlertDefinitionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	alertID, validationMessage := requiredIDArgument(request, monitorAlertIDParam)
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

	return MarshalProtoToolResponse(definition)
}

func getMonitorServiceAlertDefinition(ctx context.Context, client *linode.Client, serviceType string, alertID int) (*linodev1.MonitorAlertDefinition, string) {
	definition, err := client.GetMonitorServiceAlertDefinitionProto(ctx, serviceType, alertID)
	if err != nil {
		return nil, err.Error()
	}

	return definition, ""
}

// NewLinodeMonitorServiceAlertDefinitionDeleteTool creates a tool for deleting one monitoring alert definition.
func NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceAlertDefinitionDeleteToolName,
		"Deletes one alert definition for a supported monitoring service type by service_type and alert_id. Requires confirm=true. Pass dry_run=true to preview without deleting.",
		toolschemas.Schema("linode.mcp.v1.MonitorAlertDefinitionDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceAlertDefinitionDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLinodeMonitorServiceAlertDefinitionDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		serviceType, validationMessage := monitorServiceTypeFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		alertID, validationMessage := requiredIDArgument(request, monitorAlertIDParam)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, monitorServiceAlertDefinitionDeleteToolName, httpMethodDelete,
			fmt.Sprintf(monitorServicesPath+"/%s/alert-definitions/%d", serviceType, alertID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetMonitorServiceAlertDefinition(ctx, serviceType, alertID)
			})
	}

	if result := requireDestroyConfirmation(ctx, request, monitorServiceAlertDefinitionDeleteToolName, "This deletes a monitor alert definition. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	alertID, validationMessage := requiredIDArgument(request, monitorAlertIDParam)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deleteFailureMessage := deleteMonitorServiceAlertDefinition(ctx, client, serviceType, alertID)
	if deleteFailureMessage != "" {
		return mcp.NewToolResultError("Failed to delete " + monitorServiceAlertDefinitionDeleteToolName + ": " + deleteFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.MonitorAlertDefinitionDeleteResponse{
		Message:     fmt.Sprintf("Monitor service alert definition %d deleted for '%s'", alertID, serviceType),
		ServiceType: serviceType,
		AlertId:     linodeIDToInt32(alertID),
	})
}

func deleteMonitorServiceAlertDefinition(ctx context.Context, client *linode.Client, serviceType string, alertID int) string {
	if err := client.DeleteMonitorServiceAlertDefinition(ctx, serviceType, alertID); err != nil {
		return err.Error()
	}

	return ""
}

// NewLinodeMonitorServiceAlertDefinitionCreateTool creates a tool for creating one monitoring alert definition.
func NewLinodeMonitorServiceAlertDefinitionCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceAlertDefinitionCreateToolName,
		"Creates an alert definition for one supported monitoring service type. Requires confirm=true.",
		toolschemas.Schema("linode.mcp.v1.MonitorServiceAlertDefinitionCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceAlertDefinitionCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeMonitorServiceAlertDefinitionCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runMonitorServiceCreate(ctx, request, cfg, monitorServiceAlertDefinitionCreateToolName, "alert-definitions",
		"This creates a monitor alert definition. Set confirm=true to proceed.",
		func(r *mcp.CallToolRequest) (func(context.Context, *linode.Client, string) (proto.Message, string), string) {
			createRequest, validationMessage := monitorServiceAlertDefinitionCreateRequestFromTool(r)
			if validationMessage != "" {
				return nil, validationMessage
			}

			return func(ctx context.Context, client *linode.Client, serviceType string) (proto.Message, string) {
				definition, failureMessage := createMonitorServiceAlertDefinition(ctx, client, serviceType, createRequest)
				if failureMessage != "" {
					return nil, failureMessage
				}

				return &linodev1.MonitorAlertDefinitionWriteResponse{
					Message:         fmt.Sprintf("Monitor service alert definition created for '%s'", serviceType),
					AlertDefinition: definition,
				}, ""
			}, ""
		})
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

func createMonitorServiceAlertDefinition(ctx context.Context, client *linode.Client, serviceType string, request *linode.CreateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, string) {
	definition, err := client.CreateMonitorServiceAlertDefinitionProto(ctx, serviceType, request)
	if err != nil {
		return nil, err.Error()
	}

	return definition, ""
}

// NewLinodeMonitorDashboardsTool creates a tool for listing monitoring dashboards.
func NewLinodeMonitorDashboardsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_monitor_dashboard_list",
		"Lists monitoring dashboards available to the user.",
		"linode.mcp.v1.MonitorDashboardListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.MonitorDashboard, error) {
			return client.ListMonitorDashboardsProto(ctx, page, pageSize)
		},
		monitorDashboardsPaginationFromTool,
		nil,
		monitorDashboardListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorDashboardListResponse(items []*linodev1.MonitorDashboard, count int32, filter *string) *linodev1.MonitorDashboardListResponse {
	return &linodev1.MonitorDashboardListResponse{Count: count, Filter: filter, Dashboards: items}
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
	tool := mcp.NewToolWithRawSchema(
		"linode_monitor_dashboard_get",
		"Gets one monitoring dashboard by dashboard_id.",
		toolschemas.Schema("linode.mcp.v1.MonitorDashboardGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorDashboardGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeMonitorDashboardGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	dashboardID, validationMessage := requiredIDArgument(request, monitorDashboardIDParam)
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

	return MarshalProtoToolResponse(dashboard)
}

func getMonitorDashboard(ctx context.Context, client *linode.Client, dashboardID int) (*linodev1.MonitorDashboard, string) {
	dashboard, err := client.GetMonitorDashboardProto(ctx, dashboardID)
	if err != nil {
		return nil, err.Error()
	}

	return dashboard, ""
}

// NewLinodeMonitorAlertDefinitionsTool creates a tool for listing monitoring alert definitions.
func NewLinodeMonitorAlertDefinitionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_monitor_alert_definition_list",
		"Lists monitoring alert definitions available to the user.",
		"linode.mcp.v1.MonitorAlertDefinitionListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.MonitorAlertDefinition, error) {
			return client.ListMonitorAlertDefinitionsProto(ctx, page, pageSize)
		},
		monitorAlertDefinitionsPaginationFromTool,
		nil,
		monitorAlertDefinitionListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorAlertDefinitionListResponse(items []*linodev1.MonitorAlertDefinition, count int32, filter *string) *linodev1.MonitorAlertDefinitionListResponse {
	return &linodev1.MonitorAlertDefinitionListResponse{Count: count, Filter: filter, AlertDefinitions: items}
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
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_monitor_alert_channel_list",
		"Lists monitoring alert channels available to the user.",
		"linode.mcp.v1.MonitorAlertChannelListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.MonitorAlertChannel, error) {
			return client.ListMonitorAlertChannelsProto(ctx, page, pageSize)
		},
		monitorAlertChannelsPaginationFromTool,
		nil,
		monitorAlertChannelListResponse,
	)

	return tool, profiles.CapRead, handler
}

func monitorAlertChannelListResponse(items []*linodev1.MonitorAlertChannel, count int32, filter *string) *linodev1.MonitorAlertChannelListResponse {
	return &linodev1.MonitorAlertChannelListResponse{Count: count, Filter: filter, AlertChannels: items}
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

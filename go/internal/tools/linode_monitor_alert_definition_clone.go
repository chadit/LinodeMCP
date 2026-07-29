package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeMonitorServiceAlertDefinitionCloneTool creates a tool for cloning one monitoring alert definition.
func NewLinodeMonitorServiceAlertDefinitionCloneTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		monitorServiceAlertDefinitionCloneToolName,
		"Clones one alert definition for a supported monitoring service type. Requires confirm=true.",
		toolschemas.Schema("linode.mcp.v1.MonitorServiceAlertDefinitionCloneInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeMonitorServiceAlertDefinitionCloneRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeMonitorServiceAlertDefinitionCloneRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	alertID, validationMessage := requiredIDArgument(request, monitorAlertIDParam)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	cloneRequest, validationMessage := monitorServiceAlertDefinitionCloneRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		path := fmt.Sprintf(
			monitorServicesPath+"/%s/alert-definitions/%s/clone",
			url.PathEscape(serviceType),
			url.PathEscape(strconv.Itoa(alertID)),
		)

		return RunDryRunPreviewWithBody(ctx, request, cfg, monitorServiceAlertDefinitionCloneToolName, httpMethodPost, path, cloneRequest,
			func(ctx context.Context, client *linode.Client) (any, error) {
				return client.GetMonitorServiceAlertDefinition(ctx, serviceType, alertID)
			})
	}

	if result := RequireConfirm(request, "This clones a monitor alert definition. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	definition, err := client.CloneMonitorServiceAlertDefinitionProto(ctx, serviceType, alertID, cloneRequest)
	if err != nil {
		//nolint:nilerr // API errors are returned as MCP tool results.
		return mcp.NewToolResultError("Failed to clone " + monitorServiceAlertDefinitionCloneToolName + ": " + err.Error()), nil
	}

	return MarshalProtoToolResponse(&linodev1.MonitorAlertDefinitionWriteResponse{
		Message:         fmt.Sprintf("Monitor alert definition %d cloned", alertID),
		AlertDefinition: definition,
	})
}

func monitorServiceAlertDefinitionCloneRequestFromTool(request *mcp.CallToolRequest) (*linode.CloneAlertDefinitionRequest, string) {
	args := request.GetArguments()

	label, validationMessage := stringArgument(request, monitorAlertDefinitionLabelParam, true)
	if validationMessage != "" || strings.TrimSpace(label) == "" {
		return nil, monitorAlertDefinitionLabelParam + " must be a non-empty string"
	}

	cloneRequest := &linode.CloneAlertDefinitionRequest{Label: strings.TrimSpace(label)}

	if raw, exists := args[monitorAlertDefinitionChannelIDsParam]; exists {
		channelIDs, ok := integerArray(raw)
		if !ok {
			return nil, monitorAlertDefinitionChannelIDsParam + " must be an array of integers"
		}

		cloneRequest.ChannelIDs = &channelIDs
	}

	if raw, exists := args[monitorAlertDefinitionDescriptionParam]; exists {
		description, ok := raw.(string)
		if !ok {
			return nil, monitorAlertDefinitionDescriptionParam + " must be a string"
		}

		cloneRequest.Description = &description
	}

	if raw, exists := args[monitorAlertDefinitionGroupByParam]; exists {
		groupBy, ok := stringArray(raw)
		if !ok {
			return nil, monitorAlertDefinitionGroupByParam + " must be an array of strings"
		}

		cloneRequest.GroupBy = &groupBy
	}

	if raw, exists := args[monitorAlertDefinitionRuleCriteriaParam]; exists {
		ruleCriteria, ok := raw.(map[string]any)
		if !ok {
			return nil, monitorAlertDefinitionRuleCriteriaParam + " must be an object"
		}

		cloneRequest.RuleCriteria = &ruleCriteria
	}

	if _, exists := args[monitorAlertDefinitionSeverityParam]; exists {
		severity, severityMessage := monitorAlertDefinitionSeverityFromArgs(args)
		if severityMessage != "" {
			return nil, severityMessage
		}

		cloneRequest.Severity = &severity
	}

	if raw, exists := args[monitorAlertDefinitionTriggerConditionsParam]; exists {
		triggerConditions, ok := raw.(map[string]any)
		if !ok {
			return nil, monitorAlertDefinitionTriggerConditionsParam + " must be an object"
		}

		cloneRequest.TriggerConditions = &triggerConditions
	}

	return cloneRequest, ""
}

func integerArray(raw any) ([]int, bool) {
	rawItems, ok := raw.([]any)
	if !ok {
		return nil, false
	}

	items := make([]int, 0, len(rawItems))
	for _, rawItem := range rawItems {
		value, ok := intFromAny(rawItem)
		if !ok {
			return nil, false
		}

		items = append(items, value)
	}

	return items, true
}

func stringArray(raw any) ([]string, bool) {
	rawItems, ok := raw.([]any)
	if !ok {
		return nil, false
	}

	items := make([]string, 0, len(rawItems))
	for _, rawItem := range rawItems {
		value, ok := rawItem.(string)
		if !ok {
			return nil, false
		}

		items = append(items, value)
	}

	return items, true
}

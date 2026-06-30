package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeMonitorServiceAlertDefinitionUpdateTool creates a tool for updating one monitoring alert definition.
func NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		monitorServiceAlertDefinitionUpdateToolName,
		"Updates one alert definition for a supported monitoring service type. Requires confirm=true.",
		[]mcp.ToolOption{
			mcp.WithString(monitorServiceTypeParam, mcp.Required(), mcp.Description("Supported monitoring service type slug for the alert definition.")),
			mcp.WithNumber(monitorAlertIDParam, mcp.Required(), mcp.Description("Alert definition ID to update.")),
			mcp.WithString(monitorAlertDefinitionLabelParam, mcp.Description("Optional alert definition label.")),
			mcp.WithNumber(monitorAlertDefinitionSeverityParam, mcp.Description("Optional alert severity: 0 severe, 1 medium, 2 low, or 3 info.")),
			mcp.WithString(monitorAlertDefinitionStatusParam, mcp.Description("Optional alert status: enabled or disabled.")),
			mcp.WithObject(monitorAlertDefinitionRuleCriteriaParam, mcp.Description("Optional alert rule criteria object.")),
			mcp.WithObject(monitorAlertDefinitionTriggerConditionsParam, mcp.Description("Optional alert trigger conditions object.")),
			mcp.WithArray(monitorAlertDefinitionChannelIDsParam, mcp.Description("Optional alert channel IDs.")),
			mcp.WithString(monitorAlertDefinitionDescriptionParam, mcp.Description("Optional alert definition description.")),
			mcp.WithArray(monitorAlertDefinitionEntityIDsParam, mcp.Description("Optional service entity IDs.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm updating an alert definition. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeMonitorServiceAlertDefinitionUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeMonitorServiceAlertDefinitionUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceType, validationMessage := monitorServiceTypeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	alertID, validationMessage := requiredPositiveIntArgument(request, monitorAlertIDParam, errMonitorAlertIDMissing, errMonitorAlertIDPositive)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateRequest, validationMessage := monitorServiceAlertDefinitionUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, monitorServiceAlertDefinitionUpdateToolName, "PUT",
			fmt.Sprintf(monitorServicesPath+"/%s/alert-definitions/%d", serviceType, alertID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetMonitorServiceAlertDefinition(ctx, serviceType, alertID)
			})
	}

	if result := RequireConfirm(request, "This updates a monitor alert definition. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	definition, updateFailureMessage := updateMonitorServiceAlertDefinition(ctx, client, serviceType, alertID, updateRequest)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError("Failed to update " + monitorServiceAlertDefinitionUpdateToolName + ": " + updateFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.MonitorAlertDefinitionWriteResponse{
		Message:         fmt.Sprintf("Monitor alert definition %d updated", alertID),
		AlertDefinition: definition,
	})
}

func monitorServiceAlertDefinitionUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateAlertDefinitionRequest, string) {
	args := request.GetArguments()
	updateRequest := &linode.UpdateAlertDefinitionRequest{}

	var fieldsSet int

	if rawLabel, exists := args[monitorAlertDefinitionLabelParam]; exists {
		label, ok := rawLabel.(string)
		if !ok || strings.TrimSpace(label) == "" {
			return nil, monitorAlertDefinitionLabelParam + " must be a non-empty string"
		}

		label = strings.TrimSpace(label)
		updateRequest.Label = &label
		fieldsSet++
	}

	if _, exists := args[monitorAlertDefinitionSeverityParam]; exists {
		severity, validationMessage := monitorAlertDefinitionSeverityFromArgs(args)
		if validationMessage != "" {
			return nil, validationMessage
		}

		updateRequest.Severity = &severity
		fieldsSet++
	}

	statusSet, validationMessage := setMonitorAlertDefinitionUpdateStatus(args, updateRequest)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if statusSet {
		fieldsSet++
	}

	if _, exists := args[monitorAlertDefinitionRuleCriteriaParam]; exists {
		ruleCriteria, validationMessage := objectArgument(args, monitorAlertDefinitionRuleCriteriaParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		updateRequest.RuleCriteria = ruleCriteria
		fieldsSet++
	}

	if _, exists := args[monitorAlertDefinitionTriggerConditionsParam]; exists {
		triggerConditions, validationMessage := objectArgument(args, monitorAlertDefinitionTriggerConditionsParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		updateRequest.TriggerConditions = triggerConditions
		fieldsSet++
	}

	if _, exists := args[monitorAlertDefinitionChannelIDsParam]; exists {
		channelIDs, validationMessage := intArrayArgument(args, monitorAlertDefinitionChannelIDsParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		updateRequest.ChannelIDs = channelIDs
		fieldsSet++
	}

	if rawDescription, exists := args[monitorAlertDefinitionDescriptionParam]; exists {
		description, ok := rawDescription.(string)
		if !ok {
			return nil, monitorAlertDefinitionDescriptionParam + " must be a string"
		}

		updateRequest.Description = &description
		fieldsSet++
	}

	if _, exists := args[monitorAlertDefinitionEntityIDsParam]; exists {
		entityIDs, validationMessage := optionalStringArrayArgument(args, monitorAlertDefinitionEntityIDsParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		if len(entityIDs) == 0 {
			return nil, errMonitorAlertDefinitionEntityIDs
		}

		updateRequest.EntityIDs = entityIDs
		fieldsSet++
	}

	if fieldsSet == 0 {
		return nil, errMonitorAlertDefinitionUpdateEmpty
	}

	return updateRequest, ""
}

func setMonitorAlertDefinitionUpdateStatus(args map[string]any, updateRequest *linode.UpdateAlertDefinitionRequest) (bool, string) {
	rawStatus, exists := args[monitorAlertDefinitionStatusParam]
	if !exists {
		return false, ""
	}

	status, ok := rawStatus.(string)
	if !ok {
		return false, errMonitorAlertDefinitionStatus
	}

	status = strings.TrimSpace(status)
	if status != monitorAlertDefinitionStatusEnabled && status != monitorAlertDefinitionStatusDisabled {
		return false, errMonitorAlertDefinitionStatus
	}

	updateRequest.Status = &status

	return true, ""
}

func updateMonitorServiceAlertDefinition(ctx context.Context, client *linode.Client, serviceType string, alertID int, request *linode.UpdateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, string) {
	definition, err := client.UpdateMonitorServiceAlertDefinitionProto(ctx, serviceType, alertID, request)
	if err != nil {
		return nil, err.Error()
	}

	return definition, ""
}

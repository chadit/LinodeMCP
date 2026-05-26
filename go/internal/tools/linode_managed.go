package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	managedContactsPageSizeMin             = 25
	managedContactsPageSizeMax             = 500
	managedServiceLabelParam               = "label"
	managedServiceTypeParam                = "service_type"
	managedServiceAddressParam             = "address"
	managedServiceTimeoutParam             = "timeout"
	managedServiceBodyParam                = "body"
	managedServiceConsultationParam        = "consultation_group"
	managedServiceCredentialsParam         = "credentials"
	managedServiceNotesParam               = "notes"
	managedServiceRegionParam              = "region"
	managedServiceTimeoutMin               = 1
	managedServiceTimeoutMax               = 255
	errManagedServiceType                  = "service_type must be url or tcp"
	errManagedServiceTimeout               = "timeout must be an integer between 1 and 255"
	managedLinodeSettingsIDParam           = "linode_id"
	errManagedLinodeSettingsIDPositive     = "linode_id must be a positive integer"
	maxManagedLinodeSettingsIDFromJSON     = 9007199254740991
	managedContactGetIDParam               = "contact_id"
	errManagedContactGetIDPositive         = "contact_id must be a positive integer"
	maxManagedContactGetIDFromJSON         = 9007199254740991
	managedContactUpdateIDParam            = "contact_id"
	managedContactUpdateNameParam          = "name"
	managedContactUpdateEmailParam         = "email"
	managedContactUpdateGroupParam         = "group"
	managedContactUpdatePhone1Param        = "phone_primary"
	managedContactUpdatePhone2Param        = "phone_secondary"
	managedContactDeleteIDParam            = "contact_id"
	managedContactDeleteIDMessage          = "contact_id must be a positive integer"
	managedIssueGetIDParam                 = "issue_id"
	errManagedIssueGetIDPositive           = "issue_id must be a positive integer"
	maxManagedIssueGetIDFromJSON           = 9007199254740991
	managedIssuesPageSizeMin               = 25
	managedIssuesPageSizeMax               = 500
	managedServiceGetIDParam               = "service_id"
	managedServiceDeleteIDParam            = "service_id"
	errManagedServiceGetIDPositive         = "service_id must be a positive integer"
	errManagedServiceUpdateFields          = "at least one managed service field is required"
	maxManagedServiceGetIDFromJSON         = 9007199254740991
	managedServicesPageSizeMin             = 25
	managedServicesPageSizeMax             = 500
	managedLinodeSettingsPageSizeMin       = 25
	managedLinodeSettingsPageSizeMax       = 500
	managedLinodeSettingsUpdateIDParam     = "linode_id"
	managedLinodeSettingsUpdateAccessParam = "ssh_access"
	managedLinodeSettingsUpdateIPParam     = "ssh_ip"
	managedLinodeSettingsUpdatePortParam   = "ssh_port"
	managedLinodeSettingsUpdateUserParam   = "ssh_user"
	managedLinodeSettingsUpdateIDMessage   = "linode_id must be a positive integer"
	managedLinodeSettingsUpdateSSHMessage  = "at least one mutable SSH setting is required"
	managedLinodeSettingsUpdatePortMessage = "ssh_port must be an integer between 1 and 65535"
)

// NewLinodeManagedServiceCreateTool creates a tool for creating a Managed service monitor.
func NewLinodeManagedServiceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_service_create",
		"Creates a Managed service monitor.",
		[]mcp.ToolOption{
			mcp.WithString(managedServiceLabelParam, mcp.Required(), mcp.Description("Label for the Managed service.")),
			mcp.WithString(managedServiceTypeParam, mcp.Required(), mcp.Description("Monitor type: url or tcp.")),
			mcp.WithString(managedServiceAddressParam, mcp.Required(), mcp.Description("URL or address monitored by the service.")),
			mcp.WithNumber(managedServiceTimeoutParam, mcp.Required(), mcp.Description("Timeout in seconds, 1-255.")),
			mcp.WithString(managedServiceBodyParam, mcp.Description("Expected response body text for URL monitors.")),
			mcp.WithString(managedServiceConsultationParam, mcp.Description("Managed contact group to consult when an issue is detected.")),
			mcp.WithArray(managedServiceCredentialsParam, mcp.Description("Managed credential IDs used when resolving service issues.")),
			mcp.WithString(managedServiceNotesParam, mcp.Description("Notes for responders.")),
			mcp.WithString(managedServiceRegionParam, mcp.Description("Region for private IP services.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Managed service creation.")),
		},
		handleLinodeManagedServiceCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedLinodeSettingsGetTool creates a tool for retrieving one Linode's Managed settings.
func NewLinodeManagedLinodeSettingsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_linode_settings_get",
		"Gets Managed service settings for one Linode by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedLinodeSettingsIDParam, mcp.Required(),
				mcp.Description("Linode ID whose Managed settings should be retrieved.")),
		},
		handleLinodeManagedLinodeSettingsGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedContactGetTool creates a tool for retrieving one managed contact.
func NewLinodeManagedContactGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_contact_get",
		"Gets one Linode Managed contact by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedContactGetIDParam, mcp.Required(),
				mcp.Description("Managed contact ID to retrieve.")),
		},
		handleLinodeManagedContactGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedContactDeleteTool creates a tool for deleting one Managed contact.
func NewLinodeManagedContactDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_contact_delete",
		"Deletes a contact configured for Linode Managed service alerts.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedContactDeleteIDParam, mcp.Required(), mcp.Description("The Managed contact ID to delete.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Managed contact deletion.")),
		},
		handleLinodeManagedContactDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

// NewLinodeManagedContactsTool creates a tool for listing Managed contacts.
func NewLinodeManagedContactsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_contacts",
		"Lists contacts configured for Linode Managed service alerts.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeManagedContactsRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedLinodeSettingsTool creates a tool for listing Managed Linode settings.
func NewLinodeManagedLinodeSettingsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_linode_settings",
		"Lists Managed service settings for Linodes on the account.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeManagedLinodeSettingsRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedLinodeSettingsUpdateTool creates a tool for updating Managed settings for one Linode.
func NewLinodeManagedLinodeSettingsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_linode_settings_update",
		"Updates Managed service SSH settings for one Linode.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedLinodeSettingsUpdateIDParam, mcp.Required(), mcp.Description("The numeric Linode ID whose Managed settings should be updated.")),
			mcp.WithBoolean(managedLinodeSettingsUpdateAccessParam, mcp.Description("Whether Managed service responders may access the Linode over SSH.")),
			mcp.WithString(managedLinodeSettingsUpdateIPParam, mcp.Description("The IP address Managed service responders should use for SSH access.")),
			mcp.WithNumber(managedLinodeSettingsUpdatePortParam, mcp.Description("The SSH port Managed service responders should use, between 1 and 65535.")),
			mcp.WithString(managedLinodeSettingsUpdateUserParam, mcp.Description("The SSH username Managed service responders should use.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm updating Managed Linode settings.")),
		},
		handleLinodeManagedLinodeSettingsUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceDeleteTool creates a tool for deleting one Managed service monitor.
func NewLinodeManagedServiceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_service_delete",
		"Deletes a service monitored by Linode Managed.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedServiceDeleteIDParam, mcp.Required(), mcp.Description("The Managed service monitor ID to delete.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Managed service deletion.")),
		},
		handleLinodeManagedServiceDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

// NewLinodeManagedServiceDisableTool creates a tool for disabling one Managed service monitor.
func NewLinodeManagedServiceDisableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_service_disable",
		"Disables monitoring for a Linode Managed service.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedServiceGetIDParam, mcp.Required(), mcp.Description("The Managed service monitor ID to disable.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm disabling Managed service monitoring.")),
		},
		handleLinodeManagedServiceDisableRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceEnableTool creates a tool for enabling one Managed service monitor.
func NewLinodeManagedServiceEnableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_service_enable",
		"Enables monitoring for a Linode Managed service.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedServiceGetIDParam, mcp.Required(), mcp.Description("The Managed service monitor ID to enable.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm enabling Managed service monitoring.")),
		},
		handleLinodeManagedServiceEnableRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceGetTool creates a tool for retrieving one Managed service.
func NewLinodeManagedServiceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_service_get",
		"Gets one service monitored by Linode Managed by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedServiceGetIDParam, mcp.Required(),
				mcp.Description("Managed service monitor ID to retrieve.")),
		},
		handleLinodeManagedServiceGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedServiceUpdateTool creates a tool for updating one Managed service.
func NewLinodeManagedServiceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_service_update",
		"Updates a service monitored by Linode Managed.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedServiceGetIDParam, mcp.Required(), mcp.Description("The numeric Managed service monitor ID to update.")),
			mcp.WithString(managedServiceLabelParam, mcp.Description("Updated label for the Managed service.")),
			mcp.WithString(managedServiceTypeParam, mcp.Description("Updated monitor type: url or tcp.")),
			mcp.WithString(managedServiceAddressParam, mcp.Description("Updated URL or address monitored by the service.")),
			mcp.WithNumber(managedServiceTimeoutParam, mcp.Description("Updated timeout in seconds, 1-255.")),
			mcp.WithString(managedServiceBodyParam, mcp.Description("Updated expected response body text for URL monitors.")),
			mcp.WithString(managedServiceConsultationParam, mcp.Description("Updated Managed contact group to consult when an issue is detected.")),
			mcp.WithArray(managedServiceCredentialsParam, mcp.Description("Updated Managed credential IDs used when resolving service issues.")),
			mcp.WithString(managedServiceNotesParam, mcp.Description("Updated notes for responders.")),
			mcp.WithString(managedServiceRegionParam, mcp.Description("Updated region for private IP services.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Managed service update.")),
		},
		handleLinodeManagedServiceUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServicesTool creates a tool for listing Managed services.
func NewLinodeManagedServicesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_services",
		"Lists services monitored by Linode Managed.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeManagedServicesRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedIssueGetTool creates a tool for retrieving one Managed issue.
func NewLinodeManagedIssueGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_issue_get",
		"Gets one issue detected by Linode Managed service monitors.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedIssueGetIDParam, mcp.Required(),
				mcp.Description("Managed issue ID to retrieve.")),
		},
		handleLinodeManagedIssueGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedIssuesTool creates a tool for listing Managed issues.
func NewLinodeManagedIssuesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_issues",
		"Lists recent and ongoing issues detected by Linode Managed service monitors.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeManagedIssuesRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedContactUpdateTool creates a tool for updating a Managed contact.
func NewLinodeManagedContactUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_contact_update",
		"Updates a contact configured for Linode Managed service alerts.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedContactUpdateIDParam, mcp.Required(), mcp.Description("The numeric Managed contact ID to update.")),
			mcp.WithString(managedContactUpdateNameParam, mcp.Description("Updated contact name.")),
			mcp.WithString(managedContactUpdateEmailParam, mcp.Description("Updated contact email address.")),
			mcp.WithString(managedContactUpdateGroupParam, mcp.Description("Updated contact group.")),
			mcp.WithString(managedContactUpdatePhone1Param, mcp.Description("Updated primary phone number.")),
			mcp.WithString(managedContactUpdatePhone2Param, mcp.Description("Updated secondary phone number.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm updating the Managed contact.")),
		},
		handleLinodeManagedContactUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeManagedLinodeSettingsGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := managedLinodeSettingsIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, getFailure := client.GetManagedLinodeSettings(ctx, linodeID)
	if getFailure == nil {
		return MarshalToolResponse(settings)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_linode_settings_get: " + getFailure.Error()), nil
}

func managedLinodeSettingsIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[managedLinodeSettingsIDParam]
	if !exists {
		return 0, "linode_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 || value > maxManagedLinodeSettingsIDFromJSON {
			return 0, errManagedLinodeSettingsIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxManagedLinodeSettingsIDFromJSON || value != float64(int64(value)) {
			return 0, errManagedLinodeSettingsIDPositive
		}

		return int(value), ""
	default:
		return 0, errManagedLinodeSettingsIDPositive
	}
}

func handleLinodeManagedContactGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	contactID, validationMessage := managedContactIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	contact, getFailure := client.GetManagedContact(ctx, contactID)
	if getFailure == nil {
		return MarshalToolResponse(contact)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_contact_get: " + getFailure.Error()), nil
}

func handleLinodeManagedContactDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This deletes a Managed contact. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	contactID, validationMessage := managedContactDeleteIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if failureMessage := deleteManagedContactErrorMessage(ctx, client, contactID); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return mcp.NewToolResultText("Managed contact deleted successfully"), nil
}

func deleteManagedContactErrorMessage(ctx context.Context, client *linode.Client, contactID int) string {
	if err := client.DeleteManagedContact(ctx, contactID); err != nil {
		return "Failed to delete linode_managed_contact_delete: " + err.Error()
	}

	return ""
}

func managedContactDeleteIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return managedContactIDFromToolWithMissingMessage(request, managedContactDeleteIDMessage)
}

func managedContactIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return managedContactIDFromToolWithMissingMessage(request, "contact_id is required")
}

func managedContactIDFromToolWithMissingMessage(request *mcp.CallToolRequest, missingMessage string) (int, string) {
	raw, exists := request.GetArguments()[managedContactGetIDParam]
	if !exists {
		return 0, missingMessage
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 || value > maxManagedContactGetIDFromJSON {
			return 0, errManagedContactGetIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxManagedContactGetIDFromJSON || value != float64(int64(value)) {
			return 0, errManagedContactGetIDPositive
		}

		return int(value), ""
	default:
		return 0, errManagedContactGetIDPositive
	}
}

func handleLinodeManagedContactsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := managedContactsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	contacts, listFailure := client.ListManagedContacts(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(contacts)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_contacts: " + listFailure.Error()), nil
}

func managedContactsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", managedContactsPageSizeMin, managedContactsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeManagedLinodeSettingsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := managedLinodeSettingsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, listFailure := client.ListManagedLinodeSettings(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(settings)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_linode_settings: " + listFailure.Error()), nil
}

func managedLinodeSettingsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", managedLinodeSettingsPageSizeMin, managedLinodeSettingsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeManagedLinodeSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates Managed Linode settings. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, validationMessage := managedLinodeSettingsUpdateIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateReq, validationMessage := managedLinodeSettingsUpdateFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, updateFailure := client.UpdateManagedLinodeSettings(ctx, linodeID, *updateReq)
	if updateFailure != nil {
		return mcp.NewToolResultError(managedLinodeSettingsUpdateFailureMessage(linodeID, updateFailure)), nil
	}

	return MarshalToolResponse(struct {
		Message  string                        `json:"message"`
		Settings *linode.ManagedLinodeSettings `json:"settings"`
	}{
		Message:  fmt.Sprintf("Managed Linode settings for Linode %d updated successfully", linodeID),
		Settings: settings,
	})
}

func managedLinodeSettingsUpdateIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[managedLinodeSettingsUpdateIDParam]
	if !exists {
		return 0, managedLinodeSettingsUpdateIDMessage
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 || value > maxManagedLinodeSettingsIDFromJSON {
			return 0, errManagedLinodeSettingsIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxManagedLinodeSettingsIDFromJSON || value != float64(int64(value)) {
			return 0, errManagedLinodeSettingsIDPositive
		}

		return int(value), ""
	default:
		return 0, errManagedLinodeSettingsIDPositive
	}
}

func managedLinodeSettingsUpdateFromTool(request *mcp.CallToolRequest) (*linode.UpdateManagedLinodeSettingsRequest, string) {
	args := request.GetArguments()
	ssh := &linode.UpdateManagedLinodeSettingsSSH{}

	var fields int

	if raw, exists := args[managedLinodeSettingsUpdateAccessParam]; exists {
		value, ok := raw.(bool)
		if !ok {
			return nil, managedLinodeSettingsUpdateAccessParam + " must be a boolean"
		}

		ssh.Access = &value
		fields++
	}

	if validationMessage := managedLinodeSettingsUpdateOptionalString(args, managedLinodeSettingsUpdateIPParam, &ssh.IP, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedLinodeSettingsUpdateOptionalPort(args, &ssh.Port, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedLinodeSettingsUpdateOptionalString(args, managedLinodeSettingsUpdateUserParam, &ssh.User, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if fields == 0 {
		return nil, managedLinodeSettingsUpdateSSHMessage
	}

	return &linode.UpdateManagedLinodeSettingsRequest{SSH: ssh}, ""
}

func managedLinodeSettingsUpdateOptionalString(args map[string]any, name string, target **string, fields *int) string {
	raw, exists := args[name]
	if !exists {
		return ""
	}

	value, ok := raw.(string)
	if !ok {
		return name + " must be a string"
	}

	*target = &value
	(*fields)++

	return ""
}

func managedLinodeSettingsUpdateOptionalPort(args map[string]any, target **int, fields *int) string {
	raw, exists := args[managedLinodeSettingsUpdatePortParam]
	if !exists {
		return ""
	}

	var port int

	switch value := raw.(type) {
	case int:
		port = value
	case int64:
		if value < 1 || value > 65535 {
			return managedLinodeSettingsUpdatePortMessage
		}

		port = int(value)
	case float64:
		if value < 1 || value > 65535 || value != float64(int64(value)) {
			return managedLinodeSettingsUpdatePortMessage
		}

		port = int(value)
	default:
		return managedLinodeSettingsUpdatePortMessage
	}

	if port < 1 || port > 65535 {
		return managedLinodeSettingsUpdatePortMessage
	}

	*target = &port
	(*fields)++

	return ""
}

func managedLinodeSettingsUpdateFailureMessage(linodeID int, err error) string {
	return "Failed to update linode_managed_linode_settings_update " + strconv.Itoa(linodeID) + ": " + err.Error()
}

func handleLinodeManagedServiceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceID, validationMessage := managedServiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	service, getFailure := client.GetManagedService(ctx, serviceID)
	if getFailure == nil {
		return MarshalToolResponse(service)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_service_get: " + getFailure.Error()), nil
}

func managedServiceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[managedServiceGetIDParam]
	if !exists {
		return 0, "service_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 || value > maxManagedServiceGetIDFromJSON {
			return 0, errManagedServiceGetIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxManagedServiceGetIDFromJSON || value != float64(int64(value)) {
			return 0, errManagedServiceGetIDPositive
		}

		return int(value), ""
	default:
		return 0, errManagedServiceGetIDPositive
	}
}

func handleLinodeManagedServiceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This deletes a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	serviceID, validationMessage := managedServiceDeleteIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if failureMessage := deleteManagedServiceErrorMessage(ctx, client, serviceID); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return mcp.NewToolResultText("Managed service deleted successfully"), nil
}

func deleteManagedServiceErrorMessage(ctx context.Context, client *linode.Client, serviceID int) string {
	if err := client.DeleteManagedService(ctx, serviceID); err != nil {
		return "Failed to delete linode_managed_service_delete: " + err.Error()
	}

	return ""
}

func managedServiceDeleteIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return managedServiceIDFromTool(request)
}

func handleLinodeManagedServiceDisableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This disables a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	serviceID, validationMessage := managedServiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if failureMessage := disableManagedServiceErrorMessage(ctx, client, serviceID); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return mcp.NewToolResultText("Managed service disabled successfully"), nil
}

func disableManagedServiceErrorMessage(ctx context.Context, client *linode.Client, serviceID int) string {
	if err := client.DisableManagedService(ctx, serviceID); err != nil {
		return "Failed to disable linode_managed_service_disable: " + err.Error()
	}

	return ""
}

func handleLinodeManagedServiceEnableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This enables a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	serviceID, validationMessage := managedServiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if failureMessage := enableManagedServiceErrorMessage(ctx, client, serviceID); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return mcp.NewToolResultText("Managed service enabled successfully"), nil
}

func enableManagedServiceErrorMessage(ctx context.Context, client *linode.Client, serviceID int) string {
	if err := client.EnableManagedService(ctx, serviceID); err != nil {
		return "Failed to enable linode_managed_service_enable: " + err.Error()
	}

	return ""
}

func handleLinodeManagedServiceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	serviceID, validationMessage := managedServiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateRequest, validationMessage := managedServiceUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	service, updateFailure := updateManagedServiceForTool(ctx, client, serviceID, updateRequest)
	if updateFailure != "" {
		return mcp.NewToolResultError(updateFailure), nil
	}

	return MarshalToolResponse(service)
}

func updateManagedServiceForTool(ctx context.Context, client *linode.Client, serviceID int, request *linode.UpdateManagedServiceRequest) (*linode.ManagedService, string) {
	service, err := client.UpdateManagedService(ctx, serviceID, request)
	if err != nil {
		return nil, "Failed to update linode_managed_service_update: " + err.Error()
	}

	return service, ""
}

func managedServiceUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateManagedServiceRequest, string) {
	args := request.GetArguments()
	updateRequest := &linode.UpdateManagedServiceRequest{}

	var fields int

	if validationMessage := managedServiceOptionalStringWithCount(request, managedServiceLabelParam, &updateRequest.Label, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if _, exists := args[managedServiceTypeParam]; exists {
		serviceType, validationMessage := managedServiceOptionalNonEmptyString(request, managedServiceTypeParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		if serviceType != "url" && serviceType != "tcp" {
			return nil, errManagedServiceType
		}

		updateRequest.ServiceType = &serviceType
		fields++
	}

	if validationMessage := managedServiceOptionalStringWithCount(request, managedServiceAddressParam, &updateRequest.Address, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if _, exists := args[managedServiceTimeoutParam]; exists {
		timeout, validationMessage := boundedIntArgument(request, managedServiceTimeoutParam, managedServiceTimeoutMin, managedServiceTimeoutMax, errManagedServiceTimeout)
		if validationMessage != "" {
			return nil, validationMessage
		}

		updateRequest.Timeout = &timeout
		fields++
	}

	if validationMessage := managedServiceOptionalStringWithCount(request, managedServiceBodyParam, &updateRequest.Body, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedServiceOptionalStringWithCount(request, managedServiceConsultationParam, &updateRequest.ConsultationGroup, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if raw, exists := args[managedServiceCredentialsParam]; exists {
		credentials, validationMessage := intSliceFromToolArg(raw, managedServiceCredentialsParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		updateRequest.Credentials = &credentials
		fields++
	}

	if validationMessage := managedServiceOptionalStringWithCount(request, managedServiceNotesParam, &updateRequest.Notes, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedServiceOptionalStringWithCount(request, managedServiceRegionParam, &updateRequest.Region, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if fields == 0 {
		return nil, errManagedServiceUpdateFields
	}

	return updateRequest, ""
}

func managedServiceOptionalNonEmptyString(request *mcp.CallToolRequest, name string) (string, string) {
	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return "", validationMessage
	}

	if value == "" {
		return "", name + " must be a non-empty string"
	}

	return value, ""
}

func managedServiceOptionalStringWithCount(request *mcp.CallToolRequest, name string, target **string, fields *int) string {
	if _, exists := request.GetArguments()[name]; !exists {
		return ""
	}

	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return validationMessage
	}

	*target = &value
	(*fields)++

	return ""
}

func handleLinodeManagedServicesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := managedServicesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	services, listFailure := client.ListManagedServices(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(services)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_services: " + listFailure.Error()), nil
}

func managedServicesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", managedServicesPageSizeMin, managedServicesPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeManagedIssueGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	issueID, validationMessage := managedIssueIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	issue, getFailure := client.GetManagedIssue(ctx, issueID)
	if getFailure == nil {
		return MarshalToolResponse(issue)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_issue_get: " + getFailure.Error()), nil
}

func managedIssueIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[managedIssueGetIDParam]
	if !exists {
		return 0, "issue_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 || value > maxManagedIssueGetIDFromJSON {
			return 0, errManagedIssueGetIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxManagedIssueGetIDFromJSON || value != float64(int64(value)) {
			return 0, errManagedIssueGetIDPositive
		}

		return int(value), ""
	default:
		return 0, errManagedIssueGetIDPositive
	}
}

func handleLinodeManagedIssuesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := managedIssuesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	issues, listFailure := client.ListManagedIssues(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(issues)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_issues: " + listFailure.Error()), nil
}

func managedIssuesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", managedIssuesPageSizeMin, managedIssuesPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeManagedServiceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	createRequest, validationMessage := managedServiceCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	service, createFailureMessage := createManagedServiceForTool(ctx, client, createRequest)
	if createFailureMessage != "" {
		return mcp.NewToolResultError(createFailureMessage), nil
	}

	return MarshalToolResponse(service)
}

func createManagedServiceForTool(ctx context.Context, client *linode.Client, request *linode.CreateManagedServiceRequest) (*linode.ManagedService, string) {
	service, err := client.CreateManagedService(ctx, request)
	if err != nil {
		return nil, "Failed to create linode_managed_service_create: " + err.Error()
	}

	return service, ""
}

func managedServiceCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateManagedServiceRequest, string) {
	label, validationMessage := managedServiceRequiredString(request, managedServiceLabelParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	serviceType, validationMessage := managedServiceRequiredString(request, managedServiceTypeParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if serviceType != "url" && serviceType != "tcp" {
		return nil, errManagedServiceType
	}

	address, validationMessage := managedServiceRequiredString(request, managedServiceAddressParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	timeout, validationMessage := boundedIntArgument(request, managedServiceTimeoutParam, managedServiceTimeoutMin, managedServiceTimeoutMax, errManagedServiceTimeout)
	if validationMessage != "" {
		return nil, validationMessage
	}

	createRequest := &linode.CreateManagedServiceRequest{
		Label:       label,
		ServiceType: serviceType,
		Address:     address,
		Timeout:     timeout,
	}

	if validationMessage := managedServiceOptionalString(request, managedServiceBodyParam, &createRequest.Body); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedServiceOptionalString(request, managedServiceConsultationParam, &createRequest.ConsultationGroup); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedServiceOptionalString(request, managedServiceNotesParam, &createRequest.Notes); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedServiceOptionalString(request, managedServiceRegionParam, &createRequest.Region); validationMessage != "" {
		return nil, validationMessage
	}

	if raw, exists := request.GetArguments()[managedServiceCredentialsParam]; exists {
		credentials, validationMessage := intSliceFromToolArg(raw, managedServiceCredentialsParam)
		if validationMessage != "" {
			return nil, validationMessage
		}

		createRequest.Credentials = credentials
	}

	return createRequest, ""
}

func managedServiceRequiredString(request *mcp.CallToolRequest, name string) (string, string) {
	value, validationMessage := stringArgument(request, name, true)
	if validationMessage != "" {
		return "", validationMessage
	}

	if value == "" {
		return "", name + " must be a non-empty string"
	}

	return value, ""
}

func managedServiceOptionalString(request *mcp.CallToolRequest, name string, target **string) string {
	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()[name]; !exists {
		return ""
	}

	*target = &value

	return ""
}

func handleLinodeManagedContactUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates a Managed contact. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	contactID, ok := getPositiveIntArgument(request, managedContactUpdateIDParam)
	if !ok {
		return mcp.NewToolResultError("contact_id must be a positive integer"), nil
	}

	updateReq, validationMessage := managedContactUpdateFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	contact, updateFailure := client.UpdateManagedContact(ctx, contactID, *updateReq)
	if updateFailure != nil {
		return mcp.NewToolResultError(managedContactUpdateFailureMessage(contactID, updateFailure)), nil
	}

	return MarshalToolResponse(struct {
		Message string                 `json:"message"`
		Contact *linode.ManagedContact `json:"contact"`
	}{
		Message: fmt.Sprintf("Managed contact %d updated successfully", contactID),
		Contact: contact,
	})
}

func managedContactUpdateFromTool(request *mcp.CallToolRequest) (*linode.UpdateManagedContactRequest, string) {
	req := &linode.UpdateManagedContactRequest{}

	var fields int

	if validationMessage := managedContactOptionalString(request, managedContactUpdateNameParam, &req.Name, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedContactOptionalString(request, managedContactUpdateEmailParam, &req.Email, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedContactOptionalString(request, managedContactUpdateGroupParam, &req.Group, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	phone := &linode.UpdateManagedContactPhone{}

	var phoneFields int

	if validationMessage := managedContactOptionalString(request, managedContactUpdatePhone1Param, &phone.Primary, &phoneFields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedContactOptionalString(request, managedContactUpdatePhone2Param, &phone.Secondary, &phoneFields); validationMessage != "" {
		return nil, validationMessage
	}

	if phoneFields > 0 {
		req.Phone = phone
		fields++
	}

	if fields == 0 {
		return nil, "at least one mutable contact field is required"
	}

	return req, ""
}

func managedContactOptionalString(request *mcp.CallToolRequest, name string, target **string, fields *int) string {
	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()[name]; !exists {
		return ""
	}

	*target = &value
	(*fields)++

	return ""
}

func managedContactUpdateFailureMessage(contactID int, err error) string {
	return "Failed to update linode_managed_contact " + strconv.Itoa(contactID) + ": " + err.Error()
}

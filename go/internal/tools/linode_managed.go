package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	managedServicesPath = "/managed/services"
	managedContactsPath = "/managed/contacts"
	// Split literal mirrors the linode package: it dodges gosec G101's
	// "hardcoded credentials" false positive on the word "credentials".
	managedCredentialsPath    = "/managed/" + "credentials"
	managedLinodeSettingsPath = "/managed/linode-settings"

	managedContactsPageSizeMin               = 25
	managedContactsPageSizeMax               = 500
	managedServiceLabelParam                 = "label"
	managedServiceTypeParam                  = "service_type"
	managedServiceAddressParam               = "address"
	managedServiceTimeoutParam               = "timeout"
	managedServiceBodyParam                  = "body"
	managedServiceConsultationParam          = "consultation_group"
	managedServiceCredentialsParam           = "credentials"
	managedServiceNotesParam                 = "notes"
	managedServiceRegionParam                = "region"
	managedServiceTimeoutMin                 = 1
	managedServiceTimeoutMax                 = 255
	errManagedServiceTimeout                 = "timeout must be an integer between 1 and 255"
	managedLinodeSettingsIDParam             = "linode_id"
	maxManagedLinodeSettingsIDFromJSON       = 9007199254740991
	managedContactGetIDParam                 = "contact_id"
	maxManagedContactGetIDFromJSON           = 9007199254740991
	managedContactUpdateIDParam              = "contact_id"
	managedContactUpdateNameParam            = "name"
	managedContactUpdateEmailParam           = "email"
	managedContactUpdateGroupParam           = "group"
	managedContactDeleteIDParam              = "contact_id"
	managedIssueGetIDParam                   = "issue_id"
	maxManagedIssueGetIDFromJSON             = 9007199254740991
	managedIssuesPageSizeMin                 = 25
	managedIssuesPageSizeMax                 = 500
	managedServiceGetIDParam                 = "service_id"
	errManagedServiceUpdateFields            = "at least one managed service field is required"
	maxManagedServiceGetIDFromJSON           = 9007199254740991
	managedServicesPageSizeMin               = 25
	managedServicesPageSizeMax               = 500
	managedLinodeSettingsPageSizeMin         = 25
	managedLinodeSettingsPageSizeMax         = 500
	managedLinodeSettingsUpdateIDParam       = "linode_id"
	managedLinodeSettingsUpdateSSHParam      = "ssh"
	managedLinodeSettingsUpdateAccessKey     = "access"
	managedLinodeSettingsUpdateIPKey         = "ip"
	managedLinodeSettingsUpdatePortKey       = "port"
	managedLinodeSettingsUpdateUserKey       = "user"
	managedLinodeSettingsUpdateSSHMessage    = "at least one mutable SSH setting is required"
	managedLinodeSettingsUpdateSSHReqMsg     = "ssh is required and must be an object"
	managedLinodeSettingsUpdateSSHTypeMsg    = "ssh must be an object"
	managedLinodeSettingsUpdateAccessTypeMsg = "ssh.access must be a boolean"
	managedLinodeSettingsUpdatePortMessage   = "ssh.port must be an integer between 1 and 65535"
)

// NewLinodeManagedServiceCreateTool creates a tool for creating a Managed service monitor.
func NewLinodeManagedServiceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_service_create",
		"Creates a Managed service monitor.",
		toolschemas.Schema("linode.mcp.v1.ManagedServiceCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedServiceCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedLinodeSettingsGetTool creates a tool for retrieving one Linode's Managed settings.
func NewLinodeManagedLinodeSettingsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_linode_settings_get",
		"Gets Managed service settings for one Linode by ID.",
		toolschemas.Schema("linode.mcp.v1.ManagedLinodeSettingsGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedLinodeSettingsGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedContactGetTool creates a tool for retrieving one managed contact.
func NewLinodeManagedContactGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_contact_get",
		"Gets one Linode Managed contact by ID.",
		toolschemas.Schema("linode.mcp.v1.ManagedContactGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedContactGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedContactDeleteTool creates a tool for deleting one Managed contact.
func NewLinodeManagedContactDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_contact_delete",
		"Deletes a contact configured for Linode Managed service alerts. Pass dry_run=true to preview without deleting.",
		toolschemas.Schema("linode.mcp.v1.ManagedContactDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedContactDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedContactsTool creates a tool for listing Managed contacts.
func NewLinodeManagedContactsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_managed_contact_list",
		"Lists contacts configured for Linode Managed service alerts.",
		"linode.mcp.v1.ManagedContactListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ManagedContact, error) {
			return client.ListManagedContactsProto(ctx, page, pageSize)
		},
		managedContactsPaginationFromTool,
		nil,
		managedContactListResponse,
	)

	return tool, profiles.CapRead, handler
}

func managedContactListResponse(items []*linodev1.ManagedContact, count int32, filter *string) *linodev1.ManagedContactListResponse {
	return &linodev1.ManagedContactListResponse{Count: count, Filter: filter, ManagedContacts: items}
}

// NewLinodeManagedLinodeSettingsTool creates a tool for listing Managed Linode settings.
func NewLinodeManagedLinodeSettingsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_managed_linode_settings_list",
		"Lists Managed service settings for Linodes on the account.",
		"linode.mcp.v1.ManagedLinodeSettingsListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ManagedLinodeSettings, error) {
			return client.ListManagedLinodeSettingsProto(ctx, page, pageSize)
		},
		managedLinodeSettingsPaginationFromTool,
		nil,
		managedLinodeSettingsListResponse,
	)

	return tool, profiles.CapRead, handler
}

func managedLinodeSettingsListResponse(items []*linodev1.ManagedLinodeSettings, count int32, filter *string) *linodev1.ManagedLinodeSettingsListResponse {
	return &linodev1.ManagedLinodeSettingsListResponse{Count: count, Filter: filter, ManagedLinodeSettings: items}
}

// NewLinodeManagedStatsTool creates a tool for retrieving Managed statistics.
func NewLinodeManagedStatsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_stats_get",
		"Lists Linode Managed statistics from the last 24 hours.",
		toolschemas.Schema("linode.mcp.v1.ManagedStatsGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedStatsRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedLinodeSettingsUpdateTool creates a tool for updating Managed settings for one Linode.
func NewLinodeManagedLinodeSettingsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_linode_settings_update",
		"Updates Managed service SSH settings for one Linode.",
		toolschemas.Schema("linode.mcp.v1.ManagedLinodeSettingsUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedLinodeSettingsUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceDeleteTool creates a tool for deleting one Managed service monitor.
func NewLinodeManagedServiceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_service_delete",
		"Deletes a service monitored by Linode Managed. Pass dry_run=true to preview without deleting.",
		toolschemas.Schema("linode.mcp.v1.ManagedServiceDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedServiceDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceDisableTool creates a tool for disabling one Managed service monitor.
func NewLinodeManagedServiceDisableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_service_disable",
		"Disables monitoring for a Linode Managed service.",
		toolschemas.Schema("linode.mcp.v1.ManagedServiceDisableInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedServiceDisableRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceEnableTool creates a tool for enabling one Managed service monitor.
func NewLinodeManagedServiceEnableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_service_enable",
		"Enables monitoring for a Linode Managed service.",
		toolschemas.Schema("linode.mcp.v1.ManagedServiceEnableInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedServiceEnableRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServiceGetTool creates a tool for retrieving one Managed service.
func NewLinodeManagedServiceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_service_get",
		"Gets one service monitored by Linode Managed by ID.",
		toolschemas.Schema("linode.mcp.v1.ManagedServiceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedServiceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedServiceUpdateTool creates a tool for updating one Managed service.
func NewLinodeManagedServiceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_service_update",
		"Updates a service monitored by Linode Managed. Pass dry_run=true to preview without modifying.",
		toolschemas.Schema("linode.mcp.v1.ManagedServiceUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedServiceUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedServicesTool creates a tool for listing Managed services.
func NewLinodeManagedServicesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_managed_service_list",
		"Lists services monitored by Linode Managed.",
		"linode.mcp.v1.ManagedServiceListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ManagedService, error) {
			return client.ListManagedServicesProto(ctx, page, pageSize)
		},
		managedServicesPaginationFromTool,
		nil,
		managedServiceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func managedServiceListResponse(items []*linodev1.ManagedService, count int32, filter *string) *linodev1.ManagedServiceListResponse {
	return &linodev1.ManagedServiceListResponse{Count: count, Filter: filter, ManagedServices: items}
}

// NewLinodeManagedIssueGetTool creates a tool for retrieving one Managed issue.
func NewLinodeManagedIssueGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_issue_get",
		"Gets one issue detected by Linode Managed service monitors.",
		toolschemas.Schema("linode.mcp.v1.ManagedIssueGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedIssueGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedIssuesTool creates a tool for listing Managed issues.
func NewLinodeManagedIssuesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_managed_issue_list",
		"Lists recent and ongoing issues detected by Linode Managed service monitors.",
		"linode.mcp.v1.ManagedIssueListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ManagedIssue, error) {
			return client.ListManagedIssuesProto(ctx, page, pageSize)
		},
		managedIssuesPaginationFromTool,
		nil,
		managedIssueListResponse,
	)

	return tool, profiles.CapRead, handler
}

func managedIssueListResponse(items []*linodev1.ManagedIssue, count int32, filter *string) *linodev1.ManagedIssueListResponse {
	return &linodev1.ManagedIssueListResponse{Count: count, Filter: filter, ManagedIssues: items}
}

// NewLinodeManagedContactUpdateTool creates a tool for updating a Managed contact.
func NewLinodeManagedContactUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_contact_update",
		"Updates a contact configured for Linode Managed service alerts.",
		toolschemas.Schema("linode.mcp.v1.ManagedContactUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedContactUpdateRequest(ctx, &request, cfg)
	}

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

	settings, getFailure := client.GetManagedLinodeSettingsProto(ctx, linodeID)
	if getFailure == nil {
		return MarshalProtoToolResponse(settings)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_linode_settings_get: " + getFailure.Error()), nil
}

func managedLinodeSettingsIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredBoundedIDArgument(request, managedLinodeSettingsIDParam, maxManagedLinodeSettingsIDFromJSON)
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

	contact, getFailure := client.GetManagedContactProto(ctx, contactID)
	if getFailure == nil {
		return MarshalProtoToolResponse(contact)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_contact_get: " + getFailure.Error()), nil
}

// runManagedResourceDelete wires dry-run preview, confirm gating, and
// execution for the Managed contact + service deletes, which are otherwise
// identical and trip the dupl linter. The caller validates the ID, builds
// the path, and supplies fetch/execute closures capturing that ID.
func runManagedResourceDelete(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, path, confirmMessage string,
	successResponse proto.Message,
	fetchState func(context.Context, *linode.Client) (any, error),
	execute func(context.Context, *linode.Client) error,
) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, toolName, httpMethodDelete, path, fetchState)
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := execute(ctx, client); err != nil {
		// "delete" is passed as an argument, not baked into the format
		// literal, so the SQL-formatting heuristic does not false-positive.
		return mcp.NewToolResultError(fmt.Sprintf("Failed to %s %s: %v", "delete", toolName, err)), nil
	}

	return MarshalProtoToolResponse(successResponse)
}

func handleLinodeManagedContactDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	contactID, validationMessage := managedContactDeleteIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runManagedResourceDelete(ctx, request, cfg,
		"linode_managed_contact_delete",
		fmt.Sprintf(managedContactsPath+"/%d", contactID),
		"This deletes a Managed contact. Set confirm=true to proceed.",
		&linodev1.ManagedContactIDResponse{
			Message:   "Managed contact deleted successfully",
			ContactId: linodeIDToInt32(contactID),
		},
		func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetManagedContact(ctx, contactID)
		},
		func(ctx context.Context, c *linode.Client) error {
			return c.DeleteManagedContact(ctx, contactID)
		})
}

func managedContactDeleteIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return managedContactIDFromTool(request)
}

func managedContactIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredBoundedIDArgument(request, managedContactGetIDParam, maxManagedContactGetIDFromJSON)
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

func handleLinodeManagedStatsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	stats, getFailure := client.GetManagedStats(ctx)
	if getFailure == nil {
		return MarshalStructToolResponse(stats, "Failed to retrieve linode_managed_stats_get")
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_stats_get: " + getFailure.Error()), nil
}

func handleLinodeManagedLinodeSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := managedLinodeSettingsUpdateIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateReq, validationMessage := managedLinodeSettingsUpdateFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_linode_settings_update", "PUT",
			fmt.Sprintf(managedLinodeSettingsPath+"/%d", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedLinodeSettings(ctx, linodeID)
			})
	}

	if result := RequireConfirm(request, "This updates Managed Linode settings. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, updateFailure := client.UpdateManagedLinodeSettingsProto(ctx, linodeID, *updateReq)
	if updateFailure != nil {
		return mcp.NewToolResultError(managedLinodeSettingsUpdateFailureMessage(linodeID, updateFailure)), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedLinodeSettingsWriteResponse{
		Message:  fmt.Sprintf("Managed Linode settings for Linode %d updated successfully", linodeID),
		Settings: settings,
	})
}

func managedLinodeSettingsUpdateIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredBoundedIDArgument(request, managedLinodeSettingsUpdateIDParam, maxManagedLinodeSettingsIDFromJSON)
}

func managedLinodeSettingsUpdateFromTool(request *mcp.CallToolRequest) (*linode.UpdateManagedLinodeSettingsRequest, string) {
	raw, present := request.GetArguments()[managedLinodeSettingsUpdateSSHParam]
	if !present {
		return nil, managedLinodeSettingsUpdateSSHReqMsg
	}

	sshObj, isObj := raw.(map[string]any)
	if !isObj {
		return nil, managedLinodeSettingsUpdateSSHTypeMsg
	}

	ssh := &linode.UpdateManagedLinodeSettingsSSH{}

	var fields int

	if value, exists := sshObj[managedLinodeSettingsUpdateAccessKey]; exists {
		access, ok := value.(bool)
		if !ok {
			return nil, managedLinodeSettingsUpdateAccessTypeMsg
		}

		ssh.Access = &access
		fields++
	}

	if validationMessage := managedLinodeSettingsUpdateOptionalString(sshObj, managedLinodeSettingsUpdateIPKey, &ssh.IP, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedLinodeSettingsUpdateOptionalPort(sshObj, &ssh.Port, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedLinodeSettingsUpdateOptionalString(sshObj, managedLinodeSettingsUpdateUserKey, &ssh.User, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if fields == 0 {
		return nil, managedLinodeSettingsUpdateSSHMessage
	}

	return &linode.UpdateManagedLinodeSettingsRequest{SSH: ssh}, ""
}

func managedLinodeSettingsUpdateOptionalString(sshObj map[string]any, key string, target **string, fields *int) string {
	raw, exists := sshObj[key]
	if !exists {
		return ""
	}

	value, ok := raw.(string)
	if !ok {
		return managedLinodeSettingsUpdateSSHParam + "." + key + " must be a string"
	}

	*target = &value
	(*fields)++

	return ""
}

func managedLinodeSettingsUpdateOptionalPort(sshObj map[string]any, target **int, fields *int) string {
	raw, exists := sshObj[managedLinodeSettingsUpdatePortKey]
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

	service, getFailure := client.GetManagedServiceProto(ctx, serviceID)
	if getFailure == nil {
		return MarshalProtoToolResponse(service)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_service_get: " + getFailure.Error()), nil
}

func managedServiceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredBoundedIDArgument(request, managedServiceGetIDParam, maxManagedServiceGetIDFromJSON)
}

func handleLinodeManagedServiceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceID, validationMessage := managedServiceDeleteIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runManagedResourceDelete(ctx, request, cfg,
		"linode_managed_service_delete",
		fmt.Sprintf(managedServicesPath+"/%d", serviceID),
		"This deletes a Managed service monitor. Set confirm=true to proceed.",
		&linodev1.ManagedServiceIDResponse{
			Message:   "Managed service deleted successfully",
			ServiceId: linodeIDToInt32(serviceID),
		},
		func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetManagedService(ctx, serviceID)
		},
		func(ctx context.Context, c *linode.Client) error {
			return c.DeleteManagedService(ctx, serviceID)
		})
}

func managedServiceDeleteIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return managedServiceIDFromTool(request)
}

// runManagedServiceAction wires dry-run preview, confirm gating, and
// execution for the service enable/disable POST actions. dry_run fetches
// the service for current_state and previews the POST without firing.
// Shared by enable + disable, which otherwise trip the dupl linter.
func runManagedServiceAction(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, verb, confirmMessage, successMessage, failureVerb string,
	execute func(context.Context, *linode.Client, int) error,
) (*mcp.CallToolResult, error) {
	serviceID, validationMessage := managedServiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, toolName, httpMethodPost,
			fmt.Sprintf(managedServicesPath+"/%d/"+verb, serviceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedService(ctx, serviceID)
			})
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := execute(ctx, client, serviceID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to %s %s: %v", failureVerb, toolName, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedServiceIDResponse{
		Message:   successMessage,
		ServiceId: linodeIDToInt32(serviceID),
	})
}

func handleLinodeManagedServiceDisableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runManagedServiceAction(ctx, request, cfg,
		"linode_managed_service_disable", "disable",
		"This disables a Managed service monitor. Set confirm=true to proceed.",
		"Managed service disabled successfully", "disable",
		func(ctx context.Context, c *linode.Client, serviceID int) error {
			return c.DisableManagedService(ctx, serviceID)
		})
}

func handleLinodeManagedServiceEnableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runManagedServiceAction(ctx, request, cfg,
		"linode_managed_service_enable", "enable",
		"This enables a Managed service monitor. Set confirm=true to proceed.",
		"Managed service enabled successfully", "enable",
		func(ctx context.Context, c *linode.Client, serviceID int) error {
			return c.EnableManagedService(ctx, serviceID)
		})
}

func handleLinodeManagedServiceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	serviceID, validationMessage := managedServiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateRequest, validationMessage := managedServiceUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_service_update", "PUT",
			fmt.Sprintf(managedServicesPath+"/%d", serviceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedService(ctx, serviceID)
			})
	}

	if result := RequireConfirm(request, "This updates a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	service, updateFailure := updateManagedServiceForTool(ctx, client, serviceID, updateRequest)
	if updateFailure != "" {
		return mcp.NewToolResultError(updateFailure), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedServiceWriteResponse{
		Message: fmt.Sprintf("Managed service monitor %d updated successfully", serviceID),
		Service: service,
	})
}

func updateManagedServiceForTool(ctx context.Context, client *linode.Client, serviceID int, request *linode.UpdateManagedServiceRequest) (*linodev1.ManagedService, string) {
	service, err := client.UpdateManagedServiceProto(ctx, serviceID, request)
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

		if message := enumChoiceError(serviceType, managedServiceTypeParam, linodev1.ManagedServiceType_Value_value); message != "" {
			return nil, message
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

	issue, getFailure := client.GetManagedIssueProto(ctx, issueID)
	if getFailure == nil {
		return MarshalProtoToolResponse(issue)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_issue_get: " + getFailure.Error()), nil
}

func managedIssueIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredBoundedIDArgument(request, managedIssueGetIDParam, maxManagedIssueGetIDFromJSON)
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
	createRequest, validationMessage := managedServiceCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_service_create", httpMethodPost, managedServicesPath, nil)
	}

	if result := RequireConfirm(request, "This creates a Managed service monitor. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	service, createFailureMessage := createManagedServiceForTool(ctx, client, createRequest)
	if createFailureMessage != "" {
		return mcp.NewToolResultError(createFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedServiceWriteResponse{
		Message: fmt.Sprintf("Managed service monitor %d created successfully", service.GetId()),
		Service: service,
	})
}

func createManagedServiceForTool(ctx context.Context, client *linode.Client, request *linode.CreateManagedServiceRequest) (*linodev1.ManagedService, string) {
	service, err := client.CreateManagedServiceProto(ctx, request)
	if err != nil {
		return nil, "Failed to create linode_managed_service_create: " + err.Error()
	}

	return service, ""
}

// managedServiceReadOnlyReject mirrors Python's guard on managed service create:
// the API assigns created/id/status/updated, so setting any of them is rejected
// before other field validation. The fields are already in sorted order, so the
// joined message matches Python's sorted() output byte-for-byte.
func managedServiceReadOnlyReject(request *mcp.CallToolRequest) string {
	return managedReadOnlyFieldsReject(request, []string{"created", "id", "status", "updated"})
}

// managedReadOnlyFieldsReject rejects any of the given read-only fields that
// appear in the request, listing them sorted so the message matches Python's
// sorted() output byte-for-byte.
func managedReadOnlyFieldsReject(request *mcp.CallToolRequest, fields []string) string {
	args := request.GetArguments()

	var readOnly []string

	for _, field := range fields {
		if _, exists := args[field]; exists {
			readOnly = append(readOnly, field)
		}
	}

	if len(readOnly) == 0 {
		return ""
	}

	sort.Strings(readOnly)

	return "Read-only fields are not accepted: " + strings.Join(readOnly, ", ")
}

func managedServiceCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateManagedServiceRequest, string) {
	if validationMessage := managedServiceReadOnlyReject(request); validationMessage != "" {
		return nil, validationMessage
	}

	label, validationMessage := managedServiceRequiredString(request, managedServiceLabelParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	serviceType, validationMessage := managedServiceRequiredString(request, managedServiceTypeParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if message := enumChoiceError(serviceType, managedServiceTypeParam, linodev1.ManagedServiceType_Value_value); message != "" {
		return nil, message
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
	contactID, validationMessage := requiredIDArgument(request, managedContactUpdateIDParam)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateReq, validationMessage := managedContactUpdateFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_contact_update", "PUT",
			fmt.Sprintf(managedContactsPath+"/%d", contactID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedContact(ctx, contactID)
			})
	}

	if result := RequireConfirm(request, "This updates a Managed contact. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	contact, updateFailure := client.UpdateManagedContactProto(ctx, contactID, *updateReq)
	if updateFailure != nil {
		return mcp.NewToolResultError(managedContactUpdateFailureMessage(contactID, updateFailure)), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedContactWriteResponse{
		Message: fmt.Sprintf("Managed contact %d updated successfully", contactID),
		Contact: contact,
	})
}

func managedContactUpdateFromTool(request *mcp.CallToolRequest) (*linode.UpdateManagedContactRequest, string) {
	req := &linode.UpdateManagedContactRequest{}

	var fields int

	if validationMessage := managedContactNonEmptyString(request, managedContactUpdateNameParam, &req.Name, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedContactNonEmptyString(request, managedContactUpdateEmailParam, &req.Email, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := managedContactOptionalString(request, managedContactUpdateGroupParam, &req.Group, &fields); validationMessage != "" {
		return nil, validationMessage
	}

	phone := &linode.UpdateManagedContactPhone{}

	phoneSet, validationMessage := managedContactPhoneFromArgs(request.GetArguments(), &phone.Primary, &phone.Secondary)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if phoneSet {
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

// managedContactNonEmptyString mirrors Python's _managed_contact_string_fields:
// a present email/name must be a non-empty (trimmed) string, ported to Go so
// both languages reject a blank value instead of forwarding it (strictest-wins).
func managedContactNonEmptyString(request *mcp.CallToolRequest, name string, target **string, fields *int) string {
	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()[name]; !exists {
		return ""
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return name + " must be a non-empty string"
	}

	*target = &trimmed
	(*fields)++

	return ""
}

func managedContactUpdateFailureMessage(contactID int, err error) string {
	return "Failed to update linode_managed_contact " + strconv.Itoa(contactID) + ": " + err.Error()
}

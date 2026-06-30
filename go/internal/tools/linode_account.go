package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
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
	managedIDParam                     = "credential_" + "id"
	managedUpdateIDParam               = managedIDParam
	errManagedIDPositive               = managedIDParam + " must be a positive integer"
	maxManagedIDFromJSON               = 9007199254740991
	accountAvailabilityPageSizeMin     = 25
	accountAvailabilityPageSizeMax     = 500
	betasPageSizeMin                   = 25
	betasPageSizeMax                   = 500
	accountBetasPageSizeMin            = 25
	accountBetasPageSizeMax            = 500
	accountOAuthClientsPageSizeMin     = 25
	accountOAuthClientsPageSizeMax     = 500
	profileAppIDParam                  = "app_id"
	profileDeviceIDParam               = "device_id"
	profilePhoneISOCodeParam           = "iso_code"
	profilePhoneNumberParam            = "phone_number"
	profilePhoneOTPCodeParam           = "otp_code"
	profileTFACodeParam                = "tfa_code"
	profileTokenIDParam                = "token_id"
	profileAppIDMaxFromJSON            = 9007199254740991
	profileDeviceIDMaxFromJSON         = 9007199254740991
	errProfileAppIDPositive            = "app_id must be a positive integer"
	errProfileDeviceIDPositive         = "device_id must be a positive integer"
	errProfileTokenIDPositive          = "token_id must be a positive integer"
	longviewClientsPageSizeMin         = 25
	longviewClientsPageSizeMax         = 500
	longviewSubscriptionsPageSizeMin   = 25
	longviewSubscriptionsPageSizeMax   = 500
	monitorAlertChannelsPageSizeMin    = 25
	monitorAlertChannelsPageSizeMax    = 500
	longviewClientIDParam              = "client_id"
	longviewClientsPath                = "/longview/clients"
	longviewPlanPath                   = "/longview/plan"
	accountSettingsPath                = "/account/settings"
	accountUsersPath                   = "/account/users"
	supportTicketsPath                 = "/support/tickets"
	accountOAuthClientsPath            = "/account/oauth-clients"
	accountPaymentsPath                = "/account/payments"
	accountPaymentMethodsPath          = "/account/payment-methods"
	accountEntityTransfersPath         = "/account/entity-transfers"
	accountServiceTransfersPath        = "/account/service-transfers"
	accountServiceTransfersPageSizeMin = 25
	accountServiceTransfersPageSizeMax = 500
	accountAgreementsPath              = "/account/agreements"
	accountBetasPath                   = "/account/betas"
	accountCancelPath                  = "/account/cancel"
	accountPromoCodesPath              = "/account/promo-codes"
	profilePhoneNumberPath             = "/profile/phone-number"
	profilePhoneNumberVerifyPath       = profilePhoneNumberPath + "/verify"
	profileTFADisablePath              = "/profile/tfa-disable"
	profileTFAEnableConfirmPath        = "/profile/tfa-enable-confirm"
	accountEventsPath                  = "/account/events"
	accountChildAccountsPath           = "/account/child-accounts"
	longviewSubscriptionIDParam        = "subscription_id"
	maxLongviewClientIDFromJSON        = 9007199254740991
	errLongviewClientIDPositive        = "client_id must be a positive integer"
	errLongviewClientLabelRequired     = "label is required"
	errLongviewClientLabelPattern      = "label must be 3-32 characters and contain only letters, digits, hyphen, or underscore"
	accountPaymentMethodsPageSizeMin   = 25
	accountPaymentMethodsPageSizeMax   = 500
	accountMaintenancePageSizeMin      = 25
	accountMaintenancePageSizeMax      = 500
	accountNotificationsPageSizeMin    = 25
	accountNotificationsPageSizeMax    = 500
	managedCredentialsPageSizeMin      = 25
	managedCredentialsPageSizeMax      = 500
	managedCredentialCreateLabelParam  = "label"
	managedCredentialCreatePassParam   = "password"
	managedCredentialCreateUserParam   = "username"
	errManagedCredentialPasswordReq    = "password is required"
	accountEventsPageSizeMin           = 25
	accountEventsPageSizeMax           = 500
	taggedObjectsPageSizeMin           = 25
	taggedObjectsPageSizeMax           = 500
	tagLabelParam                      = "tag_label"
	errTagLabelRequired                = "tag_label is required"
	errTagLabelPathParam               = "tag_label must not contain '?', '#', or '..'"
	accountUsersPageSizeMin            = 25
	accountUsersPageSizeMax            = 500
	accountUserUsernameParam           = "username"
	errAccountUserUsernamePathParam    = "username must not contain '/', '?', '#', or '..'"
	errAccountUserUpdateSSHKeys        = "ssh_keys must be an array of non-empty strings"
	errAccountUserGrantsUpdateEmpty    = "at least one grant section is required"
	errAccountUserGrantsGlobalObject   = "global must be an object matching the grants schema"
	errAccountUserGrantsArray          = "grant sections must be arrays of grant objects"
	accountUserGrantsGlobalParam       = "global"
	accountUserGrantsLinodeParam       = "linode"
	accountUserGrantsDomainParam       = "domain"
	accountUserGrantsNodeBalancerParam = "nodebalancer"
	accountUserGrantsImageParam        = "image"
	accountUserGrantsLongviewParam     = "longview"
	accountUserGrantsStackScriptParam  = "stackscript"
	accountUserGrantsVolumeParam       = "volume"
	accountUserGrantsDatabaseParam     = "database"
	accountUserGrantsFirewallParam     = "firewall"
	accountUserGrantsVPCParam          = "vpc"
	accountUserGrantsLKEClusterParam   = "lkecluster"
	accountUserEmailParam              = "email"
	supportTicketSummaryParam          = "summary"
	supportTicketDescriptionParam      = "description"
	supportTicketAttachmentFileParam   = "file"
	supportTicketTicketIDParam         = "ticket_id"
	supportTicketBucketParam           = "bucket"
	supportTicketDatabaseIDParam       = "database_id"
	supportTicketDomainIDParam         = "domain_id"
	supportTicketFirewallIDParam       = "firewall_id"
	supportTicketLinodeIDParam         = "linode_id"
	supportTicketLKEClusterIDParam     = "lkecluster_id"
	supportTicketLongviewClientIDParam = "longviewclient_id"
	supportTicketManagedIssueParam     = "managed_issue"
	supportTicketNodeBalancerIDParam   = "nodebalancer_id"
	supportTicketRegionParam           = "region"
	supportTicketSeverityParam         = "severity"
	supportTicketVLANParam             = "vlan"
	supportTicketVolumeIDParam         = "volume_id"
	supportTicketVPCIDParam            = "vpc_id"
	managedContactNameParam            = "name"
	managedContactEmailParam           = "email"
	managedContactGroupParam           = "group"
	managedContactIDParam              = "id"
	managedContactUpdatedParam         = "updated"
	managedContactPhoneParam           = "phone"
	managedContactPhonePrimaryKey      = "primary"
	managedContactPhoneSecondaryKey    = "secondary"
	errManagedContactFieldRequired     = "at least one managed contact field is required"
	errManagedContactReadOnlyField     = "id and updated are read-only and cannot be set when creating a managed contact"
	accountLoginsPageSizeMin           = 25
	accountLoginsPageSizeMax           = 500
	maxAccountLoginIDFromJSON          = 9007199254740991
	maxAccountPaymentIDFromJSON        = 9007199254740991
	maxSupportTicketResourceIDFromJSON = 9007199254740991
	accountInvoicesPageSizeMin         = 25
	accountInvoicesPageSizeMax         = 500
	accountPaymentsPageSizeMin         = 25
	accountPaymentsPageSizeMax         = 500
	accountInvoiceItemsPageSizeMin     = 25
	accountInvoiceItemsPageSizeMax     = 500
	errAccountInvoiceIDPositive        = "invoice_id must be a positive integer"
	errAccountPaymentIDPositive        = "payment_id must be a positive integer"
	errAccountLoginIDPositive          = "login_id must be a positive integer"
	errLabelRequired                   = "label is required"
	errRegionRequired                  = "region is required"
	errRedirectURIRequired             = "redirect_uri is required"
	errPaymentMethodDataRequired       = "data is required"
	errPaymentMethodTypeRequired       = "type is required"
	oauthClientThumbnailPNGParam       = "thumbnail_png_base64"
	errThumbnailPNGRequired            = "thumbnail_png_base64 is required"
	accountChildAccountsPageSizeMin    = 25
	accountChildAccountsPageSizeMax    = 500
	accountEventIDParam                = "event_id"
)

// NewLinodeAccountTool creates a tool for retrieving Linode account information.
func NewLinodeAccountTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleProtoGetTool(
		cfg, "linode_account_get",
		"Retrieves the authenticated user's Linode account information including billing details and capabilities",
		"linode.mcp.v1.AccountGetInput",
		func(ctx context.Context, client *linode.Client) (proto.Message, error) {
			return client.GetAccountProto(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountTransferTool creates a tool for retrieving account network transfer usage.
func NewLinodeAccountTransferTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleProtoGetTool(
		cfg, "linode_account_transfer_get",
		"Retrieves the authenticated account's network transfer usage and quota by region.",
		"linode.mcp.v1.AccountTransferGetInput",
		func(ctx context.Context, client *linode.Client) (proto.Message, error) {
			return client.GetAccountTransferProto(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountSettingsTool creates a tool for retrieving account-wide settings.
func NewLinodeAccountSettingsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleProtoGetTool(
		cfg, "linode_account_settings_get",
		"Retrieves account-wide settings such as backups, network helper, Longview, object storage, interfaces, and maintenance policy",
		"linode.mcp.v1.AccountSettingsGetInput",
		func(ctx context.Context, client *linode.Client) (proto.Message, error) {
			return client.GetAccountSettingsProto(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountSettingsUpdateTool creates a tool for updating account-wide settings.
func NewLinodeAccountSettingsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_settings_update",
		"Updates account-wide settings such as backups, network helper, Longview, object storage, interfaces, and maintenance policy.",
		[]mcp.ToolOption{
			mcp.WithBoolean("backups_enabled", mcp.Description("Whether backups are enabled by default for new Linodes (optional).")),
			mcp.WithString("interfaces_for_new_linodes", mcp.Description("Default interface generation mode for new Linodes (optional).")),
			mcp.WithString("longview_subscription", mcp.Description("Longview subscription tier, or an empty string to disable it (optional).")),
			mcp.WithString("maintenance_policy", mcp.Description("Default maintenance policy for the account (optional).")),
			mcp.WithBoolean("managed", mcp.Description("Whether managed services are enabled for the account (optional).")),
			mcp.WithBoolean("network_helper", mcp.Description("Whether Network Helper is enabled by default (optional).")),
			mcp.WithString("object_storage", mcp.Description("Object Storage subscription status or tier (optional).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm account settings update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountSettingsUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountSettingsManagedEnableTool creates a tool for enabling Linode Managed.
func NewLinodeAccountSettingsManagedEnableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_settings_managed_enable",
		"Enables Linode Managed for the account. Pass dry_run=true to preview without enabling.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm enabling Linode Managed. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountSettingsManagedEnableRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountAgreementsTool creates a tool for listing account agreement acknowledgment status.
func NewLinodeAccountAgreementsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleProtoGetTool(
		cfg, "linode_account_agreement_list",
		"Lists account agreements and whether each has been acknowledged",
		"linode.mcp.v1.AccountAgreementsListInput",
		func(ctx context.Context, client *linode.Client) (proto.Message, error) {
			return client.GetAccountAgreementsProto(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedCredentialsTool creates a tool for listing managed credentials.
func NewLinodeManagedCredentialsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_managed_credential_list",
		"Lists stored managed credentials for the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ManagedCredential, error) {
			return client.ListManagedCredentialsProto(ctx, page, pageSize)
		},
		managedCredentialsPaginationFromTool,
		nil,
		managedCredentialListResponse,
	)

	return tool, profiles.CapRead, handler
}

func managedCredentialListResponse(items []*linodev1.ManagedCredential, count int32, filter *string) *linodev1.ManagedCredentialListResponse {
	return &linodev1.ManagedCredentialListResponse{Count: count, Filter: filter, ManagedCredentials: items}
}

// NewLinodeManagedCredentialUpdateTool creates a tool for updating one managed credential.
func NewLinodeManagedCredentialUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_credential_update",
		"Updates one stored managed credential by ID. Pass dry_run=true to preview without modifying.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedUpdateIDParam, mcp.Required(), mcp.Description("The numeric Managed credential ID to update.")),
			mcp.WithString("label", mcp.Required(), mcp.Description("Updated credential label.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm updating the Managed credential. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeManagedCredentialUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedCredentialUsernamePasswordUpdateTool creates a tool for updating a Managed credential's username and password.
func NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_credential_username_password_update",
		"Updates a stored Managed credential's username and password by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedIDParam, mcp.Required(), mcp.Description("Managed credential ID to update.")),
			mcp.WithString(managedCredentialCreatePassParam, mcp.Required(),
				mcp.Description("Updated password to store for the Managed credential.")),
			mcp.WithString(managedCredentialCreateUserParam,
				mcp.Description("Updated username to store for the Managed credential.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm updating a stored Managed credential's username and password. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeManagedCredentialUsernamePasswordUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedSSHKeyTool creates a tool for retrieving the account Managed SSH public key.
func NewLinodeManagedSSHKeyTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_sshkey_get",
		"Retrieves the Managed SSH public key assigned to the authenticated account.",
		nil,
		handleLinodeManagedSSHKeyRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeManagedCredentialCreateTool creates a tool for creating a Managed credential.
func NewLinodeManagedCredentialCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_credential_create",
		"Creates a stored Managed credential for the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithString(managedCredentialCreateLabelParam, mcp.Required(),
				mcp.Description("Label for the Managed credential.")),
			mcp.WithString(managedCredentialCreatePassParam, mcp.Required(),
				mcp.Description("Password to store for the Managed credential.")),
			mcp.WithString(managedCredentialCreateUserParam,
				mcp.Description("Username to store for the Managed credential.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm creating a stored Managed credential. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeManagedCredentialCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedCredentialGetTool creates a tool for retrieving one managed credential.
func NewLinodeManagedCredentialGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_managed_credential_get",
		"Gets one stored managed credential by ID. This account-level managed credential metadata requires admin capability. Pass dry_run=true to preview the request without retrieving it.",
		toolschemas.Schema("linode.mcp.v1.ManagedCredentialGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeManagedCredentialGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

// NewLinodeManagedCredentialRevokeTool creates a tool for revoking one managed credential.
func NewLinodeManagedCredentialRevokeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_credential_revoke",
		"Revokes one stored managed credential by ID. This credential-affecting action requires admin capability and confirm=true. Pass dry_run=true to preview without revoking.",
		[]mcp.ToolOption{
			mcp.WithNumber(managedIDParam, mcp.Required(), mcp.Description("Managed credential ID to revoke.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm revoking the stored Managed credential. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeManagedCredentialRevokeRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountMaintenanceTool creates a tool for listing account maintenance records.
func NewLinodeAccountMaintenanceTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_maintenance_list",
		"Lists maintenance records visible to the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountMaintenance, error) {
			return client.ListAccountMaintenanceProto(ctx, page, pageSize)
		},
		accountMaintenancePaginationFromTool,
		nil,
		accountMaintenanceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountMaintenanceListResponse(items []*linodev1.AccountMaintenance, count int32, filter *string) *linodev1.AccountMaintenanceListResponse {
	return &linodev1.AccountMaintenanceListResponse{Count: count, Filter: filter, AccountMaintenances: items}
}

// NewLinodeMaintenancePoliciesTool creates a tool for listing available Linode maintenance policies.
func NewLinodeMaintenancePoliciesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_maintenance_policy_list",
		"Lists available Linode maintenance policies.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.MaintenancePolicy, error) {
			return client.ListMaintenancePoliciesProto(ctx, page, pageSize)
		},
		accountMaintenancePaginationFromTool,
		nil,
		maintenancePolicyListResponse,
	)

	return tool, profiles.CapRead, handler
}

func maintenancePolicyListResponse(items []*linodev1.MaintenancePolicy, count int32, filter *string) *linodev1.MaintenancePolicyListResponse {
	return &linodev1.MaintenancePolicyListResponse{Count: count, Filter: filter, MaintenancePolicies: items}
}

// NewLinodeAccountNotificationsTool creates a tool for listing account notifications.
func NewLinodeAccountNotificationsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_notification_list",
		"Lists active notifications for the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountNotification, error) {
			return client.ListAccountNotificationsProto(ctx, page, pageSize)
		},
		accountNotificationsPaginationFromTool,
		nil,
		accountNotificationListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountNotificationListResponse(items []*linodev1.AccountNotification, count int32, filter *string) *linodev1.AccountNotificationListResponse {
	return &linodev1.AccountNotificationListResponse{Count: count, Filter: filter, AccountNotifications: items}
}

// NewLinodeAccountBetasTool creates a tool for listing enrolled account beta programs.
func NewLinodeAccountBetasTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_beta_list",
		"Lists beta programs that the account is enrolled in.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountBetaProgram, error) {
			return client.ListAccountBetasProto(ctx, page, pageSize)
		},
		accountBetasPaginationFromTool,
		nil,
		accountBetaListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountBetaListResponse(items []*linodev1.AccountBetaProgram, count int32, filter *string) *linodev1.AccountBetaProgramListResponse {
	return &linodev1.AccountBetaProgramListResponse{Count: count, Filter: filter, AccountBetas: items}
}

// NewLinodeAccountEventsTool creates a tool for listing account events.
func NewLinodeAccountEventsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_event_list",
		"Lists events that represent actions taken on the account over the last 90 days.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountEvent, error) {
			return client.ListAccountEventsProto(ctx, page, pageSize)
		},
		accountEventsPaginationFromTool,
		nil,
		accountEventListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountEventListResponse(items []*linodev1.AccountEvent, count int32, filter *string) *linodev1.AccountEventListResponse {
	return &linodev1.AccountEventListResponse{Count: count, Filter: filter, AccountEvents: items}
}

// NewLinodeTaggedObjectsTool creates a tool for listing objects with a tag label.
func NewLinodeTaggedObjectsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_tag_object_list",
		"Lists Linode objects that have the supplied tag label.",
		[]mcp.ToolOption{
			mcp.WithString(tagLabelParam, mcp.Required(), mcp.Description("Tag label to list objects for.")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeTaggedObjectsRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountUsersTool creates a tool for listing account users.
func NewLinodeAccountUsersTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_user_list",
		"Lists users on the account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountUser, error) {
			return client.ListAccountUsersProto(ctx, page, pageSize)
		},
		accountUsersPaginationFromTool,
		nil,
		accountUserListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountUserListResponse(items []*linodev1.AccountUser, count int32, filter *string) *linodev1.AccountUserListResponse {
	return &linodev1.AccountUserListResponse{Count: count, Filter: filter, AccountUsers: items}
}

// NewLinodeAccountUserGetTool creates a tool for retrieving one account user.
func NewLinodeAccountUserGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_user_get",
		"Gets one account user by username.",
		toolschemas.Schema("linode.mcp.v1.AccountUserGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountUserGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeProfileTokenGetTool creates a tool for retrieving one personal access token.
func NewLinodeProfileTokenGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_token_get",
		"Gets one personal access token by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(profileTokenIDParam, mcp.Required(), mcp.Description("Personal access token ID to retrieve.")),
		},
		handleLinodeProfileTokenGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountUserGrantsTool creates a tool for retrieving one account user's grants.
func NewLinodeAccountUserGrantsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_user_grants_get",
		"Gets grants for one account user by username.",
		[]mcp.ToolOption{
			mcp.WithString(accountUserUsernameParam, mcp.Required(), mcp.Description("Account username whose grants should be retrieved.")),
		},
		handleLinodeAccountUserGrantsRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountUserGrantsUpdateTool creates a tool for updating one account user's grants.
func NewLinodeAccountUserGrantsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_user_grants_update",
		"Updates grants for one account user by username.",
		[]mcp.ToolOption{
			mcp.WithString(accountUserUsernameParam, mcp.Required(), mcp.Description("Account username whose grants should be updated.")),
			mcp.WithObject(accountUserGrantsGlobalParam, mcp.Description("Optional global grants object.")),
			mcp.WithArray(accountUserGrantsLinodeParam, mcp.Description("Optional Linode resource grants.")),
			mcp.WithArray(accountUserGrantsDomainParam, mcp.Description("Optional domain resource grants.")),
			mcp.WithArray(accountUserGrantsNodeBalancerParam, mcp.Description("Optional NodeBalancer resource grants.")),
			mcp.WithArray(accountUserGrantsImageParam, mcp.Description("Optional image resource grants.")),
			mcp.WithArray(accountUserGrantsLongviewParam, mcp.Description("Optional Longview resource grants.")),
			mcp.WithArray(accountUserGrantsStackScriptParam, mcp.Description("Optional StackScript resource grants.")),
			mcp.WithArray(accountUserGrantsVolumeParam, mcp.Description("Optional volume resource grants.")),
			mcp.WithArray(accountUserGrantsDatabaseParam, mcp.Description("Optional database resource grants.")),
			mcp.WithArray(accountUserGrantsFirewallParam, mcp.Description("Optional firewall resource grants.")),
			mcp.WithArray(accountUserGrantsVPCParam, mcp.Description("Optional VPC resource grants.")),
			mcp.WithArray(accountUserGrantsLKEClusterParam, mcp.Description("Optional LKE cluster resource grants.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm account user grants update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountUserGrantsUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountUserUpdateTool creates a tool for updating one account user.
func NewLinodeAccountUserUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_user_update",
		"Updates one account user by username.",
		[]mcp.ToolOption{
			mcp.WithString(accountUserUsernameParam, mcp.Required(), mcp.Description("Account username to update.")),
			mcp.WithString(accountUserEmailParam, mcp.Description("New email address for the account user.")),
			mcp.WithBoolean("restricted", mcp.Description("Whether the account user is restricted.")),
			mcp.WithArray("ssh_keys", mcp.Description("SSH public keys for the account user.")),
			mcp.WithString("new_username", mcp.Description("New username for the account user.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm account user update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountUserUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountUserDeleteTool creates a tool for deleting one account user.
func NewLinodeAccountUserDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_user_delete",
		"Deletes one account user by username.",
		[]mcp.ToolOption{
			mcp.WithString(accountUserUsernameParam, mcp.Required(), mcp.Description("Account username to delete.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm account user deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountUserDeleteRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountUserCreateTool creates a tool for creating account users.
func NewLinodeAccountUserCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_user_create",
		"Creates a user on the account.",
		[]mcp.ToolOption{
			mcp.WithString(accountUserUsernameParam, mcp.Required(), mcp.Description("Username for the new account user.")),
			mcp.WithString(accountUserEmailParam, mcp.Required(), mcp.Description("Email address for the new account user.")),
			mcp.WithBoolean("restricted", mcp.Description("Whether the new account user is restricted (optional, defaults to false).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm account user creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountUserCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountSupportTicketCreateTool creates a tool for opening support tickets.
func NewLinodeAccountSupportTicketCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_support_ticket_create",
		"Opens a support ticket for the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithString(supportTicketSummaryParam, mcp.Required(), mcp.Description("Short summary for the support ticket.")),
			mcp.WithString(supportTicketDescriptionParam, mcp.Required(), mcp.Description("Detailed support ticket description.")),
			mcp.WithString(supportTicketBucketParam, mcp.Description("Object Storage bucket related to the ticket (optional).")),
			mcp.WithNumber(supportTicketDatabaseIDParam, mcp.Description("Database ID related to the ticket (optional).")),
			mcp.WithNumber(supportTicketDomainIDParam, mcp.Description("Domain ID related to the ticket (optional).")),
			mcp.WithNumber(supportTicketFirewallIDParam, mcp.Description("Firewall ID related to the ticket (optional).")),
			mcp.WithNumber(supportTicketLinodeIDParam, mcp.Description("Linode ID related to the ticket (optional).")),
			mcp.WithNumber(supportTicketLKEClusterIDParam, mcp.Description("LKE cluster ID related to the ticket (optional).")),
			mcp.WithNumber(supportTicketLongviewClientIDParam, mcp.Description("Longview client ID related to the ticket (optional).")),
			mcp.WithBoolean(supportTicketManagedIssueParam, mcp.Description("Whether the ticket concerns a Managed service issue (optional).")),
			mcp.WithNumber(supportTicketNodeBalancerIDParam, mcp.Description("NodeBalancer ID related to the ticket (optional).")),
			mcp.WithString(supportTicketRegionParam, mcp.Description("Region related to the ticket (optional).")),
			mcp.WithNumber(supportTicketSeverityParam, mcp.Description("Support ticket severity level 1, 2, or 3 (optional).")),
			mcp.WithString(supportTicketVLANParam, mcp.Description("VLAN related to the ticket (optional).")),
			mcp.WithNumber(supportTicketVolumeIDParam, mcp.Description("Volume ID related to the ticket (optional).")),
			mcp.WithNumber(supportTicketVPCIDParam, mcp.Description("VPC ID related to the ticket (optional).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm support ticket creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountSupportTicketCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeAccountSupportTicketAttachmentCreateTool creates a tool for adding support ticket attachments.
func NewLinodeAccountSupportTicketAttachmentCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_support_ticket_attachment_create",
		"Creates an attachment on an existing support ticket.",
		[]mcp.ToolOption{
			mcp.WithNumber(supportTicketTicketIDParam, mcp.Required(), mcp.Description("Support ticket ID to attach the file to.")),
			mcp.WithString(supportTicketAttachmentFileParam, mcp.Required(), mcp.Description("Attachment file content or reference accepted by the Linode API.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm support ticket attachment creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountSupportTicketAttachmentCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeAccountSupportTicketReplyCreateTool creates a tool for adding support ticket replies.
func NewLinodeAccountSupportTicketReplyCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_support_ticket_reply_create",
		"Creates a reply on an existing support ticket.",
		[]mcp.ToolOption{
			mcp.WithNumber(supportTicketTicketIDParam, mcp.Required(), mcp.Description("Support ticket ID to reply to.")),
			mcp.WithString(supportTicketDescriptionParam, mcp.Required(), mcp.Description("Reply description to add to the support ticket.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm support ticket reply creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountSupportTicketReplyCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeManagedContactCreateTool creates a tool for creating managed contacts.
func NewLinodeManagedContactCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_managed_contact_create",
		"Creates a managed contact for service monitor issue handling. Pass dry_run=true to preview without creating.",
		[]mcp.ToolOption{
			mcp.WithString(managedContactNameParam, mcp.Description("Name for the managed contact.")),
			mcp.WithString(managedContactEmailParam, mcp.Description("Email address for the managed contact.")),
			mcp.WithString(managedContactGroupParam, mcp.Description("Display grouping for the managed contact.")),
			mcp.WithObject(managedContactPhoneParam, mcp.Description("Phone numbers object: { primary: string, secondary: string }.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm managed contact creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeManagedContactCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountLoginsTool creates a tool for listing account user logins.
func NewLinodeAccountLoginsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_login_list",
		"Lists user logins for the account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountLogin, error) {
			return client.ListAccountLoginsProto(ctx, page, pageSize)
		},
		accountLoginsPaginationFromTool,
		nil,
		accountLoginListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountLoginListResponse(items []*linodev1.AccountLogin, count int32, filter *string) *linodev1.AccountLoginListResponse {
	return &linodev1.AccountLoginListResponse{Count: count, Filter: filter, AccountLogins: items}
}

// NewLinodeAccountLoginGetTool creates a tool for retrieving one account login.
func NewLinodeAccountLoginGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_login_get",
		"Gets one account login by ID.",
		toolschemas.Schema("linode.mcp.v1.AccountLoginGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountLoginGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountInvoicesTool creates a tool for listing account invoices.
func NewLinodeAccountInvoicesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_invoice_list",
		"Lists invoices for the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountInvoice, error) {
			return client.ListAccountInvoicesProto(ctx, page, pageSize)
		},
		accountInvoicesPaginationFromTool,
		nil,
		accountInvoiceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountInvoiceListResponse(items []*linodev1.AccountInvoice, count int32, filter *string) *linodev1.AccountInvoiceListResponse {
	return &linodev1.AccountInvoiceListResponse{Count: count, Filter: filter, AccountInvoices: items}
}

// NewLinodeAccountPaymentsTool creates a tool for listing account payments.
func NewLinodeAccountPaymentsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_payment_list",
		"Lists payments made on the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountPayment, error) {
			return client.ListAccountPaymentsProto(ctx, page, pageSize)
		},
		accountPaymentsPaginationFromTool,
		nil,
		accountPaymentListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountPaymentListResponse(items []*linodev1.AccountPayment, count int32, filter *string) *linodev1.AccountPaymentListResponse {
	return &linodev1.AccountPaymentListResponse{Count: count, Filter: filter, AccountPayments: items}
}

// NewLinodeAccountPaymentGetTool creates a tool for retrieving one account payment.
func NewLinodeAccountPaymentGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_payment_get",
		"Gets one payment made on the authenticated account by ID.",
		toolschemas.Schema("linode.mcp.v1.AccountPaymentGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountPaymentGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountPaymentCreateTool creates a tool for making an account payment.
func NewLinodeAccountPaymentCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_payment_create",
		"Makes a payment on the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithNumber("payment_method_id", mcp.Description("Payment method ID to charge (optional).")),
			mcp.WithString("usd", mcp.Required(), mcp.Description("Payment amount in USD, as a decimal string (e.g. \"25.50\").")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm making an account payment. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountPaymentCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountPromoCreditTool creates a tool for applying a promo credit to the account.
func NewLinodeAccountPromoCreditTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_promo_credit_add",
		"Applies a promo credit to the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithString("promo_code", mcp.Required(), mcp.Description("Promo code to apply to the account.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm applying a promo credit. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountPromoCreditRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountInvoiceGetTool creates a tool for retrieving one account invoice.
func NewLinodeAccountInvoiceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_invoice_get",
		"Gets one account invoice by ID.",
		toolschemas.Schema("linode.mcp.v1.AccountInvoiceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountInvoiceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountInvoiceItemsTool creates a tool for listing items on one account invoice.
func NewLinodeAccountInvoiceItemsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_account_invoice_item_list",
		"Lists line items for one account invoice by ID.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber("invoice_id", mcp.Required(),
				mcp.Description("Invoice ID whose items should be listed.")),
			parse: accountInvoiceIDFromTool,
		},
		accountInvoiceItemsPaginationFromTool,
		func(ctx context.Context, client *linode.Client, invoiceID, page, pageSize int) ([]*linodev1.AccountInvoiceItem, error) {
			return client.ListAccountInvoiceItemsProto(ctx, invoiceID, page, pageSize)
		},
		nil,
		accountInvoiceItemListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountInvoiceItemListResponse(items []*linodev1.AccountInvoiceItem, count int32, filter *string) *linodev1.AccountInvoiceItemListResponse {
	return &linodev1.AccountInvoiceItemListResponse{Count: count, Filter: filter, AccountInvoiceItems: items}
}

// NewLinodeProfileTFAEnableTool creates a tool for generating a two-factor authentication secret.
func NewLinodeProfileTFAEnableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_tfa_enable",
		"Generates a two-factor authentication secret for the authenticated profile. The secret must be confirmed with the API before two-factor authentication is enabled.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm generating a two-factor authentication secret. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileTFAEnableRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfilePhoneNumberSendTool creates a tool for sending a profile phone verification code.
func NewLinodeProfilePhoneNumberSendTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_phone_number_send",
		"Sends a verification code to a profile phone number.",
		[]mcp.ToolOption{
			mcp.WithString(profilePhoneISOCodeParam, mcp.Required(),
				mcp.Description("ISO 3166-1 alpha-2 country code for the phone number.")),
			mcp.WithString(profilePhoneNumberParam, mcp.Required(),
				mcp.Description("Phone number that should receive the verification code.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm sending the verification code. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfilePhoneNumberSendRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfilePhoneNumberDeleteTool creates a tool for deleting a profile phone number.
func NewLinodeProfilePhoneNumberDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_phone_number_delete",
		"Deletes the authenticated profile phone number.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deleting the profile phone number. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfilePhoneNumberDeleteRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfilePhoneNumberVerifyTool creates a tool for verifying a profile phone number.
func NewLinodeProfilePhoneNumberVerifyTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_phone_number_verify",
		"Verifies a profile phone number with a one-time SMS code.",
		[]mcp.ToolOption{
			mcp.WithString(profilePhoneOTPCodeParam, mcp.Required(),
				mcp.Description("One-time SMS code sent to the profile phone number.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm verifying the phone number. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfilePhoneNumberVerifyRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfileTFADisableTool creates a tool for disabling profile two-factor authentication.
func NewLinodeProfileTFADisableTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_tfa_disable",
		"Disables two-factor authentication for the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm disabling profile two-factor authentication. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileTFADisableRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfileTFAEnableConfirmTool creates a tool for confirming profile two-factor authentication enablement.
func NewLinodeProfileTFAEnableConfirmTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_tfa_enable_confirm",
		"Confirms two-factor authentication enablement for the authenticated profile with a TFA code.",
		[]mcp.ToolOption{
			mcp.WithString(profileTFACodeParam, mcp.Required(),
				mcp.Description("Two-factor authentication code to confirm enablement.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm enabling profile two-factor authentication. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileTFAEnableConfirmRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfileDevicesTool creates a tool for listing trusted devices for the profile.
func NewLinodeProfileDevicesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_profile_device_list",
		"Lists trusted devices for the authenticated profile.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.TrustedDevice, error) {
			return client.ListProfileDevicesProto(ctx, page, pageSize)
		},
		profileDevicesPaginationFromTool,
		nil,
		profileDeviceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func profileDeviceListResponse(items []*linodev1.TrustedDevice, count int32, filter *string) *linodev1.TrustedDeviceListResponse {
	return &linodev1.TrustedDeviceListResponse{Count: count, Filter: filter, ProfileDevices: items}
}

// NewLinodeProfileLoginGetTool creates a tool for retrieving one profile login.
func NewLinodeProfileLoginGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_login_get",
		"Gets one login for the authenticated profile by ID.",
		toolschemas.Schema("linode.mcp.v1.ProfileLoginGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeProfileLoginGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeProfileAppGetTool creates a tool for retrieving one profile authorized OAuth app.
func NewLinodeProfileAppGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_app_get",
		"Gets one OAuth app authorization from the profile.",
		toolschemas.Schema("linode.mcp.v1.ProfileAppGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeProfileAppGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeProfileAppDeleteTool creates a tool for revoking one profile authorized OAuth app.
func NewLinodeProfileAppDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_app_delete",
		"Revokes OAuth app access from the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithNumber(profileAppIDParam, mcp.Required(),
				mcp.Description("Profile authorized app ID to revoke.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm revoking OAuth app access. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileAppDeleteRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeProfileDeviceGetTool creates a tool for retrieving one profile trusted device.
func NewLinodeProfileDeviceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_device_get",
		"Gets one trusted device from the profile.",
		[]mcp.ToolOption{
			mcp.WithNumber(profileDeviceIDParam, mcp.Required(),
				mcp.Description("Profile trusted device ID.")),
		},
		handleLinodeProfileDeviceGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeProfileDeviceRevokeTool creates a tool for revoking one profile trusted device.
func NewLinodeProfileDeviceRevokeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_device_revoke",
		"Revokes a trusted device from the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithNumber(profileDeviceIDParam, mcp.Required(),
				mcp.Description("Profile trusted device ID to revoke.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm revoking a trusted device. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileDeviceRevokeRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountOAuthClientsTool creates a tool for listing OAuth clients registered on the account.
func NewLinodeAccountOAuthClientsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_oauth_client_list",
		"Lists OAuth clients registered on the account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.OAuthClient, error) {
			return client.ListAccountOAuthClientsProto(ctx, page, pageSize)
		},
		accountOAuthClientsPaginationFromTool,
		nil,
		accountOAuthClientListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountOAuthClientListResponse(items []*linodev1.OAuthClient, count int32, filter *string) *linodev1.OAuthClientListResponse {
	return &linodev1.OAuthClientListResponse{Count: count, Filter: filter, AccountOauthClients: items}
}

// NewLinodeProfileAppsTool creates a tool for listing OAuth app authorizations for the profile.
func NewLinodeProfileAppsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_profile_app_list",
		"Lists OAuth app authorizations for the authenticated profile.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ProfileApp, error) {
			return client.ListProfileAppsProto(ctx, page, pageSize)
		},
		profileAppsPaginationFromTool,
		nil,
		profileAppListResponse,
	)

	return tool, profiles.CapRead, handler
}

func profileAppListResponse(items []*linodev1.ProfileApp, count int32, filter *string) *linodev1.ProfileAppListResponse {
	return &linodev1.ProfileAppListResponse{Count: count, Filter: filter, ProfileApps: items}
}

// NewLinodeLongviewClientsTool creates a tool for listing Longview clients.
func NewLinodeLongviewClientsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_longview_client_list",
		"Lists Longview clients configured for the account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.LongviewClient, error) {
			return client.ListLongviewClientsProto(ctx, page, pageSize)
		},
		longviewClientsPaginationFromTool,
		nil,
		longviewClientListResponse,
	)

	return tool, profiles.CapRead, handler
}

func longviewClientListResponse(items []*linodev1.LongviewClient, count int32, filter *string) *linodev1.LongviewClientListResponse {
	return &linodev1.LongviewClientListResponse{Count: count, Filter: filter, LongviewClients: items}
}

// NewLinodeLongviewClientUpdateTool creates a tool for updating one Longview client's label.
func NewLinodeLongviewClientUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_client_update",
		"Updates the label for one Longview client. Pass dry_run=true to preview without modifying.",
		[]mcp.ToolOption{
			mcp.WithNumber(longviewClientIDParam, mcp.Required(),
				mcp.Description("Longview client ID to update.")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("New Longview client label. Must be 3-32 letters, digits, hyphens, or underscores.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm Longview client update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeLongviewClientUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeLongviewClientDeleteTool creates a tool for deleting one Longview client.
func NewLinodeLongviewClientDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_client_delete",
		"Deletes one Longview client. Pass dry_run=true to preview without deleting.",
		[]mcp.ToolOption{
			mcp.WithNumber(longviewClientIDParam, mcp.Required(),
				mcp.Description("Longview client ID to delete.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm Longview client deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeLongviewClientDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

// NewLinodeLongviewClientGetTool creates a tool for retrieving one Longview client.
func NewLinodeLongviewClientGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_longview_client_get",
		"Gets one Longview client. Secret-bearing Longview install fields are not included in the tool response.",
		toolschemas.Schema("linode.mcp.v1.LongviewClientGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeLongviewClientGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeLongviewSubscriptionGetTool creates a tool for retrieving one Longview subscription.
func NewLinodeLongviewSubscriptionGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_longview_subscription_get",
		"Gets one Longview subscription by ID.",
		toolschemas.Schema("linode.mcp.v1.LongviewSubscriptionGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeLongviewSubscriptionGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountPaymentMethodsTool creates a tool for listing payment methods for the account.
func NewLinodeAccountPaymentMethodsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_payment_method_list",
		"Lists payment methods for the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountPaymentMethod, error) {
			return client.ListAccountPaymentMethodsProto(ctx, page, pageSize)
		},
		accountPaymentMethodsPaginationFromTool,
		nil,
		accountPaymentMethodListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountPaymentMethodListResponse(items []*linodev1.AccountPaymentMethod, count int32, filter *string) *linodev1.AccountPaymentMethodListResponse {
	return &linodev1.AccountPaymentMethodListResponse{Count: count, Filter: filter, AccountPaymentMethods: items}
}

// NewLinodeAccountPaymentMethodGetTool creates a tool for retrieving one payment method.
func NewLinodeAccountPaymentMethodGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_payment_method_get",
		"Gets one payment method for the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithNumber("payment_method_id", mcp.Required(),
				mcp.Description("Payment method ID.")),
		},
		handleLinodeAccountPaymentMethodGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountPaymentMethodCreateTool creates a tool for adding a payment method to the account.
func NewLinodeAccountPaymentMethodCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_payment_method_create",
		"Adds a payment method to the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithString("type", mcp.Required(), mcp.Description("Payment method type.")),
			mcp.WithObject("data", mcp.Required(), mcp.Description("Payment method provider data.")),
			mcp.WithBoolean("is_default", mcp.Required(), mcp.Description("Whether the payment method should become the account default.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm payment method creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountPaymentMethodCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountPaymentMethodDeleteTool creates a tool for deleting one payment method.
func NewLinodeAccountPaymentMethodDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_payment_method_delete",
		"Deletes a payment method from the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithNumber("payment_method_id", mcp.Required(),
				mcp.Description("Payment method ID.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm payment method deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountPaymentMethodDeleteRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountPaymentMethodMakeDefaultTool creates a tool for setting the account default payment method.
func NewLinodeAccountPaymentMethodMakeDefaultTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_payment_method_make_default",
		"Sets a payment method as the authenticated account default.",
		[]mcp.ToolOption{
			mcp.WithNumber("payment_method_id", mcp.Required(),
				mcp.Description("Payment method ID.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm changing the default payment method. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountPaymentMethodMakeDefaultRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountOAuthClientGetTool creates a tool for retrieving one OAuth client.
func NewLinodeAccountOAuthClientGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_oauth_client_get",
		"Gets one OAuth client registered on the account. OAuth client secrets are not returned by this tool.",
		toolschemas.Schema("linode.mcp.v1.AccountOAuthClientGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountOAuthClientGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountOAuthClientCreateTool creates a tool for creating an OAuth client.
func NewLinodeAccountOAuthClientCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_oauth_client_create",
		"Creates an account OAuth client. WARNING: The secret is only shown once in the response and cannot be retrieved later.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(), mcp.Description("Label for the OAuth client.")),
			mcp.WithString("redirect_uri", mcp.Required(), mcp.Description("Redirect URI for the OAuth client.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm OAuth client creation. The secret is only shown once. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountOAuthClientCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountOAuthClientUpdateTool creates a tool for updating one OAuth client.
func NewLinodeAccountOAuthClientUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_oauth_client_update",
		"Updates label, redirect URI, or public setting for one account OAuth client.",
		[]mcp.ToolOption{
			mcp.WithString("client_id", mcp.Required(), mcp.Description("OAuth client ID.")),
			mcp.WithString("label", mcp.Description("New label for the OAuth client.")),
			mcp.WithString("redirect_uri", mcp.Description("New redirect URI for the OAuth client.")),
			mcp.WithBoolean("public", mcp.Description("Whether this OAuth client is public.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm OAuth client update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountOAuthClientUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountOAuthClientThumbnailUpdateTool creates a tool for updating one OAuth client's thumbnail.
func NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_oauth_client_thumbnail_update",
		"Updates one account OAuth client's thumbnail by ID.",
		[]mcp.ToolOption{
			mcp.WithString("client_id", mcp.Required(), mcp.Description("OAuth client ID.")),
			mcp.WithString(oauthClientThumbnailPNGParam, mcp.Required(), mcp.Description("Base64-encoded PNG image for the OAuth client thumbnail.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm OAuth client thumbnail update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountOAuthClientThumbnailUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountOAuthClientThumbnailGetTool creates a tool for retrieving one OAuth client's thumbnail.
func NewLinodeAccountOAuthClientThumbnailGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_oauth_client_thumbnail_get",
		"Gets one account OAuth client's thumbnail by ID. Returns base64-encoded PNG image data.",
		[]mcp.ToolOption{
			mcp.WithString("client_id", mcp.Required(), mcp.Description("OAuth client ID.")),
		},
		handleLinodeAccountOAuthClientThumbnailGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountOAuthClientDeleteTool creates a tool for deleting one OAuth client.
func NewLinodeAccountOAuthClientDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_oauth_client_delete",
		"Deletes one account OAuth client by ID.",
		[]mcp.ToolOption{
			mcp.WithString("client_id", mcp.Required(), mcp.Description("OAuth client ID.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm OAuth client deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountOAuthClientDeleteRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountOAuthClientResetSecretTool creates a tool for resetting one OAuth client secret.
func NewLinodeAccountOAuthClientResetSecretTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_oauth_client_secret_reset",
		"Resets one account OAuth client secret by ID. WARNING: The new secret is only shown once in the response and cannot be retrieved later.",
		[]mcp.ToolOption{
			mcp.WithString("client_id", mcp.Required(), mcp.Description("OAuth client ID.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm OAuth client secret reset. The new secret is only shown once. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountOAuthClientResetSecretRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountChildAccountsTool creates a tool for listing child-level accounts.
func NewLinodeAccountChildAccountsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_child_account_list",
		"Lists child-level accounts the authenticated account can access.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.ChildAccount, error) {
			return client.ListAccountChildAccountsProto(ctx, page, pageSize)
		},
		accountChildAccountsPaginationFromTool,
		nil,
		accountChildAccountListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountChildAccountListResponse(items []*linodev1.ChildAccount, count int32, filter *string) *linodev1.ChildAccountListResponse {
	return &linodev1.ChildAccountListResponse{Count: count, Filter: filter, AccountChildAccounts: items}
}

// NewLinodeAccountServiceTransfersTool creates a tool for listing account service transfers.
func NewLinodeAccountServiceTransfersTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_service_transfer_list",
		"Lists account service transfer requests.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountEntityTransfer, error) {
			return client.ListAccountServiceTransfersProto(ctx, page, pageSize)
		},
		accountServiceTransfersPaginationFromTool,
		nil,
		accountServiceTransferListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountServiceTransferListResponse(items []*linodev1.AccountEntityTransfer, count int32, filter *string) *linodev1.AccountServiceTransferListResponse {
	return &linodev1.AccountServiceTransferListResponse{Count: count, Filter: filter, AccountServiceTransfers: items}
}

// NewLinodeAccountServiceTransferGetTool creates a tool for retrieving one account service transfer.
func NewLinodeAccountServiceTransferGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_service_transfer_get",
		"Gets one account service transfer request by token.",
		toolschemas.Schema("linode.mcp.v1.AccountServiceTransferGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountServiceTransferGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountServiceTransferCreateTool creates a tool for creating an account service transfer.
func NewLinodeAccountServiceTransferCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_service_transfer_create",
		"Creates an account service transfer for the provided Linode IDs.",
		[]mcp.ToolOption{
			mcp.WithArray("linode_ids", mcp.Required(),
				mcp.Description("Linode IDs to include in the service transfer.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm service transfer creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountServiceTransferCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountServiceTransferDeleteTool creates a tool for canceling one account service transfer.
func NewLinodeAccountServiceTransferDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_service_transfer_delete",
		"Cancels one account service transfer request by token.",
		[]mcp.ToolOption{
			mcp.WithString("token", mcp.Required(),
				mcp.Description("Service transfer token to cancel.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm service transfer cancellation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountServiceTransferDeleteRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountServiceTransferAcceptTool creates a tool for accepting one account service transfer.
func NewLinodeAccountServiceTransferAcceptTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_service_transfer_accept",
		"Accepts one account service transfer request by token.",
		[]mcp.ToolOption{
			mcp.WithString("token", mcp.Required(),
				mcp.Description("Service transfer token to accept.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm service transfer acceptance. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountServiceTransferAcceptRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountEventGetTool creates a tool for retrieving one account event.
func NewLinodeAccountEventGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_event_get",
		"Gets one account event by ID.",
		toolschemas.Schema("linode.mcp.v1.AccountEventGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountEventGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountEventSeenTool creates a tool for marking one account event as seen.
func NewLinodeAccountEventSeenTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_event_seen",
		"Marks one account event as seen by ID.",
		[]mcp.ToolOption{
			mcp.WithNumber(accountEventIDParam, mcp.Required(),
				mcp.Description("Numeric account event ID to mark as seen.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm marking the account event as seen. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountEventSeenRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeAccountChildAccountGetTool creates a tool for retrieving one child-level account.
func NewLinodeAccountChildAccountGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_child_account_get",
		"Gets one child-level account the authenticated account can access.",
		[]mcp.ToolOption{
			mcp.WithString("euuid", mcp.Required(),
				mcp.Description("External unique identifier for the child account.")),
		},
		handleLinodeAccountChildAccountGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountChildAccountTokenTool creates a tool for creating a child account proxy user token.
func NewLinodeAccountChildAccountTokenTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_child_account_token_create",
		"Creates a short-lived proxy user token for one child-level account.",
		[]mcp.ToolOption{
			mcp.WithString("euuid", mcp.Required(),
				mcp.Description("External unique identifier for the child account.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm proxy user token creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountChildAccountTokenRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeBetasTool creates a tool for listing available beta programs.
func NewLinodeBetasTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_beta_list",
		"Lists available beta programs.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.BetaProgram, error) {
			return client.ListBetasProto(ctx, page, pageSize)
		},
		betasPaginationFromTool,
		nil,
		betaListResponse,
	)

	return tool, profiles.CapRead, handler
}

func betaListResponse(items []*linodev1.BetaProgram, count int32, filter *string) *linodev1.BetaProgramListResponse {
	return &linodev1.BetaProgramListResponse{Count: count, Filter: filter, Betas: items}
}

// NewLinodeBetaGetTool creates a tool for retrieving one available beta program.
func NewLinodeBetaGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_beta_get",
		"Gets one available beta program.",
		toolschemas.Schema("linode.mcp.v1.BetaGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeBetaGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountBetaGetTool creates a tool for retrieving one enrolled account beta program.
func NewLinodeAccountBetaGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_beta_get",
		"Gets one beta program that the account is enrolled in.",
		toolschemas.Schema("linode.mcp.v1.AccountBetaGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountBetaGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountBetaEnrollTool creates a tool for enrolling in an account beta program.
func NewLinodeAccountBetaEnrollTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_beta_enroll",
		"Enrolls the account in a beta program.",
		[]mcp.ToolOption{
			mcp.WithString("id", mcp.Required(),
				mcp.Description("Unique identifier for the beta program.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm beta program enrollment. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountBetaEnrollRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountAvailabilityTool creates a tool for listing account service availability by region.
func NewLinodeAccountAvailabilityTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_account_availability_list",
		"Lists services available and unavailable to the account in each region.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.AccountAvailability, error) {
			return client.ListAccountAvailabilityProto(ctx, page, pageSize)
		},
		accountAvailabilityPaginationFromTool,
		nil,
		accountAvailabilityListResponse,
	)

	return tool, profiles.CapRead, handler
}

func accountAvailabilityListResponse(items []*linodev1.AccountAvailability, count int32, filter *string) *linodev1.AccountAvailabilityListResponse {
	return &linodev1.AccountAvailabilityListResponse{Count: count, Filter: filter, AccountAvailabilities: items}
}

// NewLinodeAccountAvailabilityGetTool creates a tool for retrieving account service availability for one region.
func NewLinodeAccountAvailabilityGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_account_availability_get",
		"Gets services available and unavailable to the account in one region.",
		toolschemas.Schema("linode.mcp.v1.AccountAvailabilityGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeAccountAvailabilityGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountAgreementsAcknowledgeTool creates a tool for acknowledging account agreements.
func NewLinodeAccountAgreementsAcknowledgeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_agreement_acknowledge",
		"Acknowledges one or more account agreements.",
		[]mcp.ToolOption{
			mcp.WithBoolean("billing_agreement", mcp.Description("Acknowledge the billing agreement (optional)")),
			mcp.WithBoolean("eu_model", mcp.Description("Acknowledge the EU model agreement (optional)")),
			mcp.WithBoolean("master_service_agreement", mcp.Description("Acknowledge the master service agreement (optional)")),
			mcp.WithBoolean("privacy_policy", mcp.Description("Acknowledge the privacy policy (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm account agreement acknowledgement. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountAgreementsAcknowledgeRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountCancelTool creates a tool for canceling the account.
func NewLinodeAccountCancelTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_cancel",
		"Cancels the active account and returns the exit survey link.",
		[]mcp.ToolOption{
			mcp.WithString("comments", mcp.Description("Reason for canceling the account or other feedback (optional).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm account cancellation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountCancelRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountUpdateTool creates a tool for updating account billing/contact fields.
func NewLinodeAccountUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_update",
		"Updates account billing and contact information. Pass dry_run=true to preview without updating.",
		[]mcp.ToolOption{
			mcp.WithString("address_1", mcp.Description("First line of the account billing address (optional)")),
			mcp.WithString("address_2", mcp.Description("Second line of the account billing address (optional)")),
			mcp.WithString("city", mcp.Description("City for the account address (optional)")),
			mcp.WithString("company", mcp.Description("Company name assigned to the account (optional)")),
			mcp.WithString("country", mcp.Description("Two-letter ISO 3166 country code (optional)")),
			mcp.WithString("email", mcp.Description("Email address assigned to the account (optional)")),
			mcp.WithString("first_name", mcp.Description("First name assigned to the account (optional)")),
			mcp.WithString("last_name", mcp.Description("Last name assigned to the account (optional)")),
			mcp.WithString("phone", mcp.Description("Phone number assigned to the account (optional)")),
			mcp.WithString("state", mcp.Description("State, province, or territory for the account address (optional)")),
			mcp.WithString("tax_id", mcp.Description("Tax identification number, or an empty string if not applicable (optional)")),
			mcp.WithString("zip", mcp.Description("Zip or postal code for the account address (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm account update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeAccountUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func accountNotificationsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountNotificationsPageSizeMin, accountNotificationsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func accountBetasPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountBetasPageSizeMin, accountBetasPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// runProfilePhoneAction is the shared path for the phone-number send and
// verify tools, which post a body to a /profile/phone-number route. It
// previews with an arg-only side effect, then confirms and runs the caller's
// client action (which returns an error message or "").
func runProfilePhoneAction(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, path, confirmMessage, successMessage string,
	body any,
	sideEffects func(context.Context) (DryRunDetails, error),
	execute func(ctx context.Context, client *linode.Client) string,
) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, toolName, httpMethodPost, path, body, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return sideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	if failureMessage := execute(ctx, client); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalToolResponse(map[string]any{responseKeyMessage: successMessage})
}

func handleLinodeProfilePhoneNumberSendRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	body, validationMessage := profilePhoneNumberRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runProfilePhoneAction(ctx, request, cfg,
		"linode_profile_phone_number_send", profilePhoneNumberPath,
		"This sends a phone number verification code. Set confirm=true to proceed.",
		"Profile phone number verification code sent successfully",
		body, profilePhoneNumberSendSideEffects,
		func(ctx context.Context, client *linode.Client) string {
			return sendProfilePhoneNumberErrorMessage(ctx, client, body)
		})
}

func sendProfilePhoneNumberErrorMessage(ctx context.Context, client *linode.Client, body *linode.ProfilePhoneNumberRequest) string {
	if err := client.SendProfilePhoneNumberVerificationCode(ctx, body); err != nil {
		return "Failed to send linode_profile_phone_number_send: " + err.Error()
	}

	return ""
}

func handleLinodeProfilePhoneNumberDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_profile_phone_number_delete", httpMethodDelete, profilePhoneNumberPath, nil)
	}

	if result := RequireConfirm(request, "This deletes the profile phone number. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	if deleteFailureMessage := deleteProfilePhoneNumberErrorMessage(ctx, client); deleteFailureMessage != "" {
		return mcp.NewToolResultError(deleteFailureMessage), nil
	}

	return MarshalToolResponse(map[string]any{responseKeyMessage: "Profile phone number deleted successfully"})
}

func deleteProfilePhoneNumberErrorMessage(ctx context.Context, client *linode.Client) string {
	if err := client.DeleteProfilePhoneNumber(ctx); err != nil {
		return "Failed to delete profile phone number: " + err.Error()
	}

	return ""
}

func profilePhoneNumberRequestFromTool(request *mcp.CallToolRequest) (*linode.ProfilePhoneNumberRequest, string) {
	args := request.GetArguments()

	isoCode, validationMessage := requiredStringArg(args, profilePhoneISOCodeParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	phoneNumber, validationMessage := requiredStringArg(args, profilePhoneNumberParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.ProfilePhoneNumberRequest{ISOCode: isoCode, PhoneNumber: phoneNumber}, ""
}

func handleLinodeProfilePhoneNumberVerifyRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	body, validationMessage := profilePhoneNumberVerifyRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runProfilePhoneAction(ctx, request, cfg,
		"linode_profile_phone_number_verify", profilePhoneNumberVerifyPath,
		"This verifies a profile phone number. Set confirm=true to proceed.",
		"Profile phone number verified successfully",
		body, profilePhoneNumberVerifySideEffects,
		func(ctx context.Context, client *linode.Client) string {
			return verifyProfilePhoneNumberErrorMessage(ctx, client, body)
		})
}

func verifyProfilePhoneNumberErrorMessage(ctx context.Context, client *linode.Client, body *linode.ProfilePhoneNumberVerifyRequest) string {
	if err := client.VerifyProfilePhoneNumber(ctx, body); err != nil {
		return "Failed to verify linode_profile_phone_number_verify: " + err.Error()
	}

	return ""
}

func handleLinodeProfileTFADisableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_profile_tfa_disable", httpMethodPost, profileTFADisablePath, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return profileTFADisableSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This disables two-factor authentication for the profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	if disableFailureMessage := disableProfileTFAErrorMessage(ctx, client); disableFailureMessage != "" {
		return mcp.NewToolResultError(disableFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.MessageResponse{
		Message: "Profile two-factor authentication disabled successfully",
	})
}

func disableProfileTFAErrorMessage(ctx context.Context, client *linode.Client) string {
	if err := client.DisableProfileTFA(ctx); err != nil {
		return "Failed to disable linode_profile_tfa_disable: " + err.Error()
	}

	return ""
}

func handleLinodeProfileTFAEnableConfirmRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	body, validationMessage := profileTFAEnableConfirmRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, "linode_profile_tfa_enable_confirm", httpMethodPost, profileTFAEnableConfirmPath, body, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return profileTFAEnableConfirmSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This enables two-factor authentication for the profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	confirmed, confirmFailureMessage := confirmProfileTFAEnableResult(ctx, client, body)
	if confirmFailureMessage != "" {
		return mcp.NewToolResultError(confirmFailureMessage), nil
	}

	return MarshalToolResponse(confirmed)
}

func confirmProfileTFAEnableResult(ctx context.Context, client *linode.Client, body *linode.ProfileTFAEnableConfirmRequest) (linode.ProfileTFAEnableConfirmResponse, string) {
	confirmed, err := client.ConfirmProfileTFAEnable(ctx, body)
	if err != nil {
		return nil, "Failed to confirm linode_profile_tfa_enable_confirm: " + err.Error()
	}

	return confirmed, ""
}

func profileTFAEnableConfirmRequestFromTool(request *mcp.CallToolRequest) (*linode.ProfileTFAEnableConfirmRequest, string) {
	tfaCode, validationMessage := requiredStringArg(request.GetArguments(), profileTFACodeParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.ProfileTFAEnableConfirmRequest{TFACode: tfaCode}, ""
}

func profilePhoneNumberVerifyRequestFromTool(request *mcp.CallToolRequest) (*linode.ProfilePhoneNumberVerifyRequest, string) {
	otpCode, validationMessage := requiredStringArg(request.GetArguments(), profilePhoneOTPCodeParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.ProfilePhoneNumberVerifyRequest{OTPCode: otpCode}, ""
}

func profileDevicesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountOAuthClientsPageSizeMin, accountOAuthClientsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeProfileLoginGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	loginID, validationMessage := accountLoginIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	login, getFailure := client.GetProfileLoginProto(ctx, loginID)
	if getFailure == nil {
		return MarshalProtoToolResponse(login)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_login_get: " + getFailure.Error()), nil
}

func handleLinodeProfileAppGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	appID, validationMessage := profileAppIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	app, getFailure := client.GetProfileAppProto(ctx, appID)
	if getFailure == nil {
		return MarshalProtoToolResponse(app)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_app_get: " + getFailure.Error()), nil
}

type profileRevokeTarget struct {
	toolName       string
	pathPrefix     string
	confirmMessage string
	fetchState     func(context.Context, *linode.Client, int) (any, error)
	deleteFailure  func(context.Context, *linode.Client, int) string
	respond        func(id int) (*mcp.CallToolResult, error)
}

func handleLinodeProfileAppDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleProfileRevokeRequest(ctx, request, cfg, profileAppIDFromTool, &profileRevokeTarget{
		toolName:       "linode_profile_app_delete",
		pathPrefix:     "/profile/apps/",
		confirmMessage: "This revokes OAuth app access. Set confirm=true to proceed.",
		fetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetProfileApp(ctx, id)
		},
		deleteFailure: deleteProfileAppErrorMessage,
		respond: func(id int) (*mcp.CallToolResult, error) {
			return MarshalProtoToolResponse(&linodev1.ProfileAppIDResponse{
				Message: fmt.Sprintf("Profile app %d revoked successfully", id),
				AppId:   linodeIDToInt32(id),
			})
		},
	})
}

func deleteProfileAppErrorMessage(ctx context.Context, client *linode.Client, appID int) string {
	if err := client.DeleteProfileApp(ctx, appID); err != nil {
		return "Failed to delete linode_profile_app_delete: " + err.Error()
	}

	return ""
}

func handleLinodeProfileDeviceRevokeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleProfileRevokeRequest(ctx, request, cfg, profileDeviceIDFromTool, &profileRevokeTarget{
		toolName:       "linode_profile_device_revoke",
		pathPrefix:     "/profile/devices/",
		confirmMessage: "This revokes a trusted device. Set confirm=true to proceed.",
		fetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetProfileDevice(ctx, id)
		},
		deleteFailure: deleteProfileDeviceErrorMessage,
		respond: func(id int) (*mcp.CallToolResult, error) {
			return MarshalProtoToolResponse(&linodev1.ProfileDeviceIDResponse{
				Message:  fmt.Sprintf("Profile trusted device %d revoked successfully", id),
				DeviceId: linodeIDToInt32(id),
			})
		},
	})
}

func deleteProfileDeviceErrorMessage(ctx context.Context, client *linode.Client, deviceID int) string {
	if err := client.DeleteProfileDevice(ctx, deviceID); err != nil {
		return "Failed to delete linode_profile_device_revoke: " + err.Error()
	}

	return ""
}

func handleProfileRevokeRequest(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	idFromTool func(*mcp.CallToolRequest) (int, string),
	target *profileRevokeTarget,
) (*mcp.CallToolResult, error) {
	id, validationMessage := idFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, target.toolName, httpMethodDelete,
			target.pathPrefix+strconv.Itoa(id),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return target.fetchState(ctx, c, id)
			})
	}

	if result := RequireConfirm(request, target.confirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if deleteFailureMessage := target.deleteFailure(ctx, client, id); deleteFailureMessage != "" {
		return mcp.NewToolResultError(deleteFailureMessage), nil
	}

	return target.respond(id)
}

func profileAppIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[profileAppIDParam]
	if !exists {
		return 0, "app_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 {
			return 0, errProfileAppIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > profileAppIDMaxFromJSON || value != float64(int64(value)) {
			return 0, errProfileAppIDPositive
		}

		return int(value), ""
	default:
		return 0, errProfileAppIDPositive
	}
}

func handleLinodeProfileDeviceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	deviceID, validationMessage := profileDeviceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	device, getFailure := client.GetProfileDevice(ctx, deviceID)
	if getFailure == nil {
		return MarshalToolResponse(device)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_device_get: " + getFailure.Error()), nil
}

func profileDeviceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[profileDeviceIDParam]
	if !exists {
		return 0, "device_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 {
			return 0, errProfileDeviceIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > profileDeviceIDMaxFromJSON || value != float64(int64(value)) {
			return 0, errProfileDeviceIDPositive
		}

		return int(value), ""
	default:
		return 0, errProfileDeviceIDPositive
	}
}

func accountOAuthClientsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountOAuthClientsPageSizeMin, accountOAuthClientsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func profileAppsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountOAuthClientsPageSizeMin, accountOAuthClientsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func longviewClientsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", longviewClientsPageSizeMin, longviewClientsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeLongviewClientUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := longviewClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		if _, dryRunValidationMessage := longviewClientUpdateRequestFromTool(request); dryRunValidationMessage != "" {
			return mcp.NewToolResultError(dryRunValidationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_longview_client_update", "PUT",
			fmt.Sprintf(longviewClientsPath+"/%d", clientID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLongviewClient(ctx, strconv.Itoa(clientID))
			})
	}

	if result := RequireConfirm(request, "This updates a Longview client. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := longviewClientUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	longviewClient, updateFailureMessage := updateLongviewClient(ctx, client, clientID, req)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError("Failed to update linode_longview_client_update: " + updateFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.LongviewClientWriteResponse{
		Message:        "Longview client updated successfully",
		LongviewClient: longviewClient,
	})
}

func longviewClientIDFromTool(request *mcp.CallToolRequest) (int, string) {
	value, exists := request.GetArguments()[longviewClientIDParam]
	if !exists {
		return 0, errLongviewClientIDPositive
	}

	switch typed := value.(type) {
	case int:
		if typed <= 0 {
			return 0, errLongviewClientIDPositive
		}

		return typed, ""
	case float64:
		if typed <= 0 || typed > maxLongviewClientIDFromJSON || math.Trunc(typed) != typed {
			return 0, errLongviewClientIDPositive
		}

		return int(typed), ""
	default:
		return 0, errLongviewClientIDPositive
	}
}

func longviewClientUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateLongviewClientRequest, string) {
	args := request.GetArguments()

	label, hasLabel, validationMessage := optionalStringField(args, "label")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if !hasLabel {
		return nil, errLongviewClientLabelRequired
	}

	if !validLongviewClientLabel(label) {
		return nil, errLongviewClientLabelPattern
	}

	return &linode.UpdateLongviewClientRequest{Label: &label}, ""
}

func validLongviewClientLabel(label string) bool {
	if len(label) < 3 || len(label) > 32 {
		return false
	}

	for _, char := range label {
		if char >= 'a' && char <= 'z' {
			continue
		}

		if char >= 'A' && char <= 'Z' {
			continue
		}

		if char >= '0' && char <= '9' {
			continue
		}

		if char == '-' || char == '_' {
			continue
		}

		return false
	}

	return true
}

func updateLongviewClient(ctx context.Context, client *linode.Client, clientID int, req *linode.UpdateLongviewClientRequest) (*linodev1.LongviewClient, string) {
	longviewClient, err := client.UpdateLongviewClientProto(ctx, clientID, req)
	if err != nil {
		return nil, err.Error()
	}

	return longviewClient, ""
}

func handleLinodeLongviewClientDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := longviewClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_longview_client_delete", httpMethodDelete,
			fmt.Sprintf(longviewClientsPath+"/%d", clientID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLongviewClient(ctx, strconv.Itoa(clientID))
			})
	}

	if result := RequireConfirm(request, "This deletes a Longview client. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deleteFailureMessage := deleteLongviewClient(ctx, client, clientID)
	if deleteFailureMessage != "" {
		return mcp.NewToolResultError("Failed to delete linode_longview_client_delete: " + deleteFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.LongviewClientIDResponse{
		Message:  "Longview client deleted successfully",
		ClientId: linodeIDToInt32(clientID),
	})
}

func deleteLongviewClient(ctx context.Context, client *linode.Client, clientID int) string {
	if err := client.DeleteLongviewClient(ctx, clientID); err != nil {
		return err.Error()
	}

	return ""
}

func accountPaymentMethodsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountPaymentMethodsPageSizeMin, accountPaymentMethodsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeLongviewSubscriptionGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	subscriptionID, validationMessage := longviewSubscriptionIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subscription, getFailure := client.GetLongviewSubscriptionProto(ctx, subscriptionID)
	if getFailure == nil {
		return MarshalProtoToolResponse(subscription)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_longview_subscription_get: " + getFailure.Error()), nil
}

func longviewSubscriptionIDFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, exists := request.GetArguments()[longviewSubscriptionIDParam]
	if !exists {
		return "", "subscription_id is required"
	}

	subscriptionID, ok := raw.(string)
	if !ok || strings.TrimSpace(subscriptionID) == "" {
		return "", "subscription_id must be a non-empty string"
	}

	if subscriptionID != strings.TrimSpace(subscriptionID) || strings.Contains(subscriptionID, "/") || strings.Contains(subscriptionID, "?") || strings.Contains(subscriptionID, "#") || strings.Contains(subscriptionID, "..") {
		return "", "subscription_id must not contain path separators, query separators, or traversal segments"
	}

	return subscriptionID, ""
}

func handleLinodeLongviewClientGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := longviewClientGetIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	longviewClient, getFailure := client.GetLongviewClientProto(ctx, clientID)
	if getFailure == nil {
		return MarshalProtoToolResponse(longviewClient)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_longview_client_get: " + getFailure.Error()), nil
}

func longviewClientGetIDFromTool(request *mcp.CallToolRequest) (string, string) {
	id, validationMessage := optionalPaginationInt(request.GetArguments(), "client_id", 1, 0)
	if validationMessage != "" {
		return "", validationMessage
	}

	if id == 0 {
		return "", "client_id is required"
	}

	return strconv.Itoa(id), ""
}

func handleLinodeAccountPaymentMethodGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	paymentMethodID, validationMessage := accountPaymentMethodIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	method, getFailure := client.GetAccountPaymentMethod(ctx, paymentMethodID)
	if getFailure == nil {
		return MarshalToolResponse(method)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_payment_method_get: " + getFailure.Error()), nil
}

func accountPaymentMethodIDFromTool(request *mcp.CallToolRequest) (string, string) {
	id, validationMessage := optionalPaginationInt(request.GetArguments(), "payment_method_id", 1, 0)
	if validationMessage != "" {
		return "", validationMessage
	}

	if id == 0 {
		return "", "payment_method_id is required"
	}

	return strconv.Itoa(id), ""
}

func handleLinodeAccountPaymentMethodCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := paymentMethodCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_payment_method_create", httpMethodPost, accountPaymentMethodsPath, nil)
	}

	if result := RequireConfirm(request, "This creates a payment method. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	method, createFailureMessage := createAccountPaymentMethod(ctx, client, req)
	if createFailureMessage != "" {
		return mcp.NewToolResultError("Failed to create linode_account_payment_method_create: " + createFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountPaymentMethodWriteResponse{
		Message:       "Payment method created successfully",
		PaymentMethod: method,
	})
}

func paymentMethodCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateAccountPaymentMethodRequest, string) {
	args := request.GetArguments()

	paymentType, typeOK := args["type"].(string)
	if !typeOK || strings.TrimSpace(paymentType) == "" {
		return nil, errPaymentMethodTypeRequired
	}

	data, dataOK := args["data"].(map[string]any)
	if !dataOK || len(data) == 0 {
		return nil, errPaymentMethodDataRequired
	}

	isDefault, isDefaultOK := args["is_default"].(bool)
	if !isDefaultOK {
		return nil, "is_default must be a boolean"
	}

	return &linode.CreateAccountPaymentMethodRequest{Type: paymentType, Data: data, IsDefault: isDefault}, ""
}

func createAccountPaymentMethod(ctx context.Context, client *linode.Client, req *linode.CreateAccountPaymentMethodRequest) (*linodev1.AccountPaymentMethod, string) {
	method, err := client.CreateAccountPaymentMethodProto(ctx, req)
	if err != nil {
		return nil, err.Error()
	}

	return method, ""
}

func handleLinodeProfileTFAEnableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_profile_tfa_enable", httpMethodPost, "/profile/tfa-enable", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return profileTFAEnableSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This generates a two-factor authentication secret. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	secret, enableFailureMessage := enableProfileTFA(ctx, client)
	if enableFailureMessage != "" {
		return mcp.NewToolResultError("Failed to generate linode_profile_tfa_enable: " + enableFailureMessage), nil
	}

	// The one-time secret is returned to the user by design (it must be confirmed
	// to activate two-factor auth), so it is not output-redacted.
	secret.Warning = "IMPORTANT: Save this two-factor authentication secret now. " +
		"It must be confirmed before two-factor authentication is enabled."

	return MarshalProtoToolResponse(secret)
}

func enableProfileTFA(ctx context.Context, client *linode.Client) (*linodev1.ProfileTfaEnableResponse, string) {
	secret, err := client.EnableProfileTFAProto(ctx)
	if err != nil {
		return nil, err.Error()
	}

	return secret, ""
}

func handleLinodeAccountPaymentMethodDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	paymentMethodID, validationMessage := accountPaymentMethodIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_payment_method_delete", httpMethodDelete,
			accountPaymentMethodsPath+"/"+paymentMethodID,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountPaymentMethod(ctx, paymentMethodID)
			})
	}

	if result := RequireConfirm(request, "This deletes a payment method. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deleteFailureMessage := deleteAccountPaymentMethod(ctx, client, paymentMethodID)
	if deleteFailureMessage != "" {
		return mcp.NewToolResultError("Failed to delete linode_account_payment_method_delete: " + deleteFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountPaymentMethodIDResponse{
		Message:         "Payment method deleted successfully",
		PaymentMethodId: paymentMethodIDInt(paymentMethodID),
	})
}

func deleteAccountPaymentMethod(ctx context.Context, client *linode.Client, paymentMethodID string) string {
	if err := client.DeleteAccountPaymentMethod(ctx, paymentMethodID); err != nil {
		return err.Error()
	}

	return ""
}

func handleLinodeAccountPaymentMethodMakeDefaultRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	paymentMethodID, validationMessage := accountPaymentMethodIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_payment_method_make_default", httpMethodPost,
			accountPaymentMethodsPath+"/"+paymentMethodID+"/make-default",
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountPaymentMethod(ctx, paymentMethodID)
			})
	}

	if result := RequireConfirm(request, "This changes the default payment method. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	makeDefaultFailureMessage := makeAccountPaymentMethodDefault(ctx, client, paymentMethodID)
	if makeDefaultFailureMessage != "" {
		return mcp.NewToolResultError("Failed to set linode_account_payment_method_make_default: " + makeDefaultFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountPaymentMethodIDResponse{
		Message:         "Payment method set as default successfully",
		PaymentMethodId: paymentMethodIDInt(paymentMethodID),
	})
}

// paymentMethodIDInt converts the validated payment-method ID string back to an
// int32 for the id-echo response. The string comes from accountPaymentMethodIDFromTool,
// which already validated it as a positive integer, so the parse cannot fail.
func paymentMethodIDInt(paymentMethodID string) int32 {
	id, _ := strconv.Atoi(paymentMethodID)

	return linodeIDToInt32(id)
}

func makeAccountPaymentMethodDefault(ctx context.Context, client *linode.Client, paymentMethodID string) string {
	if err := client.MakeAccountPaymentMethodDefault(ctx, paymentMethodID); err != nil {
		return err.Error()
	}

	return ""
}

func handleLinodeAccountOAuthClientGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := accountOAuthClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	oauthClient, getFailure := client.GetAccountOAuthClientProto(ctx, clientID)
	if getFailure == nil {
		return MarshalProtoToolResponse(oauthClient)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_oauth_client_get: " + getFailure.Error()), nil
}

func accountOAuthClientIDFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, exists := request.GetArguments()["client_id"]
	if !exists {
		return "", "client_id is required"
	}

	clientID, ok := raw.(string)
	if !ok || strings.TrimSpace(clientID) == "" {
		return "", "client_id must be a non-empty string"
	}

	if clientID != strings.TrimSpace(clientID) || strings.Contains(clientID, "/") || strings.Contains(clientID, "?") || strings.Contains(clientID, "..") {
		return "", "client_id must not contain path separators, query separators, or traversal segments"
	}

	return clientID, ""
}

func handleLinodeAccountOAuthClientCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := oauthClientCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_oauth_client_create", httpMethodPost, accountOAuthClientsPath, nil)
	}

	if result := RequireConfirm(request, "This creates an OAuth client. The secret is only shown once. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	oauthClient, createFailureMessage := createOAuthClient(ctx, client, req)
	if createFailureMessage != "" {
		return mcp.NewToolResultError("Failed to create linode_account_oauth_client_create: " + createFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.OAuthClientCreateWriteResponse{
		Message: "OAuth client created successfully",
		Warning: "IMPORTANT: The secret below is shown ONLY ONCE. Save it now - it cannot be retrieved later.",
		Client:  oauthClient,
	})
}

func createOAuthClient(ctx context.Context, client *linode.Client, req *linode.CreateOAuthClientRequest) (*linodev1.CreatedOAuthClient, string) {
	oauthClient, err := client.CreateOAuthClientProto(ctx, req)
	if err != nil {
		return nil, err.Error()
	}

	return oauthClient, ""
}

func handleLinodeAccountOAuthClientUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := accountOAuthClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := oauthClientUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_oauth_client_update", "PUT",
			accountOAuthClientsPath+"/"+clientID,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountOAuthClient(ctx, clientID)
			})
	}

	if result := RequireConfirm(request, "This updates an OAuth client. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	oauthClient, updateFailureMessage := updateOAuthClient(ctx, client, clientID, req)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError("Failed to update linode_account_oauth_client_update: " + updateFailureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message string              `json:"message"`
		Client  *linode.OAuthClient `json:"client"`
	}{
		Message: "OAuth client updated successfully",
		Client:  oauthClient,
	})
}

func updateOAuthClient(ctx context.Context, client *linode.Client, clientID string, req *linode.UpdateOAuthClientRequest) (*linode.OAuthClient, string) {
	oauthClient, err := client.UpdateOAuthClient(ctx, clientID, req)
	if err != nil {
		return nil, err.Error()
	}

	return oauthClient, ""
}

func handleLinodeAccountOAuthClientThumbnailUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := accountOAuthClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	thumbnailPNG, validationMessage := oauthClientThumbnailPNGFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_oauth_client_thumbnail_update", "PUT",
			accountOAuthClientsPath+"/"+clientID+"/thumbnail",
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountOAuthClient(ctx, clientID)
			})
	}

	if result := RequireConfirm(request, "This updates an OAuth client thumbnail. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updateFailureMessage := updateOAuthClientThumbnail(ctx, client, clientID, thumbnailPNG)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError("Failed to update linode_account_oauth_client_thumbnail_update: " + updateFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.OAuthClientIDResponse{
		Message:  "OAuth client thumbnail updated successfully",
		ClientId: clientID,
	})
}

func oauthClientThumbnailPNGFromTool(request *mcp.CallToolRequest) ([]byte, string) {
	raw, exists := request.GetArguments()[oauthClientThumbnailPNGParam]
	if !exists {
		return nil, errThumbnailPNGRequired
	}

	encoded, ok := raw.(string)
	if !ok || strings.TrimSpace(encoded) == "" {
		return nil, "thumbnail_png_base64 must be a non-empty string"
	}

	thumbnailPNG, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "thumbnail_png_base64 must be valid standard base64"
	}

	return thumbnailPNG, ""
}

func updateOAuthClientThumbnail(ctx context.Context, client *linode.Client, clientID string, thumbnailPNG []byte) string {
	err := client.UpdateOAuthClientThumbnail(ctx, clientID, thumbnailPNG)
	if err != nil {
		return err.Error()
	}

	return ""
}

func handleLinodeAccountOAuthClientThumbnailGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := accountOAuthClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	thumbnailPNG, err := client.GetOAuthClientThumbnail(ctx, clientID)
	if err != nil {
		//nolint:nilerr // MCP tool errors are returned in the result, not as Go errors
		return mcp.NewToolResultError("Failed to get OAuth client thumbnail: " + err.Error()), nil
	}

	encoded := base64.StdEncoding.EncodeToString(thumbnailPNG)

	return MarshalToolResponse(struct {
		ClientID           string `json:"client_id"`
		ThumbnailPNGBase64 string `json:"thumbnail_png_base64"`
	}{
		ClientID:           clientID,
		ThumbnailPNGBase64: encoded,
	})
}

func handleLinodeAccountOAuthClientDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := accountOAuthClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_oauth_client_delete", httpMethodDelete,
			accountOAuthClientsPath+"/"+clientID,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountOAuthClient(ctx, clientID)
			})
	}

	if result := RequireConfirm(request, "This deletes an OAuth client. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deleteFailureMessage := deleteOAuthClient(ctx, client, clientID)
	if deleteFailureMessage != "" {
		return mcp.NewToolResultError("Failed to delete linode_account_oauth_client_delete: " + deleteFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.OAuthClientIDResponse{
		Message:  "OAuth client deleted successfully",
		ClientId: clientID,
	})
}

func deleteOAuthClient(ctx context.Context, client *linode.Client, clientID string) string {
	deleteFailure := client.DeleteAccountOAuthClient(ctx, clientID)
	if deleteFailure != nil {
		return deleteFailure.Error()
	}

	return ""
}

func handleLinodeAccountOAuthClientResetSecretRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	clientID, validationMessage := accountOAuthClientIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		// Credential-safe: fetch the client metadata (not the secret) and
		// preview the POST; the new secret is never surfaced.
		return RunDryRunPreview(ctx, request, cfg, "linode_account_oauth_client_secret_reset", httpMethodPost,
			accountOAuthClientsPath+"/"+clientID+"/reset-secret",
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountOAuthClient(ctx, clientID)
			})
	}

	if result := RequireConfirm(request, "This resets an OAuth client secret. The new secret is only shown once. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	secret, resetFailureMessage := resetOAuthClientSecret(ctx, client, clientID)
	if resetFailureMessage != "" {
		return mcp.NewToolResultError("Failed to reset linode_account_oauth_client_secret_reset: " + resetFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.OAuthClientSecretResetWriteResponse{
		Message:  "OAuth client secret reset successfully",
		Warning:  "IMPORTANT: The new secret below is shown ONLY ONCE. Save it now - it cannot be retrieved later.",
		ClientId: clientID,
		Secret:   secret,
	})
}

func resetOAuthClientSecret(ctx context.Context, client *linode.Client, clientID string) (*linodev1.OAuthClientSecret, string) {
	secret, err := client.ResetOAuthClientSecretProto(ctx, clientID)
	if err != nil {
		return nil, err.Error()
	}

	return secret, ""
}

func oauthClientCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateOAuthClientRequest, string) {
	args := request.GetArguments()

	label, labelOK := args["label"].(string)
	if !labelOK || strings.TrimSpace(label) == "" {
		return nil, errLabelRequired
	}

	redirectURI, redirectURIOK := args["redirect_uri"].(string)
	if !redirectURIOK || strings.TrimSpace(redirectURI) == "" {
		return nil, errRedirectURIRequired
	}

	return &linode.CreateOAuthClientRequest{Label: label, RedirectURI: redirectURI}, ""
}

func oauthClientUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateOAuthClientRequest, string) {
	args := request.GetArguments()
	req := &linode.UpdateOAuthClientRequest{}

	var hasUpdate bool

	if raw, exists := args["label"]; exists {
		label, ok := raw.(string)
		if !ok || strings.TrimSpace(label) == "" {
			return nil, errLabelRequired
		}

		req.Label = &label
		hasUpdate = true
	}

	if raw, exists := args["redirect_uri"]; exists {
		redirectURI, ok := raw.(string)
		if !ok || strings.TrimSpace(redirectURI) == "" {
			return nil, errRedirectURIRequired
		}

		req.RedirectURI = &redirectURI
		hasUpdate = true
	}

	if raw, exists := args["public"]; exists {
		public, ok := raw.(bool)
		if !ok {
			return nil, "public must be a boolean"
		}

		req.Public = &public
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, "at least one of label, redirect_uri, or public is required"
	}

	return req, ""
}

func accountEventsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountEventsPageSizeMin, accountEventsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeTaggedObjectsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tagLabel, page, pageSize, validationMessage := taggedObjectsArgsFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	taggedObjects, listFailure := client.ListTaggedObjectsProto(ctx, tagLabel, page, pageSize)
	if listFailure == nil {
		return finishProtoList(request, taggedObjects, nil, taggedObjectListResponse)
	}

	return mcp.NewToolResultError("Failed to retrieve items: " + listFailure.Error()), nil
}

func taggedObjectListResponse(items []*linodev1.TaggedObject, count int32, filter *string) *linodev1.TaggedObjectListResponse {
	return &linodev1.TaggedObjectListResponse{Count: count, Filter: filter, TaggedObjects: items}
}

func taggedObjectsArgsFromTool(request *mcp.CallToolRequest) (string, int, int, string) {
	args := request.GetArguments()

	tagLabel, validationMessage := tagLabelArgFromTool(request)
	if validationMessage != "" {
		return "", 0, 0, validationMessage
	}

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return "", 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", taggedObjectsPageSizeMin, taggedObjectsPageSizeMax)
	if validationMessage != "" {
		return "", 0, 0, validationMessage
	}

	return tagLabel, page, pageSize, ""
}

func accountUsersPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountUsersPageSizeMin, accountUsersPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeAccountUserGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	username, validationMessage := accountUserUsernamePathParamFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	user, getFailure := client.GetAccountUserProto(ctx, username)
	if getFailure == nil {
		return MarshalProtoToolResponse(user)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_user_get: " + getFailure.Error()), nil
}

func handleLinodeProfileTokenGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenID, validationMessage := profileTokenIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	token, getFailure := client.GetProfileToken(ctx, tokenID)
	if getFailure == nil {
		return MarshalToolResponse(token)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_token_get: " + getFailure.Error()), nil
}

func profileTokenIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredPositiveIntArgument(request, profileTokenIDParam, errProfileTokenIDPositive, errProfileTokenIDPositive)
}

func handleLinodeAccountUserGrantsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	username, validationMessage := accountUserUsernamePathParamFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	grants, getFailure := client.GetAccountUserGrants(ctx, username)
	if getFailure == nil {
		return MarshalToolResponse(grants)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_user_grants_get: " + getFailure.Error()), nil
}

func handleLinodeAccountUserGrantsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	username, validationMessage := accountUserUsernamePathParamFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateRequest, validationMessage := accountUserGrantsUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_user_grants_update", "PUT",
			accountUsersPath+"/"+username+"/grants",
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountUserGrants(ctx, username)
			})
	}

	if result := RequireConfirm(request, "This updates account user grants. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	grants, updateFailure := client.UpdateAccountUserGrantsProto(ctx, username, updateRequest)
	if updateFailure == nil {
		return MarshalProtoToolResponse(&linodev1.AccountUserGrantsWriteResponse{
			Message: "Account user grants updated successfully",
			Grants:  grants,
		})
	}

	return mcp.NewToolResultError("Failed to update linode_account_user_grants_update: " + updateFailure.Error()), nil
}

func accountUserUsernamePathParamFromTool(request *mcp.CallToolRequest) (string, string) {
	username, validationMessage := requiredAccountUserString(request.GetArguments(), accountUserUsernameParam)
	if validationMessage != "" {
		return "", validationMessage
	}

	if strings.ContainsAny(username, "/?#") || strings.Contains(username, "..") {
		return "", errAccountUserUsernamePathParam
	}

	return username, ""
}

func handleLinodeAccountUserUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	username, validationMessage := accountUserUsernamePathParamFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updateRequest, validationMessage := accountUserUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_user_update", "PUT",
			accountUsersPath+"/"+username,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountUser(ctx, username)
			})
	}

	if result := RequireConfirm(request, "This updates an account user. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	user, updateFailure := client.UpdateAccountUserProto(ctx, username, updateRequest)
	if updateFailure == nil {
		return MarshalProtoToolResponse(&linodev1.AccountUserWriteResponse{
			Message: "Account user updated successfully",
			User:    user,
		})
	}

	return mcp.NewToolResultError("Failed to update linode_account_user_update: " + updateFailure.Error()), nil
}

func handleLinodeAccountUserDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	username, validationMessage := accountUserUsernamePathParamFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_user_delete", httpMethodDelete,
			accountUsersPath+"/"+username,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountUser(ctx, username)
			})
	}

	if result := RequireConfirm(request, "This deletes an account user. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if deleteFailureMessage := deleteAccountUserErrorMessage(ctx, client, username); deleteFailureMessage != "" {
		return mcp.NewToolResultError(deleteFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountUserDeleteResponse{
		Message:  "Account user deleted successfully",
		Username: username,
	})
}

func deleteAccountUserErrorMessage(ctx context.Context, client *linode.Client, username string) string {
	if err := client.DeleteAccountUser(ctx, username); err != nil {
		return "Failed to delete linode_account_user_delete: " + err.Error()
	}

	return ""
}

func accountUserGrantsUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateAccountUserGrantsRequest, string) {
	args := request.GetArguments()
	req := &linode.UpdateAccountUserGrantsRequest{}

	var hasUpdate bool

	if raw, exists := args[accountUserGrantsGlobalParam]; exists {
		var global linode.UpdateAccountUserGlobalGrants
		if !decodeToolJSONValue(raw, &global) || !accountUserGlobalGrantUpdateValid(&global) {
			return nil, errAccountUserGrantsGlobalObject
		}

		req.Global = &global
		hasUpdate = true
	}

	sections := []struct {
		name string
		set  func(*[]linode.UpdateAccountUserGrant)
	}{
		{name: accountUserGrantsLinodeParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Linode = grants }},
		{name: accountUserGrantsDomainParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Domain = grants }},
		{name: accountUserGrantsNodeBalancerParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.NodeBalancer = grants }},
		{name: accountUserGrantsImageParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Image = grants }},
		{name: accountUserGrantsLongviewParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Longview = grants }},
		{name: accountUserGrantsStackScriptParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.StackScript = grants }},
		{name: accountUserGrantsVolumeParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Volume = grants }},
		{name: accountUserGrantsDatabaseParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Database = grants }},
		{name: accountUserGrantsFirewallParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.Firewall = grants }},
		{name: accountUserGrantsVPCParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.VPC = grants }},
		{name: accountUserGrantsLKEClusterParam, set: func(grants *[]linode.UpdateAccountUserGrant) { req.LKECluster = grants }},
	}

	for _, section := range sections {
		raw, exists := args[section.name]
		if !exists {
			continue
		}

		var grants []linode.UpdateAccountUserGrant
		if !decodeToolJSONValue(raw, &grants) || !accountUserGrantUpdatesValid(grants) {
			return nil, errAccountUserGrantsArray
		}

		section.set(&grants)

		hasUpdate = true
	}

	if !hasUpdate {
		return nil, errAccountUserGrantsUpdateEmpty
	}

	return req, ""
}

func accountUserGlobalGrantUpdateValid(global *linode.UpdateAccountUserGlobalGrants) bool {
	if global.AccountAccess == nil && global.AddDatabases == nil && global.AddDomains == nil && global.AddFirewalls == nil && global.AddImages == nil && global.AddLinodes == nil && global.AddLongview == nil && global.AddNodeBalancers == nil && global.AddStackScripts == nil && global.AddVolumes == nil && global.AddVPCs == nil && global.CancelAccount == nil && global.ChildAccountAccess == nil && global.LongviewSubscription == nil {
		return false
	}

	if global.AccountAccess == nil {
		return true
	}

	switch *global.AccountAccess {
	case "", grantPermissionReadOnly, grantPermissionReadWrite:
		return true
	default:
		return false
	}
}

func accountUserGrantUpdatesValid(grants []linode.UpdateAccountUserGrant) bool {
	for _, grant := range grants {
		if grant.ID <= 0 || grant.Permissions == nil {
			return false
		}

		switch *grant.Permissions {
		case "", grantPermissionReadOnly, grantPermissionReadWrite:
		default:
			return false
		}
	}

	return true
}

func decodeToolJSONValue(raw, target any) bool {
	if raw == nil {
		return false
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return false
	}

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()

	return decoder.Decode(target) == nil
}

func accountUserUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateAccountUserRequest, string) {
	args := request.GetArguments()
	req := &linode.UpdateAccountUserRequest{}

	var hasUpdate bool

	if raw, exists := args[accountUserEmailParam]; exists {
		email, validationMessage := nonEmptyToolString(raw, "email")
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.Email = &email
		hasUpdate = true
	}

	if raw, exists := args["restricted"]; exists {
		restricted, validationMessage := boolToolArg(raw, "restricted")
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.Restricted = &restricted
		hasUpdate = true
	}

	if raw, exists := args["ssh_keys"]; exists {
		sshKeys, validationMessage := accountUserSSHKeysFromTool(raw)
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.SSHKeys = sshKeys
		hasUpdate = true
	}

	if raw, exists := args["new_username"]; exists {
		newUsername, validationMessage := nonEmptyToolString(raw, "new_username")
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.Username = &newUsername
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, "at least one account user field is required"
	}

	return req, ""
}

func nonEmptyToolString(raw any, field string) (string, string) {
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", field + " must be a non-empty string"
	}

	return value, ""
}

func boolToolArg(raw any, field string) (bool, string) {
	value, ok := raw.(bool)
	if !ok {
		return false, field + " must be a boolean"
	}

	return value, ""
}

func accountUserSSHKeysFromTool(raw any) (*[]string, string) {
	rawKeys, ok := raw.([]any)
	if !ok {
		return nil, errAccountUserUpdateSSHKeys
	}

	sshKeys := make([]string, 0, len(rawKeys))
	for _, rawKey := range rawKeys {
		sshKey, ok := rawKey.(string)
		if !ok || strings.TrimSpace(sshKey) == "" {
			return nil, errAccountUserUpdateSSHKeys
		}

		sshKeys = append(sshKeys, sshKey)
	}

	return &sshKeys, ""
}

func handleLinodeAccountUserCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	createRequest, validationMessage := accountUserCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_user_create", httpMethodPost, accountUsersPath, nil)
	}

	if result := RequireConfirm(request, "This creates an account user. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	user, createFailure := client.CreateAccountUserProto(ctx, createRequest)
	if createFailure == nil {
		return MarshalProtoToolResponse(&linodev1.AccountUserWriteResponse{
			Message: "Account user created successfully",
			User:    user,
		})
	}

	return mcp.NewToolResultError("Failed to create linode_account_user_create: " + createFailure.Error()), nil
}

func accountUserCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateAccountUserRequest, string) {
	args := request.GetArguments()

	username, validationMessage := requiredAccountUserString(args, accountUserUsernameParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	email, validationMessage := requiredAccountUserString(args, accountUserEmailParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	req := &linode.CreateAccountUserRequest{Username: username, Email: email}

	if raw, exists := args["restricted"]; exists {
		restricted, restrictedMessage := boolToolArg(raw, "restricted")
		if restrictedMessage != "" {
			return nil, restrictedMessage
		}

		req.Restricted = &restricted
	}

	return req, ""
}

func handleLinodeAccountSupportTicketCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	createRequest, validationMessage := supportTicketCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, "linode_support_ticket_create", httpMethodPost, supportTicketsPath, createRequest, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return supportTicketCreateSideEffects(ctx, request.GetString("summary", ""))
			})
	}

	if result := RequireConfirm(request, "This creates a support ticket. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ticket, createFailure := client.CreateSupportTicketProto(ctx, createRequest)
	if createFailure == nil {
		return MarshalProtoToolResponse(&linodev1.SupportTicketWriteResponse{
			Message: "Support ticket opened successfully",
			Ticket:  ticket,
		})
	}

	return mcp.NewToolResultError("Failed to create linode_support_ticket_create: " + createFailure.Error()), nil
}

func supportTicketCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateSupportTicketRequest, string) {
	args := request.GetArguments()

	summary, validationMessage := requiredAccountUserString(args, supportTicketSummaryParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	description, validationMessage := requiredAccountUserString(args, supportTicketDescriptionParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	req := &linode.CreateSupportTicketRequest{Summary: summary, Description: description}

	optionalStrings := []struct {
		field string
		set   func(string)
	}{
		{field: supportTicketBucketParam, set: func(value string) { req.Bucket = &value }},
		{field: supportTicketRegionParam, set: func(value string) { req.Region = &value }},
		{field: supportTicketVLANParam, set: func(value string) { req.VLAN = &value }},
	}
	for _, optional := range optionalStrings {
		value, validationMessage := optionalSupportTicketString(args, optional.field)
		if validationMessage != "" {
			return nil, validationMessage
		}

		if value != "" {
			optional.set(value)
		}
	}

	optionalIDs := []struct {
		field string
		set   func(int)
	}{
		{field: supportTicketDatabaseIDParam, set: func(value int) { req.DatabaseID = &value }},
		{field: supportTicketDomainIDParam, set: func(value int) { req.DomainID = &value }},
		{field: supportTicketFirewallIDParam, set: func(value int) { req.FirewallID = &value }},
		{field: supportTicketLinodeIDParam, set: func(value int) { req.LinodeID = &value }},
		{field: supportTicketLKEClusterIDParam, set: func(value int) { req.LKEClusterID = &value }},
		{field: supportTicketLongviewClientIDParam, set: func(value int) { req.LongviewClientID = &value }},
		{field: supportTicketNodeBalancerIDParam, set: func(value int) { req.NodeBalancerID = &value }},
		{field: supportTicketSeverityParam, set: func(value int) { req.Severity = &value }},
		{field: supportTicketVolumeIDParam, set: func(value int) { req.VolumeID = &value }},
		{field: supportTicketVPCIDParam, set: func(value int) { req.VPCID = &value }},
	}
	for _, optional := range optionalIDs {
		value, validationMessage := optionalSupportTicketPositiveID(args, optional.field)
		if validationMessage != "" {
			return nil, validationMessage
		}

		if value > 0 {
			optional.set(value)
		}
	}

	if raw, present := args[supportTicketManagedIssueParam]; present && raw != nil {
		managed, ok := raw.(bool)
		if !ok {
			return nil, supportTicketManagedIssueParam + " must be a boolean"
		}

		req.ManagedIssue = &managed
	}

	return req, ""
}

func optionalSupportTicketString(args map[string]any, field string) (string, string) {
	raw, rawPresent := args[field]
	if !rawPresent || raw == nil {
		return "", ""
	}

	return nonEmptyToolString(raw, field)
}

func optionalSupportTicketPositiveID(args map[string]any, field string) (int, string) {
	raw, rawPresent := args[field]
	if !rawPresent || raw == nil {
		return 0, ""
	}

	value, ok := raw.(float64)
	if !ok || value <= 0 || math.Trunc(value) != value || value > maxSupportTicketResourceIDFromJSON {
		return 0, field + " must be a positive integer"
	}

	return int(value), ""
}

// runSupportTicketSubresourceCreate is the shared path for the reply and
// attachment create tools, which post a body to a /support/tickets/{id}/<sub>
// route. It previews with the ticket-scoped side effect, then confirms and
// executes via the caller's typed client call.
func runSupportTicketSubresourceCreate(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, path, confirmMessage, failurePrefix string,
	ticketID int,
	body any,
	sideEffects func(context.Context, int) (DryRunDetails, error),
	execute func(ctx context.Context, client *linode.Client) (*mcp.CallToolResult, string),
) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, toolName, httpMethodPost, path, body, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return sideEffects(ctx, ticketID)
			})
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, failureMessage := execute(ctx, client)
	if failureMessage != "" {
		return mcp.NewToolResultError(failurePrefix + failureMessage), nil
	}

	return result, nil
}

func handleLinodeAccountSupportTicketReplyCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	ticketID, createRequest, validationMessage := supportTicketReplyCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runSupportTicketSubresourceCreate(ctx, request, cfg,
		"linode_support_ticket_reply_create",
		fmt.Sprintf("%s/%d/replies", supportTicketsPath, ticketID),
		"This creates a support ticket reply. Set confirm=true to proceed.",
		"Failed to create support ticket reply: ",
		ticketID, createRequest, supportTicketReplyCreateSideEffects,
		func(ctx context.Context, client *linode.Client) (*mcp.CallToolResult, string) {
			reply, replyFailure := client.CreateSupportTicketReplyProto(ctx, ticketID, createRequest)
			if replyFailure != nil {
				return nil, replyFailure.Error()
			}

			result, marshalErr := MarshalProtoToolResponse(&linodev1.SupportTicketReplyWriteResponse{
				Message: "Support ticket reply created successfully",
				Reply:   reply,
			})
			if marshalErr != nil {
				return nil, marshalErr.Error()
			}

			return result, ""
		})
}

func supportTicketReplyCreateRequestFromTool(request *mcp.CallToolRequest) (int, *linode.CreateSupportTicketReplyRequest, string) {
	args := request.GetArguments()

	ticketID, validationMessage := requiredSupportTicketAttachmentTicketID(args)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	description, validationMessage := requiredAccountUserString(args, supportTicketDescriptionParam)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	return ticketID, &linode.CreateSupportTicketReplyRequest{Description: description}, ""
}

func handleLinodeAccountSupportTicketAttachmentCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	ticketID, createRequest, validationMessage := supportTicketAttachmentCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return runSupportTicketSubresourceCreate(ctx, request, cfg,
		"linode_support_ticket_attachment_create",
		fmt.Sprintf("%s/%d/attachments", supportTicketsPath, ticketID),
		"This creates a support ticket attachment. Set confirm=true to proceed.",
		"Failed to create support ticket attachment: ",
		ticketID, createRequest, supportTicketAttachmentCreateSideEffects,
		func(ctx context.Context, client *linode.Client) (*mcp.CallToolResult, string) {
			// The attachment endpoint returns no useful resource body, so the
			// response echoes the affected ticket id.
			if _, attachFailure := client.CreateSupportTicketAttachment(ctx, ticketID, createRequest); attachFailure != nil {
				return nil, attachFailure.Error()
			}

			result, marshalErr := MarshalProtoToolResponse(&linodev1.SupportTicketIDResponse{
				Message:  "Support ticket attachment created successfully",
				TicketId: linodeIDToInt32(ticketID),
			})
			if marshalErr != nil {
				return nil, marshalErr.Error()
			}

			return result, ""
		})
}

func supportTicketAttachmentCreateRequestFromTool(request *mcp.CallToolRequest) (int, *linode.CreateSupportTicketAttachmentRequest, string) {
	args := request.GetArguments()

	ticketID, validationMessage := requiredSupportTicketAttachmentTicketID(args)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	file, validationMessage := requiredAccountUserString(args, supportTicketAttachmentFileParam)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	return ticketID, &linode.CreateSupportTicketAttachmentRequest{File: file}, ""
}

func requiredSupportTicketAttachmentTicketID(args map[string]any) (int, string) {
	if _, ok := args[supportTicketTicketIDParam]; !ok {
		return 0, supportTicketTicketIDParam + " is required"
	}

	value, validationMessage := optionalSupportTicketPositiveID(args, supportTicketTicketIDParam)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return value, ""
}

func handleLinodeManagedContactCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	createRequest, validationMessage := managedContactCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_contact_create", httpMethodPost, managedContactsPath, nil)
	}

	if result := RequireConfirm(request, "This creates a managed contact. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	contact, createFailure := client.CreateManagedContact(ctx, createRequest)
	if createFailure == nil {
		return MarshalToolResponse(contact)
	}

	return mcp.NewToolResultError("Failed to create linode_managed_contact_create: " + createFailure.Error()), nil
}

func managedContactCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateManagedContactRequest, string) {
	args := request.GetArguments()
	if _, exists := args[managedContactIDParam]; exists {
		return nil, errManagedContactReadOnlyField
	}

	if _, exists := args[managedContactUpdatedParam]; exists {
		return nil, errManagedContactReadOnlyField
	}

	var (
		createRequest linode.CreateManagedContactRequest
		fieldSet      bool
	)

	if validationMessage := optionalManagedContactString(args, managedContactNameParam, &createRequest.Name); validationMessage != "" {
		return nil, validationMessage
	}

	if createRequest.Name != nil {
		fieldSet = true
	}

	if validationMessage := optionalManagedContactString(args, managedContactEmailParam, &createRequest.Email); validationMessage != "" {
		return nil, validationMessage
	}

	if createRequest.Email != nil {
		fieldSet = true
	}

	if validationMessage := optionalManagedContactString(args, managedContactGroupParam, &createRequest.Group); validationMessage != "" {
		return nil, validationMessage
	}

	if createRequest.Group != nil {
		fieldSet = true
	}

	var phone linode.CreateManagedContactPhoneRequest

	phoneSet, validationMessage := managedContactPhoneFromArgs(args, &phone.Primary, &phone.Secondary)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if phoneSet {
		createRequest.Phone = &phone
		fieldSet = true
	}

	if !fieldSet {
		return nil, errManagedContactFieldRequired
	}

	return &createRequest, ""
}

// managedContactPhoneFromArgs reads the nested phone object argument and
// writes its primary/secondary numbers into the provided targets. It
// reports whether at least one number was set so callers can flag the
// phone field as present. An absent phone argument is not an error.
func managedContactPhoneFromArgs(args map[string]any, primary, secondary **string) (bool, string) {
	raw, exists := args[managedContactPhoneParam]
	if !exists {
		return false, ""
	}

	phoneObj, isObject := raw.(map[string]any)
	if !isObject {
		return false, managedContactPhoneParam + " must be an object"
	}

	var fields int

	if validationMessage := optionalManagedContactString(phoneObj, managedContactPhonePrimaryKey, primary); validationMessage != "" {
		return false, managedContactPhoneParam + "." + validationMessage
	}

	if *primary != nil {
		fields++
	}

	if validationMessage := optionalManagedContactString(phoneObj, managedContactPhoneSecondaryKey, secondary); validationMessage != "" {
		return false, managedContactPhoneParam + "." + validationMessage
	}

	if *secondary != nil {
		fields++
	}

	return fields > 0, ""
}

func optionalManagedContactString(args map[string]any, name string, target **string) string {
	raw, exists := args[name]
	if !exists {
		return ""
	}

	value, isString := raw.(string)
	if !isString || strings.TrimSpace(value) == "" {
		return name + " must be a non-empty string"
	}

	*target = &value

	return ""
}

func requiredAccountUserString(args map[string]any, name string) (string, string) {
	raw, found := args[name]
	if !found {
		return "", name + " is required"
	}

	value, isString := raw.(string)
	if !isString || strings.TrimSpace(value) == "" {
		return "", name + " must be a non-empty string"
	}

	return value, ""
}

func accountLoginsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountLoginsPageSizeMin, accountLoginsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeAccountLoginGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	loginID, validationMessage := accountLoginIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	login, getFailure := client.GetAccountLoginProto(ctx, loginID)
	if getFailure == nil {
		return MarshalProtoToolResponse(login)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_login_get: " + getFailure.Error()), nil
}

func accountLoginIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["login_id"]
	if !exists {
		return 0, "login_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 {
			return 0, errAccountLoginIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxAccountLoginIDFromJSON || value != float64(int64(value)) {
			return 0, errAccountLoginIDPositive
		}

		return int(value), ""
	default:
		return 0, errAccountLoginIDPositive
	}
}

func accountInvoicesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountInvoicesPageSizeMin, accountInvoicesPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func accountPaymentsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountPaymentsPageSizeMin, accountPaymentsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeAccountPaymentGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	paymentID, validationMessage := accountPaymentIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	payment, getFailure := client.GetAccountPaymentProto(ctx, paymentID)
	if getFailure == nil {
		return MarshalProtoToolResponse(payment)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_payment_get: " + getFailure.Error()), nil
}

func accountPaymentIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["payment_id"]
	if !exists {
		return 0, "payment_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 {
			return 0, errAccountPaymentIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxAccountPaymentIDFromJSON || value != float64(int64(value)) {
			return 0, errAccountPaymentIDPositive
		}

		return int(value), ""
	default:
		return 0, errAccountPaymentIDPositive
	}
}

func handleLinodeAccountPaymentCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := accountPaymentCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_payment_create", httpMethodPost, accountPaymentsPath, nil)
	}

	if result := RequireConfirm(request, "This makes an account payment. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	payment, createFailure := client.CreateAccountPaymentProto(ctx, req)
	if createFailure == nil {
		return MarshalProtoToolResponse(&linodev1.AccountPaymentWriteResponse{
			Message: "Account payment created successfully",
			Payment: payment,
		})
	}

	return mcp.NewToolResultError("Failed to create linode_account_payment_create: " + createFailure.Error()), nil
}

func handleLinodeAccountPromoCreditRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := accountPromoCreditRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_promo_credit_add", httpMethodPost, accountPromoCodesPath, nil)
	}

	if result := RequireConfirm(request, "This applies a promo credit to the account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if addFailureMessage := addAccountPromoCredit(ctx, client, req); addFailureMessage != "" {
		return mcp.NewToolResultError("Failed to apply linode_account_promo_credit_add: " + addFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountPromoResponse{
		Message:   "Account promo credit applied successfully",
		PromoCode: req.PromoCode,
	})
}

func addAccountPromoCredit(ctx context.Context, client *linode.Client, req *linode.AddAccountPromoCreditRequest) string {
	if err := client.AddAccountPromoCredit(ctx, req); err != nil {
		return err.Error()
	}

	return ""
}

func accountPromoCreditRequestFromTool(request *mcp.CallToolRequest) (*linode.AddAccountPromoCreditRequest, string) {
	raw, exists := request.GetArguments()["promo_code"]
	if !exists {
		return nil, "promo_code is required"
	}

	promoCode, ok := raw.(string)
	if !ok || strings.TrimSpace(promoCode) == "" {
		return nil, "promo_code must be a non-empty string"
	}

	if promoCode != strings.TrimSpace(promoCode) {
		return nil, "promo_code must not include leading or trailing whitespace"
	}

	return &linode.AddAccountPromoCreditRequest{PromoCode: promoCode}, ""
}

func accountPaymentCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateAccountPaymentRequest, string) {
	args := request.GetArguments()
	req := &linode.CreateAccountPaymentRequest{}

	if raw, exists := args["payment_method_id"]; exists {
		id, ok := numberArgToInt(raw)
		if !ok || id <= 0 {
			return nil, "payment_method_id must be a positive integer"
		}

		req.PaymentMethodID = id
	}

	rawUSD, exists := args["usd"]
	if !exists {
		return nil, "usd is required"
	}

	usd, ok := rawUSD.(string)
	if !ok || strings.TrimSpace(usd) == "" {
		return nil, "usd must be a non-empty string"
	}

	amount, err := strconv.ParseFloat(strings.TrimSpace(usd), 64)
	if err != nil || amount <= 0 {
		return nil, "usd must be a positive number"
	}

	req.USD = strings.TrimSpace(usd)

	return req, ""
}

func handleLinodeAccountInvoiceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	invoiceID, validationMessage := accountInvoiceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	invoice, getFailure := client.GetAccountInvoiceProto(ctx, invoiceID)
	if getFailure == nil {
		return MarshalProtoToolResponse(invoice)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_invoice_get: " + getFailure.Error()), nil
}

func accountInvoiceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["invoice_id"]
	if !exists {
		return 0, "invoice_id is required"
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 {
			return 0, errAccountInvoiceIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value != float64(int(value)) {
			return 0, errAccountInvoiceIDPositive
		}

		return int(value), ""
	default:
		return 0, errAccountInvoiceIDPositive
	}
}

func accountInvoiceItemsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountInvoiceItemsPageSizeMin, accountInvoiceItemsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func accountChildAccountsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountChildAccountsPageSizeMin, accountChildAccountsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeAccountChildAccountGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	euuid, validationMessage := accountChildAccountEUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	childAccount, getFailure := client.GetAccountChildAccount(ctx, euuid)
	if getFailure == nil {
		return MarshalToolResponse(childAccount)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_child_account_get: " + getFailure.Error()), nil
}

func handleLinodeAccountChildAccountTokenRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	euuid, validationMessage := accountChildAccountEUUIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		// Credential-safe: fetch the child account metadata (not the proxy
		// token) and preview the POST; the token is never surfaced.
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_account_child_account_token_create", httpMethodPost,
			accountChildAccountsPath+"/"+euuid+"/token",
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountChildAccount(ctx, euuid)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return accountChildAccountTokenCreateSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This creates a proxy user token for a child account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	token, createFailure := client.CreateAccountChildAccountToken(ctx, euuid)
	if createFailure == nil {
		return MarshalToolResponse(token)
	}

	return mcp.NewToolResultError("Failed to create linode_account_child_account_token_create: " + createFailure.Error()), nil
}

func handleLinodeAccountServiceTransferGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	token, validationMessage := accountTransferTokenFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	transfer, getFailure := client.GetAccountServiceTransferProto(ctx, token)
	if getFailure == nil {
		return MarshalProtoToolResponse(transfer)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_service_transfer_get: " + getFailure.Error()), nil
}

// runAccountTransferAction wires dry-run preview, confirm gating, and
// execution for the token-based entity/service transfer accept + delete
// actions, which are otherwise identical and trip the dupl linter. verb is
// the path suffix ("accept") or empty (delete). The caller's execute closure
// returns a full failure message (already prefixed) or "" on success.
func runAccountTransferAction(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, basePath, verb, confirmMessage, successMessage, failurePrefix string,
	fetchState func(context.Context, *linode.Client, string) (any, error),
	execute func(context.Context, *linode.Client, string) string,
) (*mcp.CallToolResult, error) {
	token, validationMessage := accountTransferTokenFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		path := basePath + "/" + token
		if verb != "" {
			path += "/" + verb
		}

		var fetch func(context.Context, *linode.Client) (any, error)
		if fetchState != nil {
			fetch = func(ctx context.Context, c *linode.Client) (any, error) {
				return fetchState(ctx, c, token)
			}
		}

		return RunDryRunPreview(ctx, request, cfg, toolName, method, path, fetch)
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	if failureMessage := execute(ctx, client, token); failureMessage != "" {
		return mcp.NewToolResultError(failurePrefix + failureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountServiceTransferActionResponse{
		Message: successMessage,
		Token:   token,
	})
}

func handleLinodeAccountServiceTransferDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runAccountTransferAction(ctx, request, cfg,
		"linode_account_service_transfer_delete", httpMethodDelete, accountServiceTransfersPath, "",
		"This cancels an account service transfer. Set confirm=true to proceed.",
		"Account service transfer canceled successfully",
		"Failed to delete linode_account_service_transfer_delete: ",
		func(ctx context.Context, c *linode.Client, token string) (any, error) {
			return c.GetAccountServiceTransfer(ctx, token)
		},
		deleteAccountServiceTransfer)
}

func handleLinodeAccountServiceTransferAcceptRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runAccountTransferAction(ctx, request, cfg,
		"linode_account_service_transfer_accept", httpMethodPost, accountServiceTransfersPath, "accept",
		"This accepts an account service transfer. Set confirm=true to proceed.",
		"Account service transfer accepted successfully",
		"Failed to accept linode_account_service_transfer_accept: ",
		func(ctx context.Context, c *linode.Client, token string) (any, error) {
			return c.GetAccountServiceTransfer(ctx, token)
		},
		acceptAccountServiceTransfer)
}

func handleLinodeAccountEventGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	eventID, validationMessage := accountEventIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	event, getFailure := client.GetAccountEventProto(ctx, eventID)
	if getFailure == nil {
		return MarshalProtoToolResponse(event)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_event_get: " + getFailure.Error()), nil
}

func handleLinodeAccountEventSeenRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	eventID, validationMessage := accountEventIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_account_event_seen", httpMethodPost,
			fmt.Sprintf(accountEventsPath+"/%d/seen", eventID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountEvent(ctx, eventID)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return accountEventSeenSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This marks an account event as seen. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	seenFailureMessage := markAccountEventSeen(ctx, client, eventID)
	if seenFailureMessage != "" {
		return mcp.NewToolResultError("Failed to mark linode_account_event_seen: " + seenFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.AccountEventSeenResponse{
		Message: "Account event marked as seen successfully",
		EventId: linodeIDToInt32(eventID),
	})
}

func markAccountEventSeen(ctx context.Context, client *linode.Client, eventID int) string {
	seenFailure := client.MarkAccountEventSeen(ctx, eventID)
	if seenFailure != nil {
		return seenFailure.Error()
	}

	return ""
}

func accountEventIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[accountEventIDParam]
	if !exists {
		return 0, accountEventIDParam + " is required"
	}

	switch value := raw.(type) {
	case float64:
		if value <= 0 || value != float64(int(value)) {
			return 0, accountEventIDParam + " must be a positive integer"
		}

		return int(value), ""
	case int:
		if value <= 0 {
			return 0, accountEventIDParam + " must be a positive integer"
		}

		return value, ""
	default:
		return 0, accountEventIDParam + " must be a positive integer"
	}
}

func accountTransferTokenFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, exists := request.GetArguments()["token"]
	if !exists {
		return "", "token is required"
	}

	token, ok := raw.(string)
	if !ok || strings.TrimSpace(token) == "" {
		return "", "token must be a non-empty string"
	}

	if token != strings.TrimSpace(token) || strings.Contains(token, "/") || strings.Contains(token, "?") || strings.Contains(token, "..") {
		return "", "token must not contain path separators, query separators, or traversal segments"
	}

	return token, ""
}

func accountServiceTransfersPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountServiceTransfersPageSizeMin, accountServiceTransfersPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeAccountServiceTransferCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := accountServiceTransferCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_service_transfer_create", httpMethodPost, accountServiceTransfersPath, nil)
	}

	if result := RequireConfirm(request, "This creates an account service transfer. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	transfer, createFailure := client.CreateAccountServiceTransfer(ctx, req)
	if createFailure == nil {
		return MarshalToolResponse(transfer)
	}

	return mcp.NewToolResultError("Failed to create linode_account_service_transfer_create: " + createFailure.Error()), nil
}

func deleteAccountServiceTransfer(ctx context.Context, client *linode.Client, token string) string {
	deleteFailure := client.DeleteAccountServiceTransfer(ctx, token)
	if deleteFailure != nil {
		return deleteFailure.Error()
	}

	return ""
}

func acceptAccountServiceTransfer(ctx context.Context, client *linode.Client, token string) string {
	acceptFailure := client.AcceptAccountServiceTransfer(ctx, token)
	if acceptFailure != nil {
		return acceptFailure.Error()
	}

	return ""
}

func accountServiceTransferCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateAccountServiceTransferRequest, string) {
	raw, exists := request.GetArguments()["linode_ids"]
	if !exists {
		return nil, "linode_ids is required"
	}

	ids, validationMessage := intSliceFromToolArg(raw, "linode_ids")
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.CreateAccountServiceTransferRequest{
		Entities: linode.AccountEntityTransferEntities{Linodes: ids},
	}, ""
}

func intSliceFromToolArg(raw any, name string) ([]int, string) {
	switch values := raw.(type) {
	case []int:
		if len(values) == 0 {
			return nil, name + " must include at least one ID"
		}

		ids := make([]int, 0, len(values))
		for _, value := range values {
			if value <= 0 {
				return nil, name + " must be an array of positive integers"
			}

			ids = append(ids, value)
		}

		return ids, ""
	case []any:
		return intSliceFromAnySlice(values, name)
	default:
		return nil, name + " must be an array of positive integers"
	}
}

func intSliceFromAnySlice(values []any, name string) ([]int, string) {
	if len(values) == 0 {
		return nil, name + " must include at least one ID"
	}

	ids := make([]int, 0, len(values))

	for _, value := range values {
		switch number := value.(type) {
		case float64:
			// math.MaxInt64 has no exact float64 representation: float64(math.MaxInt64)
			// rounds up to 2^63 (math.MaxInt64+1). Use >= against that float so any
			// value at or above the representable boundary is rejected. math.Trunc
			// rejects fractional values without going through an int conversion that
			// overflows for out-of-range floats.
			if number <= 0 || number >= float64(math.MaxInt64) || math.Trunc(number) != number {
				return nil, name + " must be an array of positive integers"
			}

			ids = append(ids, int(number))
		case int:
			if number <= 0 {
				return nil, name + " must be an array of positive integers"
			}

			ids = append(ids, number)
		default:
			return nil, name + " must be an array of positive integers"
		}
	}

	return ids, ""
}

func accountChildAccountEUUIDFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, exists := request.GetArguments()["euuid"]
	if !exists {
		return "", "euuid is required"
	}

	euuid, ok := raw.(string)
	if !ok || strings.TrimSpace(euuid) == "" {
		return "", "euuid must be a non-empty string"
	}

	if euuid != strings.TrimSpace(euuid) || strings.Contains(euuid, "/") || strings.Contains(euuid, "?") || strings.Contains(euuid, "..") {
		return "", "euuid must not contain path separators, query separators, or traversal segments"
	}

	return euuid, ""
}

func handleLinodeBetaGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	betaID, validationMessage := accountBetaIDFromTool(request, "beta_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	beta, getFailure := client.GetBetaProto(ctx, betaID)
	if getFailure == nil {
		return MarshalProtoToolResponse(beta)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_beta_get: " + getFailure.Error()), nil
}

func handleLinodeAccountBetaGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	betaID, validationMessage := accountBetaIDFromTool(request, "beta_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	beta, getFailure := client.GetAccountBetaProto(ctx, betaID)
	if getFailure == nil {
		return MarshalProtoToolResponse(beta)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_beta_get: " + getFailure.Error()), nil
}

func handleLinodeAccountBetaEnrollRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := enrollAccountBetaRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_account_beta_enroll", httpMethodPost, accountBetasPath, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return accountBetaEnrollSideEffects(ctx, req.ID)
			})
	}

	if result := RequireConfirm(request, "This enrolls the account in a beta program. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	enrollErr := client.EnrollAccountBeta(ctx, req)
	if enrollErr == nil {
		response := struct {
			Message string `json:"message"`
			ID      string `json:"id"`
		}{
			Message: "Account beta enrollment requested successfully",
			ID:      req.ID,
		}

		return MarshalToolResponse(response)
	}

	return mcp.NewToolResultError("Failed to enroll linode_account_beta_enroll: " + enrollErr.Error()), nil
}

func enrollAccountBetaRequestFromTool(request *mcp.CallToolRequest) (*linode.EnrollAccountBetaRequest, string) {
	id, validationMessage := accountBetaIDFromTool(request, "id")
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.EnrollAccountBetaRequest{ID: id}, ""
}

func accountBetaIDFromTool(request *mcp.CallToolRequest, field string) (string, string) {
	raw, exists := request.GetArguments()[field]
	if !exists {
		return "", field + " is required"
	}

	id, ok := raw.(string)
	if !ok || strings.TrimSpace(id) == "" {
		return "", field + " must be a non-empty string"
	}

	if id != strings.TrimSpace(id) || !isAccountBetaID(id) {
		return "", field + " must contain only letters, numbers, underscores, and hyphens"
	}

	return id, ""
}

func isAccountBetaID(id string) bool {
	for _, char := range id {
		if char >= 'a' && char <= 'z' {
			continue
		}

		if char >= 'A' && char <= 'Z' {
			continue
		}

		if char >= '0' && char <= '9' {
			continue
		}

		if char == '_' || char == '-' {
			continue
		}

		return false
	}

	return true
}

func handleLinodeManagedCredentialUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	credentialID, ok := getPositiveIntArgument(request, managedUpdateIDParam)
	if !ok {
		return mcp.NewToolResultError(errManagedIDPositive), nil
	}

	label, validationMessage := stringArgument(request, "label", false)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if _, exists := request.GetArguments()["label"]; !exists {
		return mcp.NewToolResultError("label is required"), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_credential_update", "PUT",
			fmt.Sprintf(managedCredentialsPath+"/%d", credentialID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedCredential(ctx, credentialID)
			})
	}

	if result := RequireConfirm(request, "This updates a Managed credential. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	credential, failureMessage := updateManagedCredentialResponse(ctx, client, credentialID, label)
	if failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedCredentialWriteResponse{
		Message:    fmt.Sprintf("Managed credential %d updated successfully", credentialID),
		Credential: credential,
	})
}

func updateManagedCredentialResponse(ctx context.Context, client *linode.Client, credentialID int, label string) (*linodev1.ManagedCredential, string) {
	credential, err := client.UpdateManagedCredentialProto(ctx, credentialID, linode.UpdateManagedCredentialRequest{Label: &label})
	if err != nil {
		return nil, "Failed to update linode_managed_credential_update " + strconv.Itoa(credentialID) + ": " + err.Error()
	}

	return credential, ""
}

func handleLinodeManagedCredentialUsernamePasswordUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	credentialID, updateReq, validationMessage := managedCredentialUsernamePasswordUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		// Credential-safe: fetch the credential metadata (no secret) and
		// preview the POST; the new username/password are never echoed.
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_credential_username_password_update", httpMethodPost,
			fmt.Sprintf(managedCredentialsPath+"/%d/update", credentialID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedCredential(ctx, credentialID)
			})
	}

	if result := RequireConfirm(request, "This updates a stored Managed credential's username and password. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if failureMessage := updateManagedCredentialUsernamePasswordResponse(ctx, client, credentialID, updateReq); failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.ManagedCredentialIDResponse{
		Message:      fmt.Sprintf("Managed credential %d updated successfully", credentialID),
		CredentialId: linodeIDToInt32(credentialID),
	})
}

func updateManagedCredentialUsernamePasswordResponse(ctx context.Context, client *linode.Client, credentialID int, req *linode.UpdateManagedCredentialUsernamePasswordRequest) string {
	// The username/password update endpoint returns the credential metadata, but
	// the secret is never echoed and the metadata adds nothing over an id-echo,
	// so the body is discarded and only success/failure matters here.
	if _, err := client.UpdateManagedCredentialUsernamePassword(ctx, credentialID, req); err != nil {
		return "Failed to update linode_managed_credential_username_password_update " + strconv.Itoa(credentialID) + ": " + err.Error()
	}

	return ""
}

func managedCredentialUsernamePasswordUpdateRequestFromTool(request *mcp.CallToolRequest) (int, *linode.UpdateManagedCredentialUsernamePasswordRequest, string) {
	credentialID, validationMessage := managedCredentialIDFromTool(request)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	password, validationMessage := stringArgument(request, managedCredentialCreatePassParam, true)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	if strings.TrimSpace(password) == "" {
		return 0, nil, errManagedCredentialPasswordReq
	}

	username, validationMessage := stringArgument(request, managedCredentialCreateUserParam, false)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	req := &linode.UpdateManagedCredentialUsernamePasswordRequest{Password: password}

	if _, exists := request.GetArguments()[managedCredentialCreateUserParam]; exists {
		if strings.TrimSpace(username) == "" {
			return 0, nil, managedCredentialCreateUserParam + " must be a non-empty string"
		}

		req.Username = &username
	}

	return credentialID, req, ""
}

func handleLinodeManagedSSHKeyRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sshKey, getFailure := client.GetManagedSSHKey(ctx)
	if getFailure == nil {
		return MarshalToolResponse(sshKey)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_sshkey_get: " + getFailure.Error()), nil
}

func handleLinodeManagedCredentialGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	credentialID, validationMessage := managedCredentialIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_credential_get", "GET",
			fmt.Sprintf(managedCredentialsPath+"/%d", credentialID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedCredential(ctx, credentialID)
			})
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	credential, getFailure := client.GetManagedCredentialProto(ctx, credentialID)
	if getFailure == nil {
		return MarshalProtoToolResponse(credential)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_managed_credential_get: " + getFailure.Error()), nil
}

func handleLinodeManagedCredentialRevokeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	credentialID, validationMessage := managedCredentialIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_credential_revoke", httpMethodPost,
			fmt.Sprintf(managedCredentialsPath+"/%d/revoke", credentialID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetManagedCredential(ctx, credentialID)
			})
	}

	if result := RequireConfirm(request, "This revokes a stored Managed credential. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	revokeFailure := client.RevokeManagedCredential(ctx, credentialID)
	if revokeFailure == nil {
		return MarshalProtoToolResponse(&linodev1.ManagedCredentialIDResponse{
			Message:      fmt.Sprintf("Managed credential %d revoked successfully", credentialID),
			CredentialId: linodeIDToInt32(credentialID),
		})
	}

	return mcp.NewToolResultError("Failed to revoke linode_managed_credential_revoke: " + revokeFailure.Error()), nil
}

func managedCredentialIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[managedIDParam]
	if !exists {
		return 0, errManagedIDPositive
	}

	switch value := raw.(type) {
	case int:
		if value <= 0 || value > maxManagedIDFromJSON {
			return 0, errManagedIDPositive
		}

		return value, ""
	case float64:
		if value <= 0 || value > maxManagedIDFromJSON || value != float64(int64(value)) {
			return 0, errManagedIDPositive
		}

		return int(value), ""
	default:
		return 0, errManagedIDPositive
	}
}

func managedCredentialsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", managedCredentialsPageSizeMin, managedCredentialsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeManagedCredentialCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	createReq, validationMessage := managedCredentialCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		// Credential-safe: the create body holds the password, but the v0
		// preview is method + path only, so the secret is never echoed.
		return RunDryRunPreview(ctx, request, cfg, "linode_managed_credential_create", httpMethodPost, managedCredentialsPath, nil)
	}

	if result := RequireConfirm(request, "This creates a stored Managed credential. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	credential, createFailure := client.CreateManagedCredential(ctx, createReq)
	if createFailure == nil {
		return MarshalToolResponse(credential)
	}

	return mcp.NewToolResultError("Failed to create linode_managed_credential_create: " + createFailure.Error()), nil
}

func managedCredentialCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateManagedCredentialRequest, string) {
	label, validationMessage := stringArgument(request, managedCredentialCreateLabelParam, true)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if strings.TrimSpace(label) == "" {
		return nil, managedCredentialCreateLabelParam + " is required"
	}

	password, validationMessage := stringArgument(request, managedCredentialCreatePassParam, true)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if strings.TrimSpace(password) == "" {
		return nil, managedCredentialCreatePassParam + " is required"
	}

	username, validationMessage := stringArgument(request, managedCredentialCreateUserParam, false)
	if validationMessage != "" {
		return nil, validationMessage
	}

	var usernameSet bool

	if _, exists := request.GetArguments()[managedCredentialCreateUserParam]; exists {
		if strings.TrimSpace(username) == "" {
			return nil, managedCredentialCreateUserParam + " must be a non-empty string"
		}

		usernameSet = true
	}

	req := &linode.CreateManagedCredentialRequest{
		Label:    label,
		Password: password,
	}
	if usernameSet {
		req.Username = &username
	}

	return req, ""
}

func accountMaintenancePaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountMaintenancePageSizeMin, accountMaintenancePageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func handleLinodeAccountAvailabilityGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	regionID, validationMessage := accountAvailabilityRegionIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	availability, getFailure := client.GetAccountAvailabilityProto(ctx, regionID)
	if getFailure == nil {
		return MarshalProtoToolResponse(availability)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_availability_get: " + getFailure.Error()), nil
}

func accountAvailabilityRegionIDFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, exists := request.GetArguments()["region_id"]
	if !exists {
		return "", "region_id is required"
	}

	regionID, ok := raw.(string)
	if !ok || strings.TrimSpace(regionID) == "" {
		return "", "region_id must be a non-empty string"
	}

	if !isAccountAvailabilityRegionSlug(regionID) {
		return "", "region_id must be a lowercase region slug containing only letters, numbers, and hyphens"
	}

	return regionID, ""
}

func isAccountAvailabilityRegionSlug(regionID string) bool {
	for _, char := range regionID {
		if char >= 'a' && char <= 'z' {
			continue
		}

		if char >= '0' && char <= '9' {
			continue
		}

		if char == '-' {
			continue
		}

		return false
	}

	return true
}

func betasPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", betasPageSizeMin, betasPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func accountAvailabilityPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountAvailabilityPageSizeMin, accountAvailabilityPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func optionalPaginationInt(args map[string]any, name string, minValue, maxValue int) (int, string) {
	raw, exists := args[name]
	if !exists {
		return 0, ""
	}

	var value int

	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		value = int(typed)
		if typed != float64(value) {
			return 0, name + " must be an integer"
		}
	default:
		return 0, name + " must be an integer"
	}

	if value < minValue || (maxValue > 0 && value > maxValue) {
		if maxValue > 0 {
			return 0, name + " must be an integer from " + strconv.Itoa(minValue) + " through " + strconv.Itoa(maxValue)
		}

		return 0, name + " must be an integer greater than or equal to 1"
	}

	return value, ""
}

func handleLinodeAccountAgreementsAcknowledgeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := acknowledgeAccountAgreementsRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_account_agreement_acknowledge", httpMethodPost, accountAgreementsPath, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return accountAgreementsAcknowledgeSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This acknowledges account agreements. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ackErr := client.AcknowledgeAccountAgreements(ctx, req)
	if ackErr == nil {
		response := struct {
			Message string `json:"message"`
		}{
			Message: "Account agreements acknowledged successfully",
		}

		return MarshalToolResponse(response)
	}

	return mcp.NewToolResultError("Failed to acknowledge account agreements: " + ackErr.Error()), nil
}

func acknowledgeAccountAgreementsRequestFromTool(request *mcp.CallToolRequest) (*linode.AcknowledgeAccountAgreementsRequest, string) {
	args := request.GetArguments()
	req := linode.AcknowledgeAccountAgreementsRequest{}

	var setCount int

	setBool := func(name string, target **bool) string {
		raw, exists := args[name]
		if !exists {
			return ""
		}

		value, ok := raw.(bool)
		if !ok {
			return name + " must be a boolean"
		}

		if !value {
			return name + " must be true when provided"
		}

		*target = &value
		setCount++

		return ""
	}

	for _, field := range []struct {
		name   string
		target **bool
	}{
		{name: "billing_agreement", target: &req.BillingAgreement},
		{name: "eu_model", target: &req.EUModel},
		{name: "master_service_agreement", target: &req.MasterServiceAgreement},
		{name: "privacy_policy", target: &req.PrivacyPolicy},
	} {
		if message := setBool(field.name, field.target); message != "" {
			return nil, message
		}
	}

	if setCount == 0 {
		return nil, "at least one account agreement field is required"
	}

	return &req, ""
}

func handleLinodeAccountCancelRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := cancelAccountRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_account_cancel", httpMethodPost, accountCancelPath, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return accountCancelSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This cancels the active account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cancelResponse, cancelErr := client.CancelAccount(ctx, req)
	if cancelErr == nil {
		response := struct {
			Message    string                        `json:"message"`
			CancelInfo *linode.CancelAccountResponse `json:"cancel_info"`
		}{
			Message:    "Account canceled successfully",
			CancelInfo: cancelResponse,
		}

		return MarshalToolResponse(response)
	}

	return mcp.NewToolResultError("Failed to cancel account: " + cancelErr.Error()), nil
}

func cancelAccountRequestFromTool(request *mcp.CallToolRequest) (*linode.CancelAccountRequest, string) {
	args := request.GetArguments()
	req := linode.CancelAccountRequest{}

	raw, exists := args["comments"]
	if !exists {
		return &req, ""
	}

	comments, ok := raw.(string)
	if !ok {
		return nil, "comments must be a string"
	}

	req.Comments = &comments

	return &req, ""
}

func handleLinodeAccountUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return handleLinodeAccountUpdateDryRun(ctx, request, cfg)
	}

	if result := RequireConfirm(request, "This updates account billing/contact information. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req, validationMessage := updateAccountRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	updatedAccount, updateErr := client.UpdateAccountProto(ctx, req)
	if updateErr == nil {
		response := &linodev1.AccountWriteResponse{
			Message: "Account updated successfully",
			Account: updatedAccount,
		}

		return MarshalProtoToolResponse(response)
	}

	return mcp.NewToolResultError("Failed to update account: " + updateErr.Error()), nil
}

// handleLinodeAccountUpdateDryRun fetches the current account state
// and returns the dry-run preview without making the PUT call. The
// response includes the account's PII fields (tax_id, phone, address,
// etc.) which are returned to the model directly; audit-log Phase 4c's
// PII redaction tier scrubs the args sent to the call but does not
// touch tool responses, so the model can compare current vs proposed
// values.
func handleLinodeAccountUpdateDryRun(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	account, err := client.GetAccount(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch account for dry-run: %v", err)), nil
	}

	return BuildDryRunResponse(
		"linode_account_update",
		request.GetString(paramEnvironment, ""),
		"PUT",
		"/account",
		account,
	)
}

func updateAccountRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateAccountRequest, string) {
	args := request.GetArguments()
	req := linode.UpdateAccountRequest{}

	var setCount int

	setString := func(name string, target **string) string {
		raw, exists := args[name]
		if !exists {
			return ""
		}

		value, ok := raw.(string)
		if !ok {
			return name + " must be a string"
		}

		*target = &value
		setCount++

		return ""
	}

	for _, field := range []struct {
		name   string
		target **string
	}{
		{name: "address_1", target: &req.Address1},
		{name: "address_2", target: &req.Address2},
		{name: "city", target: &req.City},
		{name: "company", target: &req.Company},
		{name: "country", target: &req.Country},
		{name: "email", target: &req.Email},
		{name: "first_name", target: &req.FirstName},
		{name: "last_name", target: &req.LastName},
		{name: "phone", target: &req.Phone},
		{name: "state", target: &req.State},
		{name: "tax_id", target: &req.TaxID},
		{name: "zip", target: &req.Zip},
	} {
		if message := setString(field.name, field.target); message != "" {
			return nil, message
		}
	}

	if setCount == 0 {
		return nil, "at least one account field is required"
	}

	return &req, ""
}

func enableAccountManagedErrorMessage(ctx context.Context, client *linode.Client) string {
	if err := client.EnableAccountManaged(ctx); err != nil {
		return "Failed to enable Linode Managed: " + err.Error()
	}

	return ""
}

func handleLinodeAccountSettingsManagedEnableRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_settings_managed_enable", httpMethodPost,
			accountSettingsPath+"/managed-enable",
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountSettings(ctx)
			})
	}

	if result := RequireConfirm(request, "This enables Linode Managed for the account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if errorMessage := enableAccountManagedErrorMessage(ctx, client); errorMessage != "" {
		return mcp.NewToolResultError(errorMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.MessageResponse{
		Message: "Linode Managed enabled successfully",
	})
}

func handleLinodeAccountSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := updateAccountSettingsRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_account_settings_update", "PUT",
			accountSettingsPath,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetAccountSettings(ctx)
			})
	}

	if result := RequireConfirm(request, "This updates account-wide settings. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updatedSettings, updateErr := client.UpdateAccountSettings(ctx, req)
	if updateErr == nil {
		response := struct {
			Message  string                  `json:"message"`
			Settings *linode.AccountSettings `json:"settings"`
		}{
			Message:  "Account settings updated successfully",
			Settings: updatedSettings,
		}

		return MarshalToolResponse(response)
	}

	return mcp.NewToolResultError("Failed to update account settings: " + updateErr.Error()), nil
}

func updateAccountSettingsRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateAccountSettingsRequest, string) {
	args := request.GetArguments()
	req := linode.UpdateAccountSettingsRequest{}

	var setCount int

	setBool := func(name string, target **bool) string {
		raw, exists := args[name]
		if !exists {
			return ""
		}

		value, ok := raw.(bool)
		if !ok {
			return name + " must be a boolean"
		}

		*target = &value
		setCount++

		return ""
	}

	setString := func(name string, target **string) string {
		raw, exists := args[name]
		if !exists {
			return ""
		}

		value, ok := raw.(string)
		if !ok {
			return name + " must be a string"
		}

		*target = &value
		setCount++

		return ""
	}

	for _, field := range []struct {
		name   string
		target **bool
	}{
		{name: "backups_enabled", target: &req.BackupsEnabled},
		{name: "managed", target: &req.Managed},
		{name: "network_helper", target: &req.NetworkHelper},
	} {
		if message := setBool(field.name, field.target); message != "" {
			return nil, message
		}
	}

	for _, field := range []struct {
		name   string
		target **string
	}{
		{name: "interfaces_for_new_linodes", target: &req.InterfacesForNewLinodes},
		{name: "longview_subscription", target: &req.LongviewSubscription},
		{name: "maintenance_policy", target: &req.MaintenancePolicy},
		{name: "object_storage", target: &req.ObjectStorage},
	} {
		if message := setString(field.name, field.target); message != "" {
			return nil, message
		}
	}

	if setCount == 0 {
		return nil, "at least one account settings field is required"
	}

	return &req, ""
}

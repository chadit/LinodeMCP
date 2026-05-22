package tools

import (
	"context"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	accountAvailabilityPageSizeMin  = 25
	accountAvailabilityPageSizeMax  = 500
	accountBetasPageSizeMin         = 25
	accountBetasPageSizeMax         = 500
	accountChildAccountsPageSizeMin = 25
	accountChildAccountsPageSizeMax = 500
)

// NewLinodeAccountTool creates a tool for retrieving Linode account information.
func NewLinodeAccountTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg, "linode_account",
		"Retrieves the authenticated user's Linode account information including billing details and capabilities",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetAccount(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountAgreementsTool creates a tool for listing account agreement acknowledgment status.
func NewLinodeAccountAgreementsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg, "linode_account_agreements",
		"Lists account agreements and whether each has been acknowledged",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetAccountAgreements(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountBetasTool creates a tool for listing enrolled account beta programs.
func NewLinodeAccountBetasTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_betas",
		"Lists beta programs that the account is enrolled in.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeAccountBetasRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountChildAccountsTool creates a tool for listing child-level accounts.
func NewLinodeAccountChildAccountsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_child_accounts",
		"Lists child-level accounts the authenticated account can access.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeAccountChildAccountsRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountBetaGetTool creates a tool for retrieving one enrolled account beta program.
func NewLinodeAccountBetaGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_beta_get",
		"Gets one beta program that the account is enrolled in.",
		[]mcp.ToolOption{
			mcp.WithString("id", mcp.Required(),
				mcp.Description("Unique identifier for the beta program.")),
		},
		handleLinodeAccountBetaGetRequest,
	)

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
				mcp.Description("Must be true to confirm beta program enrollment.")),
		},
		handleLinodeAccountBetaEnrollRequest,
	)

	return tool, profiles.CapAdmin, handler
}

// NewLinodeAccountAvailabilityTool creates a tool for listing account service availability by region.
func NewLinodeAccountAvailabilityTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_availability",
		"Lists services available and unavailable to the account in each region.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeAccountAvailabilityRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountAvailabilityGetTool creates a tool for retrieving account service availability for one region.
func NewLinodeAccountAvailabilityGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_availability_get",
		"Gets services available and unavailable to the account in one region.",
		[]mcp.ToolOption{
			mcp.WithString("region_id", mcp.Required(),
				mcp.Description("Region slug to inspect, for example us-east.")),
		},
		handleLinodeAccountAvailabilityGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeAccountAgreementsAcknowledgeTool creates a tool for acknowledging account agreements.
func NewLinodeAccountAgreementsAcknowledgeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_account_agreements_acknowledge",
		"Acknowledges one or more account agreements.",
		[]mcp.ToolOption{
			mcp.WithBoolean("billing_agreement", mcp.Description("Acknowledge the billing agreement (optional)")),
			mcp.WithBoolean("eu_model", mcp.Description("Acknowledge the EU model agreement (optional)")),
			mcp.WithBoolean("master_service_agreement", mcp.Description("Acknowledge the master service agreement (optional)")),
			mcp.WithBoolean("privacy_policy", mcp.Description("Acknowledge the privacy policy (optional)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm account agreement acknowledgement.")),
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
				mcp.Description("Must be true to confirm account cancellation.")),
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
		"Updates account billing and contact information.",
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
				mcp.Description("Must be true to confirm account update.")),
		},
		handleLinodeAccountUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeAccountBetasRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := accountBetasPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	betas, listFailure := client.ListAccountBetas(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(betas)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_betas: " + listFailure.Error()), nil
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

func handleLinodeAccountChildAccountsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := accountChildAccountsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	childAccounts, listFailure := client.ListAccountChildAccounts(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(childAccounts)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_child_accounts: " + listFailure.Error()), nil
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

func handleLinodeAccountBetaGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	betaID, validationMessage := accountBetaIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	beta, getFailure := client.GetAccountBeta(ctx, betaID)
	if getFailure == nil {
		return MarshalToolResponse(beta)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_beta_get: " + getFailure.Error()), nil
}

func handleLinodeAccountBetaEnrollRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This enrolls the account in a beta program. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := enrollAccountBetaRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
	id, validationMessage := accountBetaIDFromTool(request)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.EnrollAccountBetaRequest{ID: id}, ""
}

func accountBetaIDFromTool(request *mcp.CallToolRequest) (string, string) {
	raw, exists := request.GetArguments()["id"]
	if !exists {
		return "", "id is required"
	}

	id, ok := raw.(string)
	if !ok || strings.TrimSpace(id) == "" {
		return "", "id must be a non-empty string"
	}

	if id != strings.TrimSpace(id) || !isAccountBetaID(id) {
		return "", "id must contain only letters, numbers, underscores, and hyphens"
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

func handleLinodeAccountAvailabilityGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	regionID, validationMessage := accountAvailabilityRegionIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	availability, getFailure := client.GetAccountAvailability(ctx, regionID)
	if getFailure == nil {
		return MarshalToolResponse(availability)
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

func handleLinodeAccountAvailabilityRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := accountAvailabilityPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	availability, listFailure := client.ListAccountAvailability(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(availability)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_account_availability: " + listFailure.Error()), nil
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
	if result := RequireConfirm(request, "This acknowledges account agreements. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req, validationMessage := acknowledgeAccountAgreementsRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
	if result := RequireConfirm(request, "This cancels the active account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req, validationMessage := cancelAccountRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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

	updatedAccount, updateErr := client.UpdateAccount(ctx, req)
	if updateErr == nil {
		response := struct {
			Message string          `json:"message"`
			Account *linode.Account `json:"account"`
		}{
			Message: "Account updated successfully",
			Account: updatedAccount,
		}

		return MarshalToolResponse(response)
	}

	return mcp.NewToolResultError("Failed to update account: " + updateErr.Error()), nil
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

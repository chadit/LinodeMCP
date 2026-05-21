package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
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

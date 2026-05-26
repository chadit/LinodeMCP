package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	managedContactsPageSizeMin     = 25
	managedContactsPageSizeMax     = 500
	managedContactGetIDParam       = "contact_id"
	errManagedContactGetIDPositive = "contact_id must be a positive integer"
	maxManagedContactGetIDFromJSON = 9007199254740991
)

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

func managedContactIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()[managedContactGetIDParam]
	if !exists {
		return 0, "contact_id is required"
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

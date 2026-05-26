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
	managedContactsPageSizeMin      = 25
	managedContactsPageSizeMax      = 500
	managedContactGetIDParam        = "contact_id"
	errManagedContactGetIDPositive  = "contact_id must be a positive integer"
	maxManagedContactGetIDFromJSON  = 9007199254740991
	managedContactUpdateIDParam     = "contact_id"
	managedContactUpdateNameParam   = "name"
	managedContactUpdateEmailParam  = "email"
	managedContactUpdateGroupParam  = "group"
	managedContactUpdatePhone1Param = "phone_primary"
	managedContactUpdatePhone2Param = "phone_secondary"
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

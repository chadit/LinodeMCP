package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	managedContactsPageSizeMin = 25
	managedContactsPageSizeMax = 500
)

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

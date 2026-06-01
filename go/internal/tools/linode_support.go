package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	supportTicketGetToolName   = "linode_support_ticket_get"
	supportTicketIDParam       = "ticket_id"
	errSupportTicketIDMissing  = "ticket_id is required"
	errSupportTicketIDPositive = "ticket_id must be a positive integer"
)

// NewLinodeSupportTicketGetTool creates a tool for retrieving one support ticket.
func NewLinodeSupportTicketGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		supportTicketGetToolName,
		"Gets one support ticket by ticket_id.",
		[]mcp.ToolOption{
			mcp.WithNumber(supportTicketIDParam, mcp.Required(), mcp.Description("Numeric support ticket ID to retrieve.")),
		},
		handleLinodeSupportTicketGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeSupportTicketGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	ticketID, validationMessage := supportTicketIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ticket, getFailureMessage := getSupportTicket(ctx, client, ticketID)
	if getFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + supportTicketGetToolName + ": " + getFailureMessage), nil
	}

	return MarshalToolResponse(ticket)
}

func supportTicketIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredPositiveIntArgument(request, supportTicketIDParam, errSupportTicketIDMissing, errSupportTicketIDPositive)
}

func getSupportTicket(ctx context.Context, client *linode.Client, ticketID int) (linode.SupportTicket, string) {
	ticket, err := client.GetSupportTicket(ctx, ticketID)
	if err != nil {
		return linode.SupportTicket{}, err.Error()
	}

	return ticket, ""
}

// NewLinodeSupportTicketsTool creates a tool for listing support tickets.
func NewLinodeSupportTicketsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_support_tickets",
		"Lists support tickets for the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeSupportTicketsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeSupportTicketsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := supportTicketsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tickets, listFailure := client.ListSupportTickets(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(tickets)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_support_tickets: " + listFailure.Error()), nil
}

func supportTicketsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
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

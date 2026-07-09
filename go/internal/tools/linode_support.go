package tools

import (
	"context"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	supportTicketGetToolName     = "linode_support_ticket_get"
	supportTicketRepliesToolName = "linode_support_ticket_reply_list"
	supportTicketIDParam         = "ticket_id"
	errSupportTicketIDMissing    = "ticket_id is required"
	errSupportTicketIDPositive   = "ticket_id must be a positive integer"
)

// NewLinodeSupportTicketGetTool creates a tool for retrieving one support ticket.
func NewLinodeSupportTicketGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		supportTicketGetToolName,
		"Gets one support ticket by ticket_id.",
		toolschemas.Schema("linode.mcp.v1.SupportTicketGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSupportTicketGetRequest(ctx, &request, cfg)
	}

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

	ticket, getFailure := client.GetSupportTicketProto(ctx, ticketID)
	if getFailure == nil {
		return MarshalProtoToolResponse(ticket)
	}

	return mcp.NewToolResultError("Failed to retrieve " + supportTicketGetToolName + ": " + getFailure.Error()), nil
}

func supportTicketIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredIDArgument(request, supportTicketIDParam)
}

// NewLinodeSupportTicketsTool creates a tool for listing support tickets.
func NewLinodeSupportTicketsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_support_ticket_list",
		"Lists support tickets for the authenticated account.",
		"linode.mcp.v1.SupportTicketListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.SupportTicket, error) {
			return client.ListSupportTicketsProto(ctx, page, pageSize)
		},
		supportTicketsPaginationFromTool,
		nil,
		supportTicketListResponse,
	)

	return tool, profiles.CapRead, handler
}

func supportTicketListResponse(items []*linodev1.SupportTicket, count int32, filter *string) *linodev1.SupportTicketListResponse {
	return &linodev1.SupportTicketListResponse{Count: count, Filter: filter, SupportTickets: items}
}

// NewLinodeSupportTicketCloseTool creates a tool for closing one support ticket.
func NewLinodeSupportTicketCloseTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_support_ticket_close",
		"Closes one support ticket by ID. Pass dry_run=true to preview without closing.",
		toolschemas.Schema("linode.mcp.v1.SupportTicketCloseInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSupportTicketCloseRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
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

func handleLinodeSupportTicketCloseRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	ticketID, validationMessage := supportTicketIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	path := supportTicketsPath + "/" + strconv.Itoa(ticketID) + "/close"
	if IsDryRun(request) {
		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_support_ticket_close", httpMethodPost, path, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return supportTicketCloseSideEffects(ctx, ticketID)
			})
	}

	if result := RequireConfirm(request, "This closes a support ticket. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	if closeFailureMessage := closeSupportTicketErrorMessage(ctx, client, ticketID); closeFailureMessage != "" {
		return mcp.NewToolResultError(closeFailureMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.SupportTicketIDResponse{
		Message:  "Support ticket closed successfully",
		TicketId: linodeIDToInt32(ticketID),
	})
}

func closeSupportTicketErrorMessage(ctx context.Context, client *linode.Client, ticketID int) string {
	if err := client.CloseSupportTicket(ctx, ticketID); err != nil {
		return "Failed to close linode_support_ticket_close: " + err.Error()
	}

	return ""
}

// NewLinodeSupportTicketRepliesTool creates a tool for listing replies for one support ticket.
func NewLinodeSupportTicketRepliesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourcePaginatedRawSchema(
		cfg,
		supportTicketRepliesToolName,
		"Lists replies for one support ticket by ticket_id.",
		"linode.mcp.v1.SupportTicketReplyListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber(supportTicketIDParam, mcp.Required(),
				mcp.Description("Numeric support ticket ID whose replies should be listed.")),
			parse: supportTicketIDFromTool,
		},
		supportTicketsPaginationFromTool,
		func(ctx context.Context, client *linode.Client, ticketID, page, pageSize int) ([]*linodev1.SupportTicketReply, error) {
			return client.ListSupportTicketRepliesProto(ctx, ticketID, page, pageSize)
		},
		nil,
		supportTicketReplyListResponse,
	)

	return tool, profiles.CapRead, handler
}

func supportTicketReplyListResponse(items []*linodev1.SupportTicketReply, count int32, filter *string) *linodev1.SupportTicketReplyListResponse {
	return &linodev1.SupportTicketReplyListResponse{Count: count, Filter: filter, SupportTicketReplies: items}
}

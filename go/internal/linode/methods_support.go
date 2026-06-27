package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const endpointSupportTickets = "/support/tickets"

// httpCreateSupportTicket opens a support ticket.
func (c *Client) httpCreateSupportTicket(ctx context.Context, request *CreateSupportTicketRequest) (*SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSupportTickets, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicket", Err: err}
	}

	defer drainClose(resp)

	var ticket SupportTicket
	if err := c.handleResponse(resp, &ticket); err != nil {
		return nil, err
	}

	return &ticket, nil
}

// httpGetSupportTicket retrieves one support ticket by ID.
func (c *Client) httpGetSupportTicket(ctx context.Context, ticketID int) (SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return SupportTicket{}, &NetworkError{Operation: "GetSupportTicket", Err: err}
	}

	defer drainClose(resp)

	var ticket SupportTicket
	if err := c.handleResponse(resp, &ticket); err != nil {
		return SupportTicket{}, err
	}

	return ticket, nil
}

// httpGetSupportTicketProto retrieves one support ticket as a proto message.
func (c *Client) httpGetSupportTicketProto(ctx context.Context, ticketID int) (*linodev1.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetSupportTicket", Err: err}
	}

	defer drainClose(resp)

	ticket := &linodev1.SupportTicket{}
	if err := c.handleProtoResponse(resp, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// httpCreateSupportTicketAttachment creates an attachment for a support ticket.
func (c *Client) httpCreateSupportTicketAttachment(ctx context.Context, ticketID int, request *CreateSupportTicketAttachmentRequest) (*SupportTicketAttachment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID)) + "/attachments"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicketAttachment", Err: err}
	}

	defer drainClose(resp)

	var attachment SupportTicketAttachment
	if err := c.handleResponse(resp, &attachment); err != nil {
		return nil, err
	}

	return &attachment, nil
}

// httpCreateSupportTicketReply creates a reply for an existing support ticket.
func (c *Client) httpCreateSupportTicketReply(ctx context.Context, ticketID int, request *CreateSupportTicketReplyRequest) (*SupportTicketReply, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID)) + "/replies"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicketReply", Err: err}
	}

	defer drainClose(resp)

	var reply SupportTicketReply
	if err := c.handleResponse(resp, &reply); err != nil {
		return nil, err
	}

	return &reply, nil
}

// httpListSupportTickets retrieves support tickets.

func (c *Client) httpListSupportTickets(ctx context.Context, page, pageSize int) (*PaginatedResponse[SupportTicket], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointSupportTickets, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListSupportTickets", Err: err}
	}

	defer drainClose(resp)

	var tickets PaginatedResponse[SupportTicket]
	if err := c.handleResponse(resp, &tickets); err != nil {
		return nil, err
	}

	return &tickets, nil
}

// httpCloseSupportTicket closes one support ticket.
func (c *Client) httpCloseSupportTicket(ctx context.Context, ticketID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSupportTickets+"/"+url.PathEscape(strconv.Itoa(ticketID))+"/close", nil)
	if err != nil {
		return &NetworkError{Operation: "CloseSupportTicket", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all support methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpListSupportTicketReplies retrieves replies for a support ticket.
func (c *Client) httpListSupportTicketReplies(ctx context.Context, ticketID, page, pageSize int) (*PaginatedResponse[SupportTicketReply], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID)) + "/replies"
	endpoint = withPaginationQuery(endpoint, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListSupportTicketReplies", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all list methods use this pattern

	var replies PaginatedResponse[SupportTicketReply]
	if err := c.handleResponse(resp, &replies); err != nil {
		return nil, err
	}

	return &replies, nil
}

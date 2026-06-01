package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
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

	defer drainClose(resp) // errcheck: body close is best-effort; all support methods use this pattern

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

	defer drainClose(resp) // errcheck: body close is best-effort; all support ticket methods use this pattern

	var ticket SupportTicket
	if err := c.handleResponse(resp, &ticket); err != nil {
		return SupportTicket{}, err
	}

	return ticket, nil
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

	defer drainClose(resp) // errcheck: body close is best-effort; all list methods use this pattern

	var tickets PaginatedResponse[SupportTicket]
	if err := c.handleResponse(resp, &tickets); err != nil {
		return nil, err
	}

	return &tickets, nil
}

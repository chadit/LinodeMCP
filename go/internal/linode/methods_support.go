package linode

import (
	"context"
	"net/http"
)

const endpointSupportTickets = "/support/tickets"

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

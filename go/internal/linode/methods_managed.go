package linode

import (
	"context"
	"net/http"
)

const endpointManagedContacts = "/managed/contacts"

// httpListManagedContacts retrieves Managed contacts.
func (c *Client) httpListManagedContacts(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedContact], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointManagedContacts, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListManagedContacts", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var contacts PaginatedResponse[ManagedContact]
	if err := c.handleResponse(resp, &contacts); err != nil {
		return nil, err
	}

	return &contacts, nil
}

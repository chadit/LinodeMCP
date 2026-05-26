package linode

import (
	"context"
	"net/http"
)

// httpListMonitorAlertChannels retrieves monitoring alert channels.
func (c *Client) httpListMonitorAlertChannels(ctx context.Context, page, pageSize int) (*PaginatedResponse[AlertChannel], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointMonitorAlertChannels, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorAlertChannels", Err: err}
	}

	defer drainClose(resp)

	var channels PaginatedResponse[AlertChannel]
	if err := c.handleResponse(resp, &channels); err != nil {
		return nil, err
	}

	return &channels, nil
}

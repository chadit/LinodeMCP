package linode

import (
	"context"
	"net/http"
)

// httpListMonitorDashboards retrieves monitoring dashboards.
func (c *Client) httpListMonitorDashboards(ctx context.Context, page, pageSize int) (*PaginatedResponse[MonitorDashboard], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointMonitorDashboards, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorDashboards", Err: err}
	}

	defer drainClose(resp)

	var dashboards PaginatedResponse[MonitorDashboard]
	if err := c.handleResponse(resp, &dashboards); err != nil {
		return nil, err
	}

	return &dashboards, nil
}

// httpListMonitorAlertDefinitions retrieves monitoring alert definitions.
func (c *Client) httpListMonitorAlertDefinitions(ctx context.Context, page, pageSize int) (*PaginatedResponse[AlertDefinition], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointMonitorAlertDefinitions, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorAlertDefinitions", Err: err}
	}

	defer drainClose(resp)

	var definitions PaginatedResponse[AlertDefinition]
	if err := c.handleResponse(resp, &definitions); err != nil {
		return nil, err
	}

	return &definitions, nil
}

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

package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// httpListMonitorServices retrieves supported monitoring service types.
func (c *Client) httpListMonitorServices(ctx context.Context) (*PaginatedResponse[MonitorService], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointMonitorServices, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorServices", Err: err}
	}

	defer drainClose(resp)

	var services PaginatedResponse[MonitorService]
	if err := c.handleResponse(resp, &services); err != nil {
		return nil, err
	}

	return &services, nil
}

// httpGetMonitorService retrieves details for one supported monitoring service type.
func (c *Client) httpGetMonitorService(ctx context.Context, serviceType string) (MonitorService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return MonitorService{}, &NetworkError{Operation: "GetMonitorService", Err: err}
	}

	defer drainClose(resp)

	var service MonitorService
	if err := c.handleResponse(resp, &service); err != nil {
		return MonitorService{}, err
	}

	return service, nil
}

// httpListMonitorServiceAlertDefinitions retrieves alert definitions for one monitoring service type.
func (c *Client) httpListMonitorServiceAlertDefinitions(ctx context.Context, serviceType string) (*PaginatedResponse[AlertDefinition], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorServiceAlertDefinitions", Err: err}
	}

	defer drainClose(resp)

	var definitions PaginatedResponse[AlertDefinition]
	if err := c.handleResponse(resp, &definitions); err != nil {
		return nil, err
	}

	return &definitions, nil
}

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

// httpGetMonitorDashboard retrieves one monitoring dashboard.
func (c *Client) httpGetMonitorDashboard(ctx context.Context, dashboardID int) (MonitorDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorDashboards + "/" + url.PathEscape(strconv.Itoa(dashboardID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetMonitorDashboard", Err: err}
	}

	defer drainClose(resp)

	var dashboard MonitorDashboard
	if err := c.handleResponse(resp, &dashboard); err != nil {
		return nil, err
	}

	return dashboard, nil
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

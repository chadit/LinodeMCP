package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
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

// httpGetMonitorServiceProto retrieves a Monitor service as a proto message.
func (c *Client) httpGetMonitorServiceProto(ctx context.Context, serviceType string) (*linodev1.MonitorService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetMonitorService", Err: err}
	}

	defer drainClose(resp)

	service := &linodev1.MonitorService{}
	if err := c.handleProtoResponse(resp, service); err != nil {
		return nil, err
	}

	return service, nil
}

// httpListMonitorServiceMetricDefinitions retrieves metric definitions for one monitoring service type.
func (c *Client) httpListMonitorServiceMetricDefinitions(ctx context.Context, serviceType string) (*PaginatedResponse[MonitorMetricDefinition], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/metric-definitions"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorServiceMetricDefinitions", Err: err}
	}

	defer drainClose(resp)

	var definitions PaginatedResponse[MonitorMetricDefinition]
	if err := c.handleResponse(resp, &definitions); err != nil {
		return nil, err
	}

	return &definitions, nil
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

// httpListMonitorServiceDashboards retrieves dashboards for one monitoring service type.
func (c *Client) httpListMonitorServiceDashboards(ctx context.Context, serviceType string) (*PaginatedResponse[MonitorDashboard], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/dashboards"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMonitorServiceDashboards", Err: err}
	}

	defer drainClose(resp)

	var dashboards PaginatedResponse[MonitorDashboard]
	if err := c.handleResponse(resp, &dashboards); err != nil {
		return nil, err
	}

	return &dashboards, nil
}

// httpGetMonitorServiceMetrics retrieves metrics for one monitoring service type.
func (c *Client) httpGetMonitorServiceMetrics(ctx context.Context, serviceType string) (MonitorMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/metrics"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, map[string]any{})
	if err != nil {
		return nil, &NetworkError{Operation: "GetMonitorServiceMetrics", Err: err}
	}

	defer drainClose(resp)

	var metrics MonitorMetrics
	if err := c.handleResponse(resp, &metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

// httpCreateMonitorServiceToken creates a token for one monitoring service type.
func (c *Client) httpCreateMonitorServiceToken(ctx context.Context, serviceType string, request *CreateMonitorServiceTokenRequest) (*MonitorServiceToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/token"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateMonitorServiceToken", Err: err}
	}

	defer drainClose(resp)

	var token MonitorServiceToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// httpCreateMonitorServiceAlertDefinition creates an alert definition for one monitoring service type.
func (c *Client) httpCreateMonitorServiceAlertDefinition(ctx context.Context, serviceType string, request *CreateAlertDefinitionRequest) (*AlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	var definition AlertDefinition
	if err := c.handleResponse(resp, &definition); err != nil {
		return nil, err
	}

	return &definition, nil
}

// httpGetMonitorServiceAlertDefinition retrieves one alert definition for one monitoring service type.
func (c *Client) httpGetMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int) (AlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions/" + url.PathEscape(strconv.Itoa(alertID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return AlertDefinition{}, &NetworkError{Operation: "GetMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	var definition AlertDefinition
	if err := c.handleResponse(resp, &definition); err != nil {
		return AlertDefinition{}, err
	}

	return definition, nil
}

// httpDeleteMonitorServiceAlertDefinition deletes one alert definition for one monitoring service type.
func (c *Client) httpDeleteMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions/" + url.PathEscape(strconv.Itoa(alertID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUpdateMonitorServiceAlertDefinition updates one alert definition for one monitoring service type.
func (c *Client) httpUpdateMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int, request *UpdateAlertDefinitionRequest) (*AlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions/" + url.PathEscape(strconv.Itoa(alertID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	var definition AlertDefinition
	if err := c.handleResponse(resp, &definition); err != nil {
		return nil, err
	}

	return &definition, nil
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
		return MonitorDashboard(nil), &NetworkError{Operation: "GetMonitorDashboard", Err: err}
	}

	defer drainClose(resp)

	var dashboard MonitorDashboard
	if err := c.handleResponse(resp, &dashboard); err != nil {
		return MonitorDashboard(nil), err
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

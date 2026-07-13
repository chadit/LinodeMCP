package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpListMonitorServicesProto retrieves supported monitoring service types as
// proto messages for the proto-backed list path. The endpoint returns a {data:
// [...]} page envelope, so listProtoElements reads data.
func (c *Client) httpListMonitorServicesProto(ctx context.Context) ([]*linodev1.MonitorService, error) {
	return listProtoElements(ctx, c, "ListMonitorServices", endpointMonitorServices,
		func() *linodev1.MonitorService { return &linodev1.MonitorService{} })
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

// httpListMonitorServiceMetricDefinitionsProto retrieves metric definitions for
// one monitoring service type as proto messages for the proto-backed list path.
// The endpoint returns a {data:[...]} page envelope, so listProtoElements reads
// data. The service type is formatted into the path exactly like the non-proto
// method.
func (c *Client) httpListMonitorServiceMetricDefinitionsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorMetricDefinition, error) {
	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/metric-definitions"

	return listProtoElements(ctx, c, "ListMonitorServiceMetricDefinitions", endpoint,
		func() *linodev1.MonitorMetricDefinition { return &linodev1.MonitorMetricDefinition{} })
}

// httpListMonitorServiceAlertDefinitionsProto retrieves alert definitions for one
// monitoring service type as proto messages for the proto-backed list path. The
// endpoint returns a {data:[...]} page envelope, so listProtoElements reads data.
// The service type is formatted into the path exactly like the non-proto method.
func (c *Client) httpListMonitorServiceAlertDefinitionsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorAlertDefinition, error) {
	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions"

	return listProtoElements(ctx, c, "ListMonitorServiceAlertDefinitions", endpoint,
		func() *linodev1.MonitorAlertDefinition { return &linodev1.MonitorAlertDefinition{} })
}

// httpListMonitorServiceDashboardsProto retrieves dashboards for one monitoring
// service type as proto messages for the proto-backed list path. The endpoint
// returns a {data:[...]} page envelope, so listProtoElements reads data. The
// service type is formatted into the path exactly like the non-proto method.
func (c *Client) httpListMonitorServiceDashboardsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorDashboard, error) {
	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/dashboards"

	return listProtoElements(ctx, c, "ListMonitorServiceDashboards", endpoint,
		func() *linodev1.MonitorDashboard { return &linodev1.MonitorDashboard{} })
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

// httpCreateMonitorServiceToken creates a token for one monitoring service
// type and decodes the response into the MonitorServiceTokenCreateResponse
// proto message so the write tool emits the proto-canonical body.
func (c *Client) httpCreateMonitorServiceToken(ctx context.Context, serviceType string, request *CreateMonitorServiceTokenRequest) (*linodev1.MonitorServiceTokenCreateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/token"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateMonitorServiceToken", Err: err}
	}

	defer drainClose(resp)

	token := &linodev1.MonitorServiceTokenCreateResponse{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpCreateMonitorServiceAlertDefinitionProto creates an alert definition and
// decodes the response into the MonitorAlertDefinition proto element so the
// write tool emits the same field set as the alert-definition GET/LIST path.
func (c *Client) httpCreateMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, request *CreateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	definition := &linodev1.MonitorAlertDefinition{}
	if err := c.handleProtoResponse(resp, definition); err != nil {
		return nil, err
	}

	return definition, nil
}

// httpUpdateMonitorServiceAlertDefinitionProto updates an alert definition and
// decodes the response into the MonitorAlertDefinition proto element.
func (c *Client) httpUpdateMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, alertID int, request *UpdateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions/" + url.PathEscape(strconv.Itoa(alertID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	definition := &linodev1.MonitorAlertDefinition{}
	if err := c.handleProtoResponse(resp, definition); err != nil {
		return nil, err
	}

	return definition, nil
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

// httpGetMonitorServiceAlertDefinitionProto retrieves one alert definition for
// one monitoring service type and decodes it into the MonitorAlertDefinition
// proto element for the proto-backed read path. criteria, rule_criteria, and
// trigger_conditions are arbitrary JSON the element models as
// google.protobuf.Struct, so the body decodes straight into the element.
func (c *Client) httpGetMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, alertID int) (*linodev1.MonitorAlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorServices + "/" + url.PathEscape(serviceType) + "/alert-definitions/" + url.PathEscape(strconv.Itoa(alertID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetMonitorServiceAlertDefinition", Err: err}
	}

	defer drainClose(resp)

	definition := &linodev1.MonitorAlertDefinition{}
	if err := c.handleProtoResponse(resp, definition); err != nil {
		return nil, err
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

// httpListMonitorDashboardsProto retrieves monitoring dashboards as proto
// messages for the proto-backed list path. The endpoint built with
// withPaginationQuery returns a {data:[...]} page envelope, so
// listProtoElementsPaginated reads data after adding page/page_size, matching
// the non-proto request exactly.
func (c *Client) httpListMonitorDashboardsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorDashboard, error) {
	return listProtoElementsPaginated(ctx, c, "ListMonitorDashboards", endpointMonitorDashboards, page, pageSize,
		func() *linodev1.MonitorDashboard { return &linodev1.MonitorDashboard{} })
}

// httpGetMonitorDashboardProto retrieves one monitoring dashboard and decodes it
// into the MonitorDashboard proto element for the proto-backed read path. Each
// widget is arbitrary JSON the API returns verbatim, so the element models
// widgets as google.protobuf.Struct; the body decodes straight into the element.
func (c *Client) httpGetMonitorDashboardProto(ctx context.Context, dashboardID int) (*linodev1.MonitorDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointMonitorDashboards + "/" + url.PathEscape(strconv.Itoa(dashboardID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetMonitorDashboard", Err: err}
	}

	defer drainClose(resp)

	dashboard := &linodev1.MonitorDashboard{}
	if err := c.handleProtoResponse(resp, dashboard); err != nil {
		return nil, err
	}

	return dashboard, nil
}

// httpListMonitorAlertDefinitionsProto retrieves monitoring alert definitions as
// proto messages for the proto-backed list path. page/page_size flow through
// withPaginationQuery, so the request matches the non-proto method.
func (c *Client) httpListMonitorAlertDefinitionsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorAlertDefinition, error) {
	return listProtoElementsPaginated(ctx, c, "ListMonitorAlertDefinitions", endpointMonitorAlertDefinitions, page, pageSize,
		func() *linodev1.MonitorAlertDefinition { return &linodev1.MonitorAlertDefinition{} })
}

// httpListMonitorAlertChannelsProto retrieves monitoring alert channels as proto
// messages for the proto-backed list path. page/page_size flow through
// withPaginationQuery, so the request matches the non-proto method.
func (c *Client) httpListMonitorAlertChannelsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorAlertChannel, error) {
	return listProtoElementsPaginated(ctx, c, "ListMonitorAlertChannels", endpointMonitorAlertChannels, page, pageSize,
		func() *linodev1.MonitorAlertChannel { return &linodev1.MonitorAlertChannel{} })
}

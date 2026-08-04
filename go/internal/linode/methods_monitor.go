package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Proto-backed list paths read the {data:[...]} page envelope these endpoints
// return; trailing args fill the path slots the tool's route declares. Write
// paths decode into the same proto element as the matching read path, so both
// emit the same field set.

// httpListMonitorServicesProto lists supported monitoring service types.
func (c *Client) httpListMonitorServicesProto(ctx context.Context) ([]*linodev1.MonitorService, error) {
	return listProtoElementsRouted(ctx, c, "ListMonitorServices",
		"linode_monitor_service_list", "", nil,
		func() *linodev1.MonitorService { return &linodev1.MonitorService{} })
}

// httpGetMonitorServiceProto retrieves one monitoring service type.
func (c *Client) httpGetMonitorServiceProto(ctx context.Context, serviceType string) (*linodev1.MonitorService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_get", nil, serviceType)
	if err != nil {
		return nil, wrapRequestError("GetMonitorService", err)
	}

	defer drainClose(resp)

	service := &linodev1.MonitorService{}
	if err := c.handleProtoResponse(resp, service); err != nil {
		return nil, err
	}

	return service, nil
}

// httpListMonitorServiceMetricDefinitionsProto lists metric definitions for one
// monitoring service type.
func (c *Client) httpListMonitorServiceMetricDefinitionsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorMetricDefinition, error) {
	return listProtoElementsRouted(ctx, c, "ListMonitorServiceMetricDefinitions",
		"linode_monitor_service_metric_definition_list", "", []any{serviceType},
		func() *linodev1.MonitorMetricDefinition { return &linodev1.MonitorMetricDefinition{} })
}

// httpListMonitorServiceAlertDefinitionsProto lists alert definitions for one
// monitoring service type.
func (c *Client) httpListMonitorServiceAlertDefinitionsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorAlertDefinition, error) {
	return listProtoElementsRouted(ctx, c, "ListMonitorServiceAlertDefinitions",
		"linode_monitor_service_alert_definition_list", "", []any{serviceType},
		func() *linodev1.MonitorAlertDefinition { return &linodev1.MonitorAlertDefinition{} })
}

// httpListMonitorServiceDashboardsProto lists dashboards for one monitoring
// service type.
func (c *Client) httpListMonitorServiceDashboardsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorDashboard, error) {
	return listProtoElementsRouted(ctx, c, "ListMonitorServiceDashboards",
		"linode_monitor_service_dashboard_list", "", []any{serviceType},
		func() *linodev1.MonitorDashboard { return &linodev1.MonitorDashboard{} })
}

// httpGetMonitorServiceMetrics retrieves metrics for one monitoring service type.
func (c *Client) httpGetMonitorServiceMetrics(ctx context.Context, serviceType string) (MonitorMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_metric_query", map[string]any{}, serviceType)
	if err != nil {
		return nil, wrapRequestError("GetMonitorServiceMetrics", err)
	}

	defer drainClose(resp)

	var metrics MonitorMetrics
	if err := c.handleResponse(resp, &metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

// httpCreateMonitorServiceToken creates a token for one monitoring service type.
func (c *Client) httpCreateMonitorServiceToken(ctx context.Context, serviceType string, request *CreateMonitorServiceTokenRequest) (*linodev1.MonitorServiceTokenCreateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_token_create", request, serviceType)
	if err != nil {
		return nil, wrapRequestError("CreateMonitorServiceToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.MonitorServiceTokenCreateResponse{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpCreateMonitorServiceAlertDefinitionProto creates an alert definition for
// one monitoring service type.
func (c *Client) httpCreateMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, request *CreateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_alert_definition_create", request, serviceType)
	if err != nil {
		return nil, wrapRequestError("CreateMonitorServiceAlertDefinition", err)
	}

	defer drainClose(resp)

	definition := &linodev1.MonitorAlertDefinition{}
	if err := c.handleProtoResponse(resp, definition); err != nil {
		return nil, err
	}

	return definition, nil
}

// httpCloneMonitorServiceAlertDefinitionProto clones one alert definition.
func (c *Client) httpCloneMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, alertID int, request *CloneAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_alert_definition_clone", request, serviceType, alertID)
	if err != nil {
		return nil, wrapRequestError("CloneMonitorServiceAlertDefinition", err)
	}

	defer drainClose(resp)

	definition := &linodev1.MonitorAlertDefinition{}
	if err := c.handleProtoResponse(resp, definition); err != nil {
		return nil, err
	}

	return definition, nil
}

// httpUpdateMonitorServiceAlertDefinitionProto updates one alert definition.
func (c *Client) httpUpdateMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, alertID int, request *UpdateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_alert_definition_update", request, serviceType, alertID)
	if err != nil {
		return nil, wrapRequestError("UpdateMonitorServiceAlertDefinition", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_alert_definition_get", nil, serviceType, alertID)
	if err != nil {
		return AlertDefinition{}, wrapRequestError("GetMonitorServiceAlertDefinition", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_alert_definition_get", nil, serviceType, alertID)
	if err != nil {
		return nil, wrapRequestError("GetMonitorServiceAlertDefinition", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_service_alert_definition_delete", nil, serviceType, alertID)
	if err != nil {
		return wrapRequestError("DeleteMonitorServiceAlertDefinition", err)
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
	return listProtoElementsPaginatedRouted(ctx, c, "ListMonitorDashboards",
		"linode_monitor_dashboard_list", "", nil, page, pageSize,
		func() *linodev1.MonitorDashboard { return &linodev1.MonitorDashboard{} })
}

// httpGetMonitorDashboardProto retrieves one monitoring dashboard and decodes it
// into the MonitorDashboard proto element for the proto-backed read path. Each
// widget is arbitrary JSON the API returns verbatim, so the element models
// widgets as google.protobuf.Struct; the body decodes straight into the element.
func (c *Client) httpGetMonitorDashboardProto(ctx context.Context, dashboardID int) (*linodev1.MonitorDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_monitor_dashboard_get", nil, dashboardID)
	if err != nil {
		return nil, wrapRequestError("GetMonitorDashboard", err)
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
	return listProtoElementsPaginatedRouted(ctx, c, "ListMonitorAlertDefinitions",
		"linode_monitor_alert_definition_list", "", nil, page, pageSize,
		func() *linodev1.MonitorAlertDefinition { return &linodev1.MonitorAlertDefinition{} })
}

// httpListMonitorAlertChannelsProto retrieves monitoring alert channels as proto
// messages for the proto-backed list path. page/page_size flow through
// withPaginationQuery, so the request matches the non-proto method.
func (c *Client) httpListMonitorAlertChannelsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorAlertChannel, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListMonitorAlertChannels",
		"linode_monitor_alert_channel_list", "", nil, page, pageSize,
		func() *linodev1.MonitorAlertChannel { return &linodev1.MonitorAlertChannel{} })
}

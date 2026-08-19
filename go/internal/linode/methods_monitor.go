package linode

import (
	"context"
)

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

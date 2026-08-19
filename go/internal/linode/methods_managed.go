package linode

import (
	"context"
)

// httpGetManagedLinodeSettings retrieves Managed settings for one Linode.
func (c *Client) httpGetManagedLinodeSettings(ctx context.Context, linodeID int) (*ManagedLinodeSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_linode_settings_get", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("GetManagedLinodeSettings", err)
	}

	defer drainClose(resp)

	var settings ManagedLinodeSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpGetManagedContact retrieves one managed contact by ID.
func (c *Client) httpGetManagedContact(ctx context.Context, contactID int) (*ManagedContact, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_contact_get", nil, contactID)
	if err != nil {
		return nil, wrapRequestError("GetManagedContact", err)
	}

	defer drainClose(resp)

	var contact ManagedContact
	if err := c.handleResponse(resp, &contact); err != nil {
		return nil, err
	}

	return &contact, nil
}

// httpGetManagedService retrieves one Managed service by ID.
func (c *Client) httpGetManagedService(ctx context.Context, serviceID int) (*ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_get", nil, serviceID)
	if err != nil {
		return nil, wrapRequestError("GetManagedService", err)
	}

	defer drainClose(resp)

	var service ManagedService
	if err := c.handleResponse(resp, &service); err != nil {
		return nil, err
	}

	return &service, nil
}

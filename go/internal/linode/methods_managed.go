package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
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

// httpGetManagedLinodeSettingsProto retrieves Managed Linode settings as a proto
// message.
func (c *Client) httpGetManagedLinodeSettingsProto(ctx context.Context, linodeID int) (*linodev1.ManagedLinodeSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_linode_settings_get", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("GetManagedLinodeSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.ManagedLinodeSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
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

// httpGetManagedContactProto retrieves a Managed contact as a proto message.
func (c *Client) httpGetManagedContactProto(ctx context.Context, contactID int) (*linodev1.ManagedContact, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_contact_get", nil, contactID)
	if err != nil {
		return nil, wrapRequestError("GetManagedContact", err)
	}

	defer drainClose(resp)

	contact := &linodev1.ManagedContact{}
	if err := c.handleProtoResponse(resp, contact); err != nil {
		return nil, err
	}

	return contact, nil
}

// httpDeleteManagedContact deletes one Managed contact.
func (c *Client) httpDeleteManagedContact(ctx context.Context, contactID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_contact_delete", nil, contactID)
	if err != nil {
		return wrapRequestError("DeleteManagedContact", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetManagedStats retrieves Managed statistics from the last 24 hours.
func (c *Client) httpGetManagedStats(ctx context.Context) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_stats_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetManagedStats", err)
	}

	defer drainClose(resp)

	var stats map[string]any
	if err := c.handleResponse(resp, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// httpUpdateManagedLinodeSettingsProto updates Managed settings for one Linode
// and decodes the response into the proto element so the write tool emits the
// same field set as the settings GET/LIST path.
func (c *Client) httpUpdateManagedLinodeSettingsProto(ctx context.Context, linodeID int, req UpdateManagedLinodeSettingsRequest) (*linodev1.ManagedLinodeSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_linode_settings_update", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("UpdateManagedLinodeSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.ManagedLinodeSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
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

// httpGetManagedServiceProto retrieves a Managed service as a proto message.
func (c *Client) httpGetManagedServiceProto(ctx context.Context, serviceID int) (*linodev1.ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_get", nil, serviceID)
	if err != nil {
		return nil, wrapRequestError("GetManagedService", err)
	}

	defer drainClose(resp)

	service := &linodev1.ManagedService{}
	if err := c.handleProtoResponse(resp, service); err != nil {
		return nil, err
	}

	return service, nil
}

// httpUpdateManagedServiceProto updates one Managed service monitor and decodes
// the response into the proto element so the write tool emits the same field set
// as the service GET/LIST path.
func (c *Client) httpUpdateManagedServiceProto(ctx context.Context, serviceID int, req *UpdateManagedServiceRequest) (*linodev1.ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_update", req, serviceID)
	if err != nil {
		return nil, wrapRequestError("UpdateManagedService", err)
	}

	defer drainClose(resp)

	service := &linodev1.ManagedService{}
	if err := c.handleProtoResponse(resp, service); err != nil {
		return nil, err
	}

	return service, nil
}

// httpDeleteManagedService deletes one Managed service monitor.
func (c *Client) httpDeleteManagedService(ctx context.Context, serviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_delete", nil, serviceID)
	if err != nil {
		return wrapRequestError("DeleteManagedService", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpDisableManagedService disables one Managed service monitor.
func (c *Client) httpDisableManagedService(ctx context.Context, serviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_disable", nil, serviceID)
	if err != nil {
		return wrapRequestError("DisableManagedService", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpEnableManagedService enables one Managed service monitor.
func (c *Client) httpEnableManagedService(ctx context.Context, serviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_enable", nil, serviceID)
	if err != nil {
		return wrapRequestError("EnableManagedService", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListManagedServicesProto retrieves Managed services as proto messages for
// the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListManagedServices.
func (c *Client) httpListManagedServicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedService, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListManagedServices",
		"linode_managed_service_list", "", nil, page, pageSize,
		func() *linodev1.ManagedService { return &linodev1.ManagedService{} })
}

// httpListManagedContactsProto retrieves Managed contacts as proto messages for
// the proto-backed list path. page/page_size flows through withPaginationQuery,
// so the request matches httpListManagedContacts.
func (c *Client) httpListManagedContactsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedContact, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListManagedContacts",
		"linode_managed_contact_list", "", nil, page, pageSize,
		func() *linodev1.ManagedContact { return &linodev1.ManagedContact{} })
}

// httpListManagedLinodeSettingsProto retrieves Managed Linode settings as proto
// messages for the proto-backed list path. page/page_size flows through
// withPaginationQuery, so the request matches httpListManagedLinodeSettings.
func (c *Client) httpListManagedLinodeSettingsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedLinodeSettings, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListManagedLinodeSettings",
		"linode_managed_linode_settings_list", "", nil, page, pageSize,
		func() *linodev1.ManagedLinodeSettings { return &linodev1.ManagedLinodeSettings{} })
}

// httpListManagedIssuesProto retrieves Managed issues as proto messages for the
// proto-backed list path. page/page_size flows through withPaginationQuery, so
// the request matches httpListManagedIssues.
func (c *Client) httpListManagedIssuesProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedIssue, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListManagedIssues",
		"linode_managed_issue_list", "", nil, page, pageSize,
		func() *linodev1.ManagedIssue { return &linodev1.ManagedIssue{} })
}

// httpGetManagedIssueProto retrieves one Managed issue as a proto message.
func (c *Client) httpGetManagedIssueProto(ctx context.Context, issueID int) (*linodev1.ManagedIssue, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_issue_get", nil, issueID)
	if err != nil {
		return nil, wrapRequestError("GetManagedIssue", err)
	}

	defer drainClose(resp)

	issue := &linodev1.ManagedIssue{}
	if err := c.handleProtoResponse(resp, issue); err != nil {
		return nil, err
	}

	return issue, nil
}

// httpCreateManagedServiceProto creates a Managed service monitor and decodes the
// response into the proto element so the write tool emits the same field set as
// the service GET/LIST path.
func (c *Client) httpCreateManagedServiceProto(ctx context.Context, request *CreateManagedServiceRequest) (*linodev1.ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_service_create", request)
	if err != nil {
		return nil, wrapRequestError("CreateManagedService", err)
	}

	defer drainClose(resp)

	service := &linodev1.ManagedService{}
	if err := c.handleProtoResponse(resp, service); err != nil {
		return nil, err
	}

	return service, nil
}

// httpUpdateManagedContactProto updates one Managed contact and decodes the
// response into the proto element so the write tool emits the same field set as
// the contact GET/LIST path.
func (c *Client) httpUpdateManagedContactProto(ctx context.Context, contactID int, req UpdateManagedContactRequest) (*linodev1.ManagedContact, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_contact_update", req, contactID)
	if err != nil {
		return nil, wrapRequestError("UpdateManagedContact", err)
	}

	defer drainClose(resp)

	contact := &linodev1.ManagedContact{}
	if err := c.handleProtoResponse(resp, contact); err != nil {
		return nil, err
	}

	return contact, nil
}

// httpCreateManagedContactProto creates a managed contact and decodes the
// response into the proto element so the write tool emits the same field set as
// the contact GET/LIST path.
func (c *Client) httpCreateManagedContactProto(ctx context.Context, request *CreateManagedContactRequest) (*linodev1.ManagedContact, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_contact_create", request)
	if err != nil {
		return nil, wrapRequestError("CreateManagedContact", err)
	}

	defer drainClose(resp)

	contact := &linodev1.ManagedContact{}
	if err := c.handleProtoResponse(resp, contact); err != nil {
		return nil, err
	}

	return contact, nil
}

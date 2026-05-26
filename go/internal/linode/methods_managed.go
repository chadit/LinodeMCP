package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

const (
	endpointManagedContacts       = "/managed/contacts"
	endpointManagedServices       = "/managed/services"
	endpointManagedIssues         = "/managed/issues"
	endpointManagedLinodeSettings = "/managed/linode-settings"
)

// httpGetManagedLinodeSettings retrieves Managed settings for one Linode.
func (c *Client) httpGetManagedLinodeSettings(ctx context.Context, linodeID int) (*ManagedLinodeSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedLinodeSettings + "/" + url.PathEscape(strconv.Itoa(linodeID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedLinodeSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

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

	endpoint := endpointManagedContacts + "/" + url.PathEscape(strconv.Itoa(contactID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedContact", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var contact ManagedContact
	if err := c.handleResponse(resp, &contact); err != nil {
		return nil, err
	}

	return &contact, nil
}

// httpDeleteManagedContact deletes one Managed contact.
func (c *Client) httpDeleteManagedContact(ctx context.Context, contactID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedContacts + "/" + url.PathEscape(strconv.Itoa(contactID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteManagedContact", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	return c.handleResponse(resp, nil)
}

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

// httpListManagedLinodeSettings retrieves Managed settings for Linodes.
func (c *Client) httpListManagedLinodeSettings(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedLinodeSettings], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointManagedLinodeSettings, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListManagedLinodeSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var settings PaginatedResponse[ManagedLinodeSettings]
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpUpdateManagedLinodeSettings updates Managed settings for one Linode.
func (c *Client) httpUpdateManagedLinodeSettings(ctx context.Context, linodeID int, req UpdateManagedLinodeSettingsRequest) (*ManagedLinodeSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedLinodeSettings + "/" + url.PathEscape(strconv.Itoa(linodeID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateManagedLinodeSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var settings ManagedLinodeSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpGetManagedService retrieves one Managed service by ID.
func (c *Client) httpGetManagedService(ctx context.Context, serviceID int) (*ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedServices + "/" + url.PathEscape(strconv.Itoa(serviceID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedService", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var service ManagedService
	if err := c.handleResponse(resp, &service); err != nil {
		return nil, err
	}

	return &service, nil
}

// httpUpdateManagedService updates one Managed service monitor.
func (c *Client) httpUpdateManagedService(ctx context.Context, serviceID int, req *UpdateManagedServiceRequest) (*ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedServices + "/" + url.PathEscape(strconv.Itoa(serviceID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateManagedService", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var service ManagedService
	if err := c.handleResponse(resp, &service); err != nil {
		return nil, err
	}

	return &service, nil
}

// httpDeleteManagedService deletes one Managed service monitor.
func (c *Client) httpDeleteManagedService(ctx context.Context, serviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedServices + "/" + url.PathEscape(strconv.Itoa(serviceID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteManagedService", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpDisableManagedService disables one Managed service monitor.
func (c *Client) httpDisableManagedService(ctx context.Context, serviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedServices + "/" + url.PathEscape(strconv.Itoa(serviceID)) + "/disable"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DisableManagedService", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpEnableManagedService enables one Managed service monitor.
func (c *Client) httpEnableManagedService(ctx context.Context, serviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedServices + "/" + url.PathEscape(strconv.Itoa(serviceID)) + "/enable"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "EnableManagedService", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpListManagedServices retrieves Managed services.
func (c *Client) httpListManagedServices(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedService], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointManagedServices, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListManagedServices", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var services PaginatedResponse[ManagedService]
	if err := c.handleResponse(resp, &services); err != nil {
		return nil, err
	}

	return &services, nil
}

// httpGetManagedIssue retrieves one Managed issue by ID.
func (c *Client) httpGetManagedIssue(ctx context.Context, issueID int) (*ManagedIssue, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedIssues + "/" + url.PathEscape(strconv.Itoa(issueID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedIssue", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var issue ManagedIssue
	if err := c.handleResponse(resp, &issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// httpListManagedIssues retrieves Managed issues.
func (c *Client) httpListManagedIssues(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedIssue], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointManagedIssues, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListManagedIssues", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var issues PaginatedResponse[ManagedIssue]
	if err := c.handleResponse(resp, &issues); err != nil {
		return nil, err
	}

	return &issues, nil
}

// httpCreateManagedService creates a Managed service monitor.
func (c *Client) httpCreateManagedService(ctx context.Context, request *CreateManagedServiceRequest) (*ManagedService, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointManagedServices, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateManagedService", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var service ManagedService
	if err := c.handleResponse(resp, &service); err != nil {
		return nil, err
	}

	return &service, nil
}

// httpCreateManagedContact creates a managed contact.
func (c *Client) httpCreateManagedContact(ctx context.Context, request *CreateManagedContactRequest) (*ManagedContact, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointManagedContacts, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateManagedContact", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var contact ManagedContact
	if err := c.handleResponse(resp, &contact); err != nil {
		return nil, err
	}

	return &contact, nil
}

// httpUpdateManagedContact updates one Managed contact.
func (c *Client) httpUpdateManagedContact(ctx context.Context, contactID int, req UpdateManagedContactRequest) (*ManagedContact, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedContacts + "/" + url.PathEscape(strconv.Itoa(contactID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateManagedContact", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all client methods use this pattern

	var contact ManagedContact
	if err := c.handleResponse(resp, &contact); err != nil {
		return nil, err
	}

	return &contact, nil
}

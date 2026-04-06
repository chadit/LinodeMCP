package linode

import (
	"context"
	"fmt"
	"net/http"
)

const (
	endpointInstances    = "/linode/instances"
	endpointRegions      = "/regions"
	endpointTypes        = "/linode/types"
	endpointImages       = "/images"
	endpointStackScripts = "/linode/stackscripts"
)

// ListInstances retrieves all Linode instances for the authenticated user.
func (c *Client) httpListInstances(ctx context.Context) ([]Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointInstances, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstances", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[Instance]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetInstance retrieves a single Linode instance by its ID.
func (c *Client) httpGetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// ListRegions retrieves all available Linode regions.
func (c *Client) httpListRegions(ctx context.Context) ([]Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointRegions, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListRegions", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[Region]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListTypes retrieves all available Linode instance types.
func (c *Client) httpListTypes(ctx context.Context) ([]InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointTypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListTypes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[InstanceType]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListImages retrieves all available Linode images.
func (c *Client) httpListImages(ctx context.Context) ([]Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointImages, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImages", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[Image]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListStackScripts retrieves StackScripts available to the authenticated user.
func (c *Client) httpListStackScripts(ctx context.Context) ([]StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointStackScripts, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListStackScripts", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[StackScript]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// BootInstance boots a Linode instance.
func (c *Client) httpBootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/boot", instanceID)

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "BootInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RebootInstance reboots a Linode instance.
func (c *Client) httpRebootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/reboot", instanceID)

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "RebootInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ShutdownInstance shuts down a Linode instance.
func (c *Client) httpShutdownInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/shutdown", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ShutdownInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateInstance creates a new Linode instance.
func (c *Client) httpCreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointInstances, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// DeleteInstance deletes a Linode instance.
func (c *Client) httpDeleteInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ResizeInstance resizes a Linode instance to a new plan.
func (c *Client) httpResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/resize", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "ResizeInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

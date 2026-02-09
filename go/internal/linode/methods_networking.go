package linode

import (
	"context"
	"fmt"
	"net/http"
)

// ListFirewalls retrieves all Cloud Firewalls for the authenticated user.
func (c *Client) ListFirewalls(ctx context.Context) ([]Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/networking/firewalls", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewalls", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Firewall `json:"data"`
		Page    int        `json:"page"`
		Pages   int        `json:"pages"`
		Results int        `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetFirewall retrieves a single firewall by its ID.
func (c *Client) GetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/networking/firewalls/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// CreateFirewall creates a new Cloud Firewall.
func (c *Client) CreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/networking/firewalls", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// UpdateFirewall updates an existing Cloud Firewall.
func (c *Client) UpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/networking/firewalls/%d", firewallID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// DeleteFirewall deletes a Cloud Firewall.
func (c *Client) DeleteFirewall(ctx context.Context, firewallID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/networking/firewalls/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ListNodeBalancers retrieves all NodeBalancers for the authenticated user.
func (c *Client) ListNodeBalancers(ctx context.Context) ([]NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/nodebalancers", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancers", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []NodeBalancer `json:"data"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
		Results int            `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetNodeBalancer retrieves a single NodeBalancer by its ID.
func (c *Client) GetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/nodebalancers/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// CreateNodeBalancer creates a new NodeBalancer.
func (c *Client) CreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/nodebalancers", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// UpdateNodeBalancer updates an existing NodeBalancer.
func (c *Client) UpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/nodebalancers/%d", nodeBalancerID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// DeleteNodeBalancer deletes a NodeBalancer.
func (c *Client) DeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/nodebalancers/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

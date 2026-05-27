package linode

import (
	"context"
	"fmt"
	"net/http"
)

const (
	endpointFirewalls             = "/networking/firewalls"
	endpointFirewallSettings      = endpointFirewalls + "/settings"
	endpointNetworkTransferPrices = "/network-transfer/prices"
	endpointNodeBalancers         = "/nodebalancers"
)

// ListFirewalls retrieves all Cloud Firewalls for the authenticated user.
func (c *Client) httpListFirewalls(ctx context.Context) ([]Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointFirewalls, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListFirewallSettings retrieves default firewall assignments.
func (c *Client) httpListFirewallSettings(ctx context.Context, page, pageSize int) (*FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallSettings, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallSettings", Err: err}
	}

	defer drainClose(resp)

	var settings FirewallSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpUpdateFirewallSettings updates default firewall assignments.
func (c *Client) httpUpdateFirewallSettings(ctx context.Context, req *UpdateFirewallSettingsRequest) (*FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointFirewallSettings, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewallSettings", Err: err}
	}

	defer drainClose(resp)

	var settings FirewallSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// GetFirewall retrieves a single firewall by its ID.
func (c *Client) httpGetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewall", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// CreateFirewall creates a new Cloud Firewall.
func (c *Client) httpCreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointFirewalls, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewall", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// UpdateFirewall updates an existing Cloud Firewall.
func (c *Client) httpUpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewall", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// DeleteFirewall deletes a Cloud Firewall.
func (c *Client) httpDeleteFirewall(ctx context.Context, firewallID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteFirewall", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ListNetworkTransferPrices retrieves network transfer prices.
func (c *Client) httpListNetworkTransferPrices(ctx context.Context) (*PaginatedResponse[NetworkTransferPrice], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointNetworkTransferPrices, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNetworkTransferPrices", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NetworkTransferPrice]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListNodeBalancers retrieves all NodeBalancers for the authenticated user.
func (c *Client) httpListNodeBalancers(ctx context.Context) ([]NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointNodeBalancers, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancers", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancer]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetNodeBalancer retrieves a single NodeBalancer by its ID.
func (c *Client) httpGetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// CreateNodeBalancer creates a new NodeBalancer.
func (c *Client) httpCreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNodeBalancers, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// UpdateNodeBalancer updates an existing NodeBalancer.
func (c *Client) httpUpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// DeleteNodeBalancer deletes a NodeBalancer.
func (c *Client) httpDeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

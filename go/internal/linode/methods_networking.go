package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	endpointFirewalls             = "/networking/firewalls"
	endpointFirewallSettings      = endpointFirewalls + "/settings"
	endpointFirewallTemplates     = endpointFirewalls + "/templates"
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

// ListFirewallDevices retrieves devices assigned to a Cloud Firewall.
func (c *Client) httpListFirewallDevices(ctx context.Context, firewallID, page, pageSize int) (*PaginatedResponse[FirewallDevice], error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := withPaginationQuery(endpointFirewalls+"/"+encodedFirewallID+"/devices", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallDevices", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallDevice]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateFirewallDevice assigns a device to a Cloud Firewall.
func (c *Client) httpCreateFirewallDevice(ctx context.Context, firewallID int, req *CreateFirewallDeviceRequest) (*FirewallDevice, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if req == nil {
		return nil, ErrFirewallDeviceIDPositive
	}

	if req.ID <= 0 {
		return nil, ErrFirewallDeviceIDPositive
	}

	if req.Type == "" {
		return nil, ErrFirewallDeviceTypeRequired
	}

	if !isFirewallDeviceType(req.Type) {
		return nil, ErrInvalidFirewallDeviceType
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewallDevice", Err: err}
	}

	defer drainClose(resp)

	var device FirewallDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// GetFirewallDevice retrieves one device assigned to a Cloud Firewall.
func (c *Client) httpGetFirewallDevice(ctx context.Context, firewallID, deviceID int) (*FirewallDevice, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if deviceID <= 0 {
		return nil, ErrFirewallDeviceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	encodedDeviceID := url.PathEscape(strconv.Itoa(deviceID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices/" + encodedDeviceID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewallDevice", Err: err}
	}

	defer drainClose(resp)

	var device FirewallDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// DeleteFirewallDevice removes one device assignment from a Cloud Firewall.
func (c *Client) httpDeleteFirewallDevice(ctx context.Context, firewallID, deviceID int) error {
	if firewallID <= 0 {
		return ErrFirewallIDPositive
	}

	if deviceID <= 0 {
		return ErrFirewallDeviceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	encodedDeviceID := url.PathEscape(strconv.Itoa(deviceID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices/" + encodedDeviceID

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteFirewallDevice", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

func isFirewallDeviceType(deviceType string) bool {
	switch deviceType {
	case "linode", "nodebalancer", "linode_interface":
		return true
	default:
		return false
	}
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

// ListFirewallTemplates retrieves reusable Cloud Firewall templates.
func (c *Client) httpListFirewallTemplates(ctx context.Context, page, pageSize int) (*PaginatedResponse[FirewallTemplate], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallTemplates, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallTemplates", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallTemplate]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func isFirewallTemplateSlug(slug string) bool {
	switch slug {
	case "public", "vpc":
		return true
	default:
		return false
	}
}

// GetFirewallTemplate retrieves a reusable Cloud Firewall template by slug.
func (c *Client) httpGetFirewallTemplate(ctx context.Context, slug string, page, pageSize int) (*PaginatedResponse[FirewallTemplate], error) {
	if !isFirewallTemplateSlug(slug) {
		return nil, ErrInvalidFirewallTemplateSlug
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallTemplates+"/"+url.PathEscape(slug), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewallTemplate", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallTemplate]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
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

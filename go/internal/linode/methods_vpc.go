package linode

import (
	"context"
	"fmt"
	"net/http"
)

const (
	endpointVPCs = "/vpcs"
)

// ListVPCs retrieves all VPCs for the authenticated user.
func (c *Client) httpListVPCs(ctx context.Context) ([]VPC, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointVPCs, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVPCs", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[VPC]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetVPC retrieves a single VPC by its ID.
func (c *Client) httpGetVPC(ctx context.Context, vpcID int) (*VPC, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d", vpcID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetVPC", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var vpc VPC
	if err := c.handleResponse(resp, &vpc); err != nil {
		return nil, err
	}

	return &vpc, nil
}

// CreateVPC creates a new VPC.
func (c *Client) httpCreateVPC(ctx context.Context, req CreateVPCRequest) (*VPC, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointVPCs, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateVPC", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var vpc VPC
	if err := c.handleResponse(resp, &vpc); err != nil {
		return nil, err
	}

	return &vpc, nil
}

// UpdateVPC updates an existing VPC.
func (c *Client) httpUpdateVPC(ctx context.Context, vpcID int, req UpdateVPCRequest) (*VPC, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d", vpcID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateVPC", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var vpc VPC
	if err := c.handleResponse(resp, &vpc); err != nil {
		return nil, err
	}

	return &vpc, nil
}

// DeleteVPC deletes a VPC.
func (c *Client) httpDeleteVPC(ctx context.Context, vpcID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d", vpcID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteVPC", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ListVPCIPs retrieves all IP addresses across all VPCs.
func (c *Client) httpListVPCIPs(ctx context.Context) ([]VPCIP, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointVPCs+"/ips", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVPCIPs", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[VPCIP]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListVPCIPAddresses retrieves all IP addresses for a specific VPC.
func (c *Client) httpListVPCIPAddresses(ctx context.Context, vpcID int) ([]VPCIP, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d/ips", vpcID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVPCIPAddresses", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[VPCIP]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListVPCSubnets retrieves all subnets for a VPC.
func (c *Client) httpListVPCSubnets(ctx context.Context, vpcID int) ([]VPCSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d/subnets", vpcID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVPCSubnets", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[VPCSubnet]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetVPCSubnet retrieves a single subnet by its ID within a VPC.
func (c *Client) httpGetVPCSubnet(ctx context.Context, vpcID, subnetID int) (*VPCSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d/subnets/%d", vpcID, subnetID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetVPCSubnet", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var subnet VPCSubnet
	if err := c.handleResponse(resp, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

// CreateVPCSubnet creates a new subnet within a VPC.
func (c *Client) httpCreateVPCSubnet(ctx context.Context, vpcID int, req CreateSubnetRequest) (*VPCSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d/subnets", vpcID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateVPCSubnet", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var subnet VPCSubnet
	if err := c.handleResponse(resp, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

// UpdateVPCSubnet updates an existing subnet within a VPC.
func (c *Client) httpUpdateVPCSubnet(ctx context.Context, vpcID, subnetID int, req UpdateSubnetRequest) (*VPCSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d/subnets/%d", vpcID, subnetID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateVPCSubnet", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var subnet VPCSubnet
	if err := c.handleResponse(resp, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

// DeleteVPCSubnet deletes a subnet from a VPC.
func (c *Client) httpDeleteVPCSubnet(ctx context.Context, vpcID, subnetID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVPCs+"/%d/subnets/%d", vpcID, subnetID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteVPCSubnet", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

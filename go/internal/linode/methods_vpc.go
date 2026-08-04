package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpListVPCsProto retrieves one page of VPCs.
func (c *Client) httpListVPCsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Vpc, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListVPCs",
		"linode_vpc_list", "", nil, page, pageSize,
		func() *linodev1.Vpc { return &linodev1.Vpc{} })
}

// GetVPC retrieves a single VPC by its ID.
func (c *Client) httpGetVPC(ctx context.Context, vpcID int) (*VPC, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_get", nil, vpcID)
	if err != nil {
		return nil, wrapRequestError("GetVPC", err)
	}

	defer drainClose(resp)

	var vpc VPC
	if err := c.handleResponse(resp, &vpc); err != nil {
		return nil, err
	}

	return &vpc, nil
}

// httpGetVPCProto retrieves a VPC and decodes it as a proto message.
func (c *Client) httpGetVPCProto(ctx context.Context, vpcID int) (*linodev1.Vpc, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_get", nil, vpcID)
	if err != nil {
		return nil, wrapRequestError("GetVPC", err)
	}

	defer drainClose(resp)

	vpc := &linodev1.Vpc{}
	if err := c.handleProtoResponse(resp, vpc); err != nil {
		return nil, err
	}

	return vpc, nil
}

// httpCreateVPCProto creates a VPC and decodes it as a proto message.
func (c *Client) httpCreateVPCProto(ctx context.Context, req CreateVPCRequest) (*linodev1.Vpc, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateVPC", err)
	}

	defer drainClose(resp)

	vpc := &linodev1.Vpc{}
	if err := c.handleProtoResponse(resp, vpc); err != nil {
		return nil, err
	}

	return vpc, nil
}

// httpUpdateVPCProto updates a VPC and decodes it as a proto message.
func (c *Client) httpUpdateVPCProto(ctx context.Context, vpcID int, req UpdateVPCRequest) (*linodev1.Vpc, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_update", req, vpcID)
	if err != nil {
		return nil, wrapRequestError("UpdateVPC", err)
	}

	defer drainClose(resp)

	vpc := &linodev1.Vpc{}
	if err := c.handleProtoResponse(resp, vpc); err != nil {
		return nil, err
	}

	return vpc, nil
}

// DeleteVPC deletes a VPC.
func (c *Client) httpDeleteVPC(ctx context.Context, vpcID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_delete", nil, vpcID)
	if err != nil {
		return wrapRequestError("DeleteVPC", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListVPCIPsProto retrieves one page of IP addresses across every VPC,
// unlike httpListVPCIPAddressesProto, which scopes to a single VPC.
func (c *Client) httpListVPCIPsProto(ctx context.Context, page, pageSize int) ([]*linodev1.VPCIP, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListVPCIPs",
		"linode_vpc_ip_all_list", "", nil, page, pageSize,
		func() *linodev1.VPCIP { return &linodev1.VPCIP{} })
}

// httpListVPCIPAddressesProto retrieves one page of a single VPC's IP addresses.
func (c *Client) httpListVPCIPAddressesProto(ctx context.Context, vpcID, page, pageSize int) ([]*linodev1.VPCIP, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListVPCIPAddresses",
		"linode_vpc_ip_list", "", []any{vpcID}, page, pageSize,
		func() *linodev1.VPCIP { return &linodev1.VPCIP{} })
}

// ListVPCSubnets retrieves all subnets for a VPC.
func (c *Client) httpListVPCSubnets(ctx context.Context, vpcID int) ([]VPCSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_subnet_list", nil, vpcID)
	if err != nil {
		return nil, wrapRequestError("ListVPCSubnets", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[VPCSubnet]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListVPCSubnetsProto retrieves one page of a VPC's subnets. It names the
// same tool as httpListVPCSubnets, so both resolve the one declared route.
func (c *Client) httpListVPCSubnetsProto(ctx context.Context, vpcID, page, pageSize int) ([]*linodev1.VpcSubnet, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListVPCSubnets",
		"linode_vpc_subnet_list", "", []any{vpcID}, page, pageSize,
		func() *linodev1.VpcSubnet { return &linodev1.VpcSubnet{} })
}

// GetVPCSubnet retrieves a single subnet by its ID within a VPC.
func (c *Client) httpGetVPCSubnet(ctx context.Context, vpcID, subnetID int) (*VPCSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_subnet_get", nil, vpcID, subnetID)
	if err != nil {
		return nil, wrapRequestError("GetVPCSubnet", err)
	}

	defer drainClose(resp)

	var subnet VPCSubnet
	if err := c.handleResponse(resp, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

// httpGetVPCSubnetProto retrieves a subnet as a proto message.
func (c *Client) httpGetVPCSubnetProto(ctx context.Context, vpcID, subnetID int) (*linodev1.VpcSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_subnet_get", nil, vpcID, subnetID)
	if err != nil {
		return nil, wrapRequestError("GetVPCSubnet", err)
	}

	defer drainClose(resp)

	subnet := &linodev1.VpcSubnet{}
	if err := c.handleProtoResponse(resp, subnet); err != nil {
		return nil, err
	}

	return subnet, nil
}

// httpCreateVPCSubnetProto creates a subnet as a proto message.
func (c *Client) httpCreateVPCSubnetProto(ctx context.Context, vpcID int, req CreateSubnetRequest) (*linodev1.VpcSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_subnet_create", req, vpcID)
	if err != nil {
		return nil, wrapRequestError("CreateVPCSubnet", err)
	}

	defer drainClose(resp)

	subnet := &linodev1.VpcSubnet{}
	if err := c.handleProtoResponse(resp, subnet); err != nil {
		return nil, err
	}

	return subnet, nil
}

// httpUpdateVPCSubnetProto updates a subnet as a proto message.
func (c *Client) httpUpdateVPCSubnetProto(ctx context.Context, vpcID, subnetID int, req UpdateSubnetRequest) (*linodev1.VpcSubnet, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_subnet_update", req, vpcID, subnetID)
	if err != nil {
		return nil, wrapRequestError("UpdateVPCSubnet", err)
	}

	defer drainClose(resp)

	subnet := &linodev1.VpcSubnet{}
	if err := c.handleProtoResponse(resp, subnet); err != nil {
		return nil, err
	}

	return subnet, nil
}

// DeleteVPCSubnet deletes a subnet from a VPC.
func (c *Client) httpDeleteVPCSubnet(ctx context.Context, vpcID, subnetID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_vpc_subnet_delete", nil, vpcID, subnetID)
	if err != nil {
		return wrapRequestError("DeleteVPCSubnet", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

package linode

import (
	"context"
)

// httpGetVPC retrieves a single VPC by its ID.
func (c *Client) httpGetVPC(ctx context.Context, vpcID int) (*VPC, error) {
	return routedGet[VPC](ctx, c, "GetVPC", "linode_vpc_get", vpcID)
}

// httpListVPCSubnets retrieves all subnets for a VPC.
func (c *Client) httpListVPCSubnets(ctx context.Context, vpcID int) ([]VPCSubnet, error) {
	return listData(routedGet[PaginatedResponse[VPCSubnet]](ctx, c, "ListVPCSubnets", "linode_vpc_subnet_list", vpcID))
}

// httpGetVPCSubnet retrieves a single subnet by its ID within a VPC.
func (c *Client) httpGetVPCSubnet(ctx context.Context, vpcID, subnetID int) (*VPCSubnet, error) {
	return routedGet[VPCSubnet](ctx, c, "GetVPCSubnet", "linode_vpc_subnet_get", vpcID, subnetID)
}

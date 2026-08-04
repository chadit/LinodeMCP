package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and the operation each one reports under. Every string is
// named because a method name repeats across this package's tables, and a
// plain method and its proto variant share one operation.
const (
	opCreateVPC            = "CreateVPC"
	opCreateVPCProto       = "CreateVPCProto"
	opCreateVPCSubnet      = "CreateVPCSubnet"
	opCreateVPCSubnetProto = "CreateVPCSubnetProto"
	opDeleteVPC            = "DeleteVPC"
	opDeleteVPCSubnet      = "DeleteVPCSubnet"
	opGetVPC               = "GetVPC"
	opGetVPCProto          = "GetVPCProto"
	opGetVPCSubnet         = "GetVPCSubnet"
	opGetVPCSubnetProto    = "GetVPCSubnetProto"
	opListVPCSubnets       = "ListVPCSubnets"
	opUpdateVPC            = "UpdateVPC"
	opUpdateVPCProto       = "UpdateVPCProto"
	opUpdateVPCSubnet      = "UpdateVPCSubnet"
	opUpdateVPCSubnetProto = "UpdateVPCSubnetProto"
)

// TestRoutedTransportVpc checks that each vpc method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportVpc(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCreateVPCProto,
			operation: opCreateVPC,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateVPCProto(ctx, linode.CreateVPCRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateVPCSubnetProto,
			operation: opCreateVPCSubnet,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateVPCSubnetProto(ctx, 4242, linode.CreateSubnetRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteVPC,
			operation: opDeleteVPC,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteVPC(ctx, 4242)
			},
		},
		{
			name:      opDeleteVPCSubnet,
			operation: opDeleteVPCSubnet,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteVPCSubnet(ctx, 4242, 4242)
			},
		},
		{
			name:      opGetVPC,
			operation: opGetVPC,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVPC(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVPCProto,
			operation: opGetVPC,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVPCProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVPCSubnet,
			operation: opGetVPCSubnet,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVPCSubnet(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVPCSubnetProto,
			operation: opGetVPCSubnet,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVPCSubnetProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListVPCSubnets,
			operation: opListVPCSubnets,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListVPCSubnets(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateVPCProto,
			operation: opUpdateVPC,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateVPCProto(ctx, 4242, linode.UpdateVPCRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateVPCSubnetProto,
			operation: opUpdateVPCSubnet,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateVPCSubnetProto(ctx, 4242, 4242, linode.UpdateSubnetRequest{})

				return clientRouteError(err)
			},
		},
	})
}

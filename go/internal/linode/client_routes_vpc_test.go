package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesVPCPart1 pins the VPC client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesVPCPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CreateVPCProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathVpcs,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateVPCProto(ctx, linode.CreateVPCRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateVPCSubnetProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathVpcs4242Subnets,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateVPCSubnetProto(ctx, 4242, linode.CreateSubnetRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteVPC",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathVpcs4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteVPC(ctx, 4242))
			},
		},
		{
			name:     "DeleteVPCSubnet",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathVpcs4242Subnets8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteVPCSubnet(ctx, 4242, 8615))
			},
		},
		{
			name:     "GetVPC",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVPC(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetVPCProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVPCProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetVPCSubnet",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs4242Subnets8615,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVPCSubnet(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetVPCSubnetProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs4242Subnets8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVPCSubnetProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListVPCIPAddressesProto",
			wantVerb: http.MethodGet,
			wantPath: "/vpcs/4242/ips",
			response: clientRouteProtoPageRegion,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVPCIPAddressesProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.VPCIP).GetRegion) })
			},
		},
		{
			name:     "ListVPCIPsProto",
			wantVerb: http.MethodGet,
			wantPath: "/vpcs/ips",
			response: clientRouteProtoPageRegion,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVPCIPsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.VPCIP).GetRegion) })
			},
		},
		{
			name:     "ListVPCSubnets",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs4242Subnets,
			response: clientRoutePageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVPCSubnets(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, func(item linode.VPCSubnet) string { return item.Label }) })
			},
		},
		{
			name:     "ListVPCSubnetsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs4242Subnets,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVPCSubnetsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.VpcSubnet).GetLabel) })
			},
		},
	})
}

// TestClientRoutesVPCPart2 pins the VPC client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesVPCPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListVPCsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVpcs,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVPCsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Vpc).GetLabel) })
			},
		},
		{
			name:     "UpdateVPCProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathVpcs4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateVPCProto(ctx, 4242, linode.UpdateVPCRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateVPCSubnetProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathVpcs4242Subnets8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateVPCSubnetProto(ctx, 4242, 8615, linode.UpdateSubnetRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

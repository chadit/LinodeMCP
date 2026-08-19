package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesVPCPart1 pins the VPC client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesVPCPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
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
	})
}

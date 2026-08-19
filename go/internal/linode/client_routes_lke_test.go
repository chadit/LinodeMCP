package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesLKEPart1 pins the LKE client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesLKEPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     labelGetLKECluster,
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKECluster(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     labelGetLKEControlPlaneACL,
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242ControlPlaneACL,
			response: clientRouteACLEnvelope,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEControlPlaneACL(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Enabled })
			},
		},
	})
}

// TestClientRoutesLKEPart2 pins the LKE client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesLKEPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     labelGetLKENode,
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242NodesAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKENode(ctx, 4242, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     labelGetLKENodePool,
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242Pools8615,
			response: clientRouteObjType,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKENodePool(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Type })
			},
		},
		{
			name:     "ListLKENodePools",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242Pools,
			response: clientRoutePageType,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKENodePools(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, func(item linode.LKENodePool) string { return item.Type }) })
			},
		},
	})
}

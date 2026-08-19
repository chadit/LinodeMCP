package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Operation labels the cases below share with another table in this
// package. Naming each one once keeps the same string from being spelled
// in three places, which is what a plain method and its proto variant
// reporting under one label looks like.
const (
	labelGetLKECluster         = "GetLKECluster"
	labelGetLKEControlPlaneACL = "GetLKEControlPlaneACL"
	labelGetLKENode            = "GetLKENode"
	labelGetLKENodePool        = "GetLKENodePool"
)

// TestRoutedTransportLKE checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportLKE(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: labelGetLKECluster,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKECluster(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetLKEControlPlaneACL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKEControlPlaneACL(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetLKENode,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKENode(ctx, 4242, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetLKENodePool,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKENodePool(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "ListLKENodePools",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListLKENodePools(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

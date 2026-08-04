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
	labelCreateLKEClusterProto  = "CreateLKEClusterProto"
	labelCreateLKENodePoolProto = "CreateLKENodePoolProto"
	labelGetLKECluster          = "GetLKECluster"
	labelGetLKEControlPlaneACL  = "GetLKEControlPlaneACL"
	labelGetLKENode             = "GetLKENode"
	labelGetLKENodePool         = "GetLKENodePool"
)

// TestRoutedTransportLKE checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportLKE(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      labelCreateLKEClusterProto,
			operation: "CreateLKECluster",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateLKEClusterProto(ctx, &linode.CreateLKEClusterRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      labelCreateLKENodePoolProto,
			operation: "CreateLKENodePool",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateLKENodePoolProto(ctx, 4242, &linode.CreateLKENodePoolRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "DeleteLKECluster",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLKECluster(ctx, 4242))
			},
		},
		{
			name: "DeleteLKEControlPlaneACL",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLKEControlPlaneACL(ctx, 4242))
			},
		},
		{
			name: "DeleteLKEKubeconfig",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLKEKubeconfig(ctx, 4242))
			},
		},
		{
			name: "DeleteLKENode",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLKENode(ctx, 4242, "alpha"))
			},
		},
		{
			name: "DeleteLKENodePool",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLKENodePool(ctx, 4242, 4242))
			},
		},
		{
			name: "DeleteLKEServiceToken",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLKEServiceToken(ctx, 4242))
			},
		},
		{
			name: labelGetLKECluster,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKECluster(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetLKEClusterProto",
			operation: labelGetLKECluster,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKEClusterProto(ctx, 4242)

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
			name:      "GetLKEControlPlaneACLProto",
			operation: labelGetLKEControlPlaneACL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKEControlPlaneACLProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetLKEDashboardProto",
			operation: "GetLKEDashboard",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKEDashboardProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetLKEKubeconfigProto",
			operation: "GetLKEKubeconfig",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKEKubeconfigProto(ctx, 4242)

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
			name:      "GetLKENodePoolProto",
			operation: labelGetLKENodePool,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKENodePoolProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetLKENodeProto",
			operation: labelGetLKENode,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKENodeProto(ctx, 4242, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetLKETierVersionProto",
			operation: "GetLKETierVersion",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKETierVersionProto(ctx, "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetLKEVersionProto",
			operation: "GetLKEVersion",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLKEVersionProto(ctx, "alpha")

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
		{
			name: "RecycleLKECluster",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RecycleLKECluster(ctx, 4242))
			},
		},
		{
			name: "RecycleLKENode",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RecycleLKENode(ctx, 4242, "alpha"))
			},
		},
		{
			name: "RecycleLKENodePool",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RecycleLKENodePool(ctx, 4242, 4242))
			},
		},
		{
			name: "RegenerateLKECluster",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RegenerateLKECluster(ctx, 4242, linode.RegenerateLKEClusterRequest{ServiceToken: true}))
			},
		},
		{
			name:      "UpdateLKEClusterProto",
			operation: "UpdateLKECluster",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateLKEClusterProto(ctx, 4242, linode.UpdateLKEClusterRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateLKEControlPlaneACLProto",
			operation: "UpdateLKEControlPlaneACL",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateLKEControlPlaneACLProto(ctx, 4242, linode.UpdateLKEControlPlaneACLRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateLKENodePoolProto",
			operation: "UpdateLKENodePool",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateLKENodePoolProto(ctx, 4242, 4242, &linode.UpdateLKENodePoolRequest{})

				return clientRouteError(err)
			},
		},
	})
}

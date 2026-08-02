package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesLKEPart1 pins the LKE client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesLKEPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CreateLKEClusterProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLkeClusters,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateLKEClusterProto(ctx, &linode.CreateLKEClusterRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateLKENodePoolProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLkeClusters4242Pools,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateLKENodePoolProto(ctx, 4242, &linode.CreateLKENodePoolRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteLKECluster",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLkeClusters4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLKECluster(ctx, 4242))
			},
		},
		{
			name:     "DeleteLKEControlPlaneACL",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLkeClusters4242ControlPlaneACL,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLKEControlPlaneACL(ctx, 4242))
			},
		},
		{
			name:     "DeleteLKEKubeconfig",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLkeClusters4242Kubeconfig,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLKEKubeconfig(ctx, 4242))
			},
		},
		{
			name:     "DeleteLKENode",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLkeClusters4242NodesAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLKENode(ctx, 4242, "alpha"))
			},
		},
		{
			name:     "DeleteLKENodePool",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLkeClusters4242Pools8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLKENodePool(ctx, 4242, 8615))
			},
		},
		{
			name:     "DeleteLKEServiceToken",
			wantVerb: http.MethodDelete,
			wantPath: "/lke/clusters/4242/servicetoken",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLKEServiceToken(ctx, 4242))
			},
		},
		{
			name:     "GetLKECluster",
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
			name:     "GetLKEClusterProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEClusterProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetLKEControlPlaneACL",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242ControlPlaneACL,
			response: clientRouteACLEnvelope,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEControlPlaneACL(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Enabled })
			},
		},
		{
			name:     "GetLKEControlPlaneACLProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242ControlPlaneACL,
			response: clientRouteACLEnvelope,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEControlPlaneACLProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetEnabled() })
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
			name:     "GetLKEDashboardProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/clusters/4242/dashboard",
			response: clientRouteProtoObjURL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEDashboardProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetUrl() })
			},
		},
		{
			name:     "GetLKEKubeconfigProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242Kubeconfig,
			response: clientRouteProtoObjKubeconfig,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEKubeconfigProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetKubeconfig() })
			},
		},
		{
			name:     "GetLKENode",
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
			name:     "GetLKENodePool",
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
			name:     "GetLKENodePoolProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242Pools8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKENodePoolProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetLKENodeProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242NodesAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKENodeProto(ctx, 4242, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetLKETierVersionProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/tiers/alpha/versions/bravo",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKETierVersionProto(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetLKEVersionProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/versions/alpha",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLKEVersionProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListLKEAPIEndpointsProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/clusters/4242/api-endpoints",
			response: clientRouteProtoPageEndpoint,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKEAPIEndpointsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LKEAPIEndpoint).GetEndpoint) })
			},
		},
		{
			name:     "ListLKEClustersProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKEClustersProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LKECluster).GetLabel) })
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
		{
			name:     "ListLKENodePoolsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLkeClusters4242Pools,
			response: clientRouteProtoPageType,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKENodePoolsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LKENodePool).GetType) })
			},
		},
	})
}

// TestClientRoutesLKEPart3 pins the LKE client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesLKEPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListLKETierVersionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/tiers/alpha/versions",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKETierVersionsProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LKETierVersion).GetId) })
			},
		},
		{
			name:     "ListLKETypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKETypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LinodeType).GetId) })
			},
		},
		{
			name:     "ListLKEVersionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/lke/versions",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLKEVersionsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LKEVersion).GetId) })
			},
		},
		{
			name:     "RecycleLKECluster",
			wantVerb: http.MethodPost,
			wantPath: "/lke/clusters/4242/recycle",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RecycleLKECluster(ctx, 4242))
			},
		},
		{
			name:     "RecycleLKENode",
			wantVerb: http.MethodPost,
			wantPath: "/lke/clusters/4242/nodes/alpha/recycle",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RecycleLKENode(ctx, 4242, "alpha"))
			},
		},
		{
			name:     "RecycleLKENodePool",
			wantVerb: http.MethodPost,
			wantPath: "/lke/clusters/4242/pools/8615/recycle",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RecycleLKENodePool(ctx, 4242, 8615))
			},
		},
		{
			name:     "RegenerateLKECluster",
			wantVerb: http.MethodPost,
			wantPath: "/lke/clusters/4242/regenerate",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RegenerateLKECluster(ctx, 4242))
			},
		},
		{
			name:     "UpdateLKEClusterProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLkeClusters4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateLKEClusterProto(ctx, 4242, linode.UpdateLKEClusterRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateLKEControlPlaneACLProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLkeClusters4242ControlPlaneACL,
			response: clientRouteACLEnvelope,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateLKEControlPlaneACLProto(ctx, 4242, linode.UpdateLKEControlPlaneACLRequest{})

				return clientRouteProbe(err, func() any { return got.GetEnabled() })
			},
		},
		{
			name:     "UpdateLKENodePoolProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLkeClusters4242Pools8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateLKENodePoolProto(ctx, 4242, 8615, linode.UpdateLKENodePoolRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

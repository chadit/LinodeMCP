package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesLongview pins the Longview client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesLongview(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetLongviewClient",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLongviewClientsAlpha,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewClient(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetLongviewClientProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLongviewClientsAlpha,
			response: clientRouteProtoObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewClientProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetCreated() })
			},
		},
		{
			name:     "GetLongviewPlan",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLongviewPlan,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewPlan(ctx)

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     "GetLongviewPlanProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLongviewPlan,
			response: clientRouteProtoObjClientsIncludedInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewPlanProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetClientsIncluded() })
			},
		},
		{
			name:     "GetLongviewSubscriptionProto",
			wantVerb: http.MethodGet,
			wantPath: "/longview/subscriptions/alpha",
			response: clientRouteProtoObjClientsIncludedInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewSubscriptionProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetClientsIncluded() })
			},
		},
		{
			name:     "ListLongviewSubscriptionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/longview/subscriptions",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLongviewSubscriptionsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LongviewSubscription).GetId) })
			},
		},
		{
			name:     "ListLongviewTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/longview/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLongviewTypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LongviewType).GetId) })
			},
		},
	})
}

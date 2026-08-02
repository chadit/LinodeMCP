package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesPlacement pins the placement group client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesPlacement(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AssignPlacementGroupLinodesProto",
			wantVerb: http.MethodPost,
			wantPath: "/placement/groups/4242/assign",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AssignPlacementGroupLinodesProto(ctx, 4242, &linode.AssignPlacementGroupLinodesRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreatePlacementGroupProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathPlacementGroups,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreatePlacementGroupProto(ctx, &linode.CreatePlacementGroupRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeletePlacementGroup",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathPlacementGroups4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeletePlacementGroup(ctx, 4242))
			},
		},
		{
			name:     "GetPlacementGroup",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathPlacementGroups4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetPlacementGroup(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetPlacementGroupProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathPlacementGroups4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetPlacementGroupProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListPlacementGroupsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathPlacementGroups,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListPlacementGroupsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.PlacementGroup).GetLabel) })
			},
		},
		{
			name:     "UnassignPlacementGroupProto",
			wantVerb: http.MethodPost,
			wantPath: "/placement/groups/4242/unassign",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UnassignPlacementGroupProto(ctx, 4242, &linode.PlacementGroupUnassignRequest{Linodes: []int{4242}})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdatePlacementGroupProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathPlacementGroups4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdatePlacementGroupProto(ctx, 4242, &linode.UpdatePlacementGroupRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

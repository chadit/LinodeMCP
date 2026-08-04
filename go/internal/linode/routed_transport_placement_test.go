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
	opAssignPlacementGroupLinodes      = "AssignPlacementGroupLinodes"
	opAssignPlacementGroupLinodesProto = "AssignPlacementGroupLinodesProto"
	opCreatePlacementGroup             = "CreatePlacementGroup"
	opCreatePlacementGroupProto        = "CreatePlacementGroupProto"
	opDeletePlacementGroup             = "DeletePlacementGroup"
	opGetPlacementGroup                = "GetPlacementGroup"
	opGetPlacementGroupProto           = "GetPlacementGroupProto"
	opUnassignPlacementGroup           = "UnassignPlacementGroup"
	opUnassignPlacementGroupProto      = "UnassignPlacementGroupProto"
	opUpdatePlacementGroup             = "UpdatePlacementGroup"
	opUpdatePlacementGroupProto        = "UpdatePlacementGroupProto"
)

// TestRoutedTransportPlacement checks that each placement method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportPlacement(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opAssignPlacementGroupLinodesProto,
			operation: opAssignPlacementGroupLinodes,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AssignPlacementGroupLinodesProto(ctx, 4242, &linode.AssignPlacementGroupLinodesRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreatePlacementGroupProto,
			operation: opCreatePlacementGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreatePlacementGroupProto(ctx, &linode.CreatePlacementGroupRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeletePlacementGroup,
			operation: opDeletePlacementGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeletePlacementGroup(ctx, 4242)
			},
		},
		{
			name:      opGetPlacementGroup,
			operation: opGetPlacementGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetPlacementGroup(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetPlacementGroupProto,
			operation: opGetPlacementGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetPlacementGroupProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opUnassignPlacementGroupProto,
			operation: opUnassignPlacementGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UnassignPlacementGroupProto(ctx, 4242,
					&linode.PlacementGroupUnassignRequest{Linodes: []int{4242}})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdatePlacementGroupProto,
			operation: opUpdatePlacementGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdatePlacementGroupProto(ctx, 4242, &linode.UpdatePlacementGroupRequest{})

				return clientRouteError(err)
			},
		},
	})
}

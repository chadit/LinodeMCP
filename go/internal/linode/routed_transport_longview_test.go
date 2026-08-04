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
	opCreateLongviewClient         = "CreateLongviewClient"
	opCreateLongviewClientProto    = "CreateLongviewClientProto"
	opGetLongviewClient            = "GetLongviewClient"
	opGetLongviewClientProto       = "GetLongviewClientProto"
	opGetLongviewPlan              = "GetLongviewPlan"
	opGetLongviewPlanProto         = "GetLongviewPlanProto"
	opGetLongviewSubscription      = "GetLongviewSubscription"
	opGetLongviewSubscriptionProto = "GetLongviewSubscriptionProto"
	opUpdateLongviewPlan           = "UpdateLongviewPlan"
	opUpdateLongviewPlanProto      = "UpdateLongviewPlanProto"
)

// TestRoutedTransportLongview checks that each longview method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportLongview(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCreateLongviewClientProto,
			operation: opCreateLongviewClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateLongviewClientProto(ctx, &linode.CreateLongviewClientRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opGetLongviewClient,
			operation: opGetLongviewClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLongviewClient(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetLongviewClientProto,
			operation: opGetLongviewClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLongviewClientProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetLongviewPlan,
			operation: opGetLongviewPlan,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLongviewPlan(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetLongviewPlanProto,
			operation: opGetLongviewPlan,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLongviewPlanProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetLongviewSubscriptionProto,
			operation: opGetLongviewSubscription,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLongviewSubscriptionProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateLongviewPlanProto,
			operation: opUpdateLongviewPlan,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateLongviewPlanProto(ctx, &linode.UpdateLongviewPlanRequest{})

				return clientRouteError(err)
			},
		},
	})
}

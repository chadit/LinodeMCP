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
	opGetLongviewClient = "GetLongviewClient"
	opGetLongviewPlan   = "GetLongviewPlan"
)

// TestRoutedTransportLongview checks that each longview method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportLongview(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetLongviewClient,
			operation: opGetLongviewClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetLongviewClient(ctx, "alpha")

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
	})
}

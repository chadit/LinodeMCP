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
	opGetMonitorServiceAlertDefinition = "GetMonitorServiceAlertDefinition"
)

// TestRoutedTransportMonitor checks that each monitor method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportMonitor(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetMonitorServiceAlertDefinition,
			operation: opGetMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetMonitorServiceAlertDefinition(ctx, "alpha", 4242)

				return clientRouteError(err)
			},
		},
	})
}

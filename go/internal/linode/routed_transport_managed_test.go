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
	opGetManagedContact        = "GetManagedContact"
	opGetManagedLinodeSettings = "GetManagedLinodeSettings"
	opGetManagedService        = "GetManagedService"
)

// TestRoutedTransportManaged checks that each managed method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportManaged(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetManagedContact,
			operation: opGetManagedContact,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedContact(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetManagedLinodeSettings,
			operation: opGetManagedLinodeSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedLinodeSettings(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetManagedService,
			operation: opGetManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedService(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

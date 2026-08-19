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
	opGetSSHKey = "GetSSHKey"
	opGetVolume = "GetVolume"
)

// TestRoutedTransportStorage checks that each storage method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportStorage(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetSSHKey,
			operation: opGetSSHKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetSSHKey(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVolume,
			operation: opGetVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVolume(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

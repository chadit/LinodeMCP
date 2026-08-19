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
	opGetIPv6Range = "GetIPv6Range"
)

// TestRoutedTransportNetworkingIpv6Ranges checks that each networking ipv6 ranges method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportNetworkingIpv6Ranges(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetIPv6Range,
			operation: opGetIPv6Range,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetIPv6Range(ctx, testIPv6Range)

				return clientRouteError(err)
			},
		},
	})
}

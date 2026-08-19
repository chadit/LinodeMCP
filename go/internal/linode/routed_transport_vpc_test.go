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
	opGetVPC         = "GetVPC"
	opGetVPCSubnet   = "GetVPCSubnet"
	opListVPCSubnets = "ListVPCSubnets"
)

// TestRoutedTransportVpc checks that each vpc method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportVpc(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetVPC,
			operation: opGetVPC,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVPC(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVPCSubnet,
			operation: opGetVPCSubnet,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVPCSubnet(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListVPCSubnets,
			operation: opListVPCSubnets,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListVPCSubnets(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

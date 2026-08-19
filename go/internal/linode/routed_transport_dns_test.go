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
	opGetDomain         = "GetDomain"
	opGetDomainRecord   = "GetDomainRecord"
	opListDomainRecords = "ListDomainRecords"
)

// TestRoutedTransportDns checks that each dns method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportDns(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetDomain,
			operation: opGetDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomain(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetDomainRecord,
			operation: opGetDomainRecord,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomainRecord(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListDomainRecords,
			operation: opListDomainRecords,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListDomainRecords(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

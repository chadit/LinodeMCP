package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Operation labels the cases below share with another table in this
// package. Naming each one once keeps the same string from being spelled
// in three places, which is what a plain method and its proto variant
// reporting under one label looks like.
const (
	labelGetDatabaseInstance           = "GetDatabaseInstance"
	labelGetDatabasePostgreSQLInstance = "GetDatabasePostgreSQLInstance"
)

// TestRoutedTransportDatabases checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportDatabases(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: labelGetDatabaseInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseInstance(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetDatabasePostgreSQLInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabasePostgreSQLInstance(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

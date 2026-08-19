package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesDatabasesPart1 pins the managed database client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDatabasesPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetDatabaseInstance",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDatabasesMysqlInstances4242,
			response: clientRouteObjVersion,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseInstance(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Version })
			},
		},
		{
			name:     "GetDatabasePostgreSQLInstance",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDatabasesPostgresqlInstances4242,
			response: clientRouteObjVersion,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabasePostgreSQLInstance(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Version })
			},
		},
	})
}

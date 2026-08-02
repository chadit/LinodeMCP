package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesDatabasesPart1 pins the managed database client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDatabasesPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CreateDatabaseInstanceProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathDatabasesMysqlInstances,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateDatabaseInstanceProto(ctx, &linode.CreateDatabaseInstanceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteDatabaseInstance",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathDatabasesMysqlInstances4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteDatabaseInstance(ctx, 4242))
			},
		},
		{
			name:     "DeleteDatabasePostgreSQLInstance",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathDatabasesPostgresqlInstances4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name:     "GetDatabaseEngineProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/engines/alpha",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseEngineProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
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
			name:     "GetDatabaseInstanceCredentials",
			wantVerb: http.MethodGet,
			wantPath: "/databases/mysql/instances/4242/credentials",
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseInstanceCredentials(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "GetDatabaseInstanceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDatabasesMysqlInstances4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseInstanceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetDatabaseInstanceSSLProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/mysql/instances/4242/ssl",
			response: clientRouteProtoObjCaCertificate,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseInstanceSSLProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetCaCertificate() })
			},
		},
		{
			name:     "GetDatabaseMySQLConfig",
			wantVerb: http.MethodGet,
			wantPath: "/databases/mysql/config",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseMySQLConfig(ctx)

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
			},
		},
		{
			name:     "GetDatabasePostgreSQLConfig",
			wantVerb: http.MethodGet,
			wantPath: "/databases/postgresql/config",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabasePostgreSQLConfig(ctx)

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
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
		{
			name:     "GetDatabasePostgreSQLInstanceCredentials",
			wantVerb: http.MethodGet,
			wantPath: "/databases/postgresql/instances/4242/credentials",
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabasePostgreSQLInstanceCredentials(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
	})
}

// TestClientRoutesDatabasesPart2 pins the managed database client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDatabasesPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetDatabasePostgreSQLInstanceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDatabasesPostgresqlInstances4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabasePostgreSQLInstanceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetDatabasePostgreSQLInstanceSSLProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/postgresql/instances/4242/ssl",
			response: clientRouteProtoObjCaCertificate,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabasePostgreSQLInstanceSSLProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetCaCertificate() })
			},
		},
		{
			name:     "GetDatabaseTypeProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/types/alpha",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDatabaseTypeProto(ctx, "alpha", 1, 25)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListAllDatabaseInstancesProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/instances",
			response: clientRouteProtoPageStatus,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAllDatabaseInstancesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.DatabaseInstance).GetStatus) })
			},
		},
		{
			name:     "ListDatabaseEnginesProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/engines",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDatabaseEnginesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.DatabaseEngine).GetId) })
			},
		},
		{
			name:     "ListDatabaseInstancesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDatabasesMysqlInstances,
			response: clientRouteProtoPageStatus,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDatabaseInstancesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.DatabaseInstance).GetStatus) })
			},
		},
		{
			name:     "ListDatabasePostgreSQLInstancesProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/postgresql/instances",
			response: clientRouteProtoPageStatus,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDatabasePostgreSQLInstancesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.DatabaseInstance).GetStatus) })
			},
		},
		{
			name:     "ListDatabaseTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/databases/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDatabaseTypesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.DatabaseType).GetId) })
			},
		},
		{
			name:     "PatchDatabaseInstance",
			wantVerb: http.MethodPost,
			wantPath: "/databases/mysql/instances/4242/patch",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.PatchDatabaseInstance(ctx, 4242))
			},
		},
		{
			name:     "PatchDatabasePostgreSQLInstance",
			wantVerb: http.MethodPost,
			wantPath: "/databases/postgresql/instances/4242/patch",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.PatchDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name:     "ResetDatabaseInstanceCredentials",
			wantVerb: http.MethodPost,
			wantPath: "/databases/mysql/instances/4242/credentials/reset",
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ResetDatabaseInstanceCredentials(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "ResetDatabasePostgreSQLInstanceCredentials",
			wantVerb: http.MethodPost,
			wantPath: "/databases/postgresql/instances/4242/credentials/reset",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResetDatabasePostgreSQLInstanceCredentials(ctx, 4242))
			},
		},
	})
}

// TestClientRoutesDatabasesPart3 pins the managed database client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDatabasesPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ResumeDatabaseInstance",
			wantVerb: http.MethodPost,
			wantPath: "/databases/mysql/instances/4242/resume",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResumeDatabaseInstance(ctx, 4242))
			},
		},
		{
			name:     "ResumeDatabasePostgreSQLInstance",
			wantVerb: http.MethodPost,
			wantPath: "/databases/postgresql/instances/4242/resume",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResumeDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name:     "SuspendDatabaseInstance",
			wantVerb: http.MethodPost,
			wantPath: "/databases/mysql/instances/4242/suspend",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.SuspendDatabaseInstance(ctx, 4242))
			},
		},
		{
			name:     "SuspendDatabasePostgreSQLInstance",
			wantVerb: http.MethodPost,
			wantPath: "/databases/postgresql/instances/4242/suspend",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.SuspendDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
	})
}

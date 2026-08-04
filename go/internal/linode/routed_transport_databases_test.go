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
			name:      "CreateDatabaseInstanceProto",
			operation: "CreateDatabaseInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateDatabaseInstanceProto(ctx, &linode.CreateDatabaseInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateDatabasePostgreSQLInstanceProto",
			operation: "CreateDatabasePostgreSQLInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateDatabasePostgreSQLInstanceProto(ctx, &linode.CreateDatabaseInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "DeleteDatabaseInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteDatabaseInstance(ctx, 4242))
			},
		},
		{
			name: "DeleteDatabasePostgreSQLInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name:      "GetDatabaseEngineProto",
			operation: "GetDatabaseEngine",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseEngineProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetDatabaseInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseInstance(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "GetDatabaseInstanceCredentials",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseInstanceCredentials(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetDatabaseInstanceProto",
			operation: labelGetDatabaseInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseInstanceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetDatabaseInstanceSSLProto",
			operation: "GetDatabaseInstanceSSL",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseInstanceSSLProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "GetDatabaseMySQLConfig",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseMySQLConfig(ctx)

				return clientRouteError(err)
			},
		},
		{
			name: "GetDatabasePostgreSQLConfig",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabasePostgreSQLConfig(ctx)

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
		{
			name: "GetDatabasePostgreSQLInstanceCredentials",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabasePostgreSQLInstanceCredentials(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetDatabasePostgreSQLInstanceProto",
			operation: labelGetDatabasePostgreSQLInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabasePostgreSQLInstanceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetDatabasePostgreSQLInstanceSSLProto",
			operation: "GetDatabasePostgreSQLInstanceSSL",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabasePostgreSQLInstanceSSLProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetDatabaseTypeProto",
			operation: "GetDatabaseType",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDatabaseTypeProto(ctx, "alpha", 1, 100)

				return clientRouteError(err)
			},
		},
		{
			name: "PatchDatabaseInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.PatchDatabaseInstance(ctx, 4242))
			},
		},
		{
			name: "PatchDatabasePostgreSQLInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.PatchDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name: "ResetDatabaseInstanceCredentials",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ResetDatabaseInstanceCredentials(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "ResetDatabasePostgreSQLInstanceCredentials",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ResetDatabasePostgreSQLInstanceCredentials(ctx, 4242))
			},
		},
		{
			name: "ResumeDatabaseInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ResumeDatabaseInstance(ctx, 4242))
			},
		},
		{
			name: "ResumeDatabasePostgreSQLInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ResumeDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name: "SuspendDatabaseInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.SuspendDatabaseInstance(ctx, 4242))
			},
		},
		{
			name: "SuspendDatabasePostgreSQLInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.SuspendDatabasePostgreSQLInstance(ctx, 4242))
			},
		},
		{
			name:      "UpdateDatabaseInstanceProto",
			operation: "UpdateDatabaseInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateDatabaseInstanceProto(ctx, 4242, &linode.UpdateDatabaseInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateDatabasePostgreSQLInstanceProto",
			operation: "UpdateDatabasePostgreSQLInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateDatabasePostgreSQLInstanceProto(ctx, 4242, &linode.UpdateDatabaseInstanceRequest{})

				return clientRouteError(err)
			},
		},
	})
}

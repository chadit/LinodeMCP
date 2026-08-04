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
	opCreateManagedContact             = "CreateManagedContact"
	opCreateManagedContactProto        = "CreateManagedContactProto"
	opCreateManagedService             = "CreateManagedService"
	opCreateManagedServiceProto        = "CreateManagedServiceProto"
	opDeleteManagedContact             = "DeleteManagedContact"
	opDeleteManagedService             = "DeleteManagedService"
	opDisableManagedService            = "DisableManagedService"
	opEnableManagedService             = "EnableManagedService"
	opGetManagedContact                = "GetManagedContact"
	opGetManagedContactProto           = "GetManagedContactProto"
	opGetManagedIssue                  = "GetManagedIssue"
	opGetManagedIssueProto             = "GetManagedIssueProto"
	opGetManagedLinodeSettings         = "GetManagedLinodeSettings"
	opGetManagedLinodeSettingsProto    = "GetManagedLinodeSettingsProto"
	opGetManagedService                = "GetManagedService"
	opGetManagedServiceProto           = "GetManagedServiceProto"
	opGetManagedStats                  = "GetManagedStats"
	opUpdateManagedContact             = "UpdateManagedContact"
	opUpdateManagedContactProto        = "UpdateManagedContactProto"
	opUpdateManagedLinodeSettings      = "UpdateManagedLinodeSettings"
	opUpdateManagedLinodeSettingsProto = "UpdateManagedLinodeSettingsProto"
	opUpdateManagedService             = "UpdateManagedService"
	opUpdateManagedServiceProto        = "UpdateManagedServiceProto"
)

// TestRoutedTransportManaged checks that each managed method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportManaged(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCreateManagedContactProto,
			operation: opCreateManagedContact,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateManagedContactProto(ctx, &linode.CreateManagedContactRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateManagedServiceProto,
			operation: opCreateManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateManagedServiceProto(ctx, &linode.CreateManagedServiceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteManagedContact,
			operation: opDeleteManagedContact,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteManagedContact(ctx, 4242)
			},
		},
		{
			name:      opDeleteManagedService,
			operation: opDeleteManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteManagedService(ctx, 4242)
			},
		},
		{
			name:      opDisableManagedService,
			operation: opDisableManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DisableManagedService(ctx, 4242)
			},
		},
		{
			name:      opEnableManagedService,
			operation: opEnableManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.EnableManagedService(ctx, 4242)
			},
		},
		{
			name:      opGetManagedContact,
			operation: opGetManagedContact,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedContact(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetManagedContactProto,
			operation: opGetManagedContact,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedContactProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetManagedIssueProto,
			operation: opGetManagedIssue,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedIssueProto(ctx, 4242)

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
			name:      opGetManagedLinodeSettingsProto,
			operation: opGetManagedLinodeSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedLinodeSettingsProto(ctx, 4242)

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
		{
			name:      opGetManagedServiceProto,
			operation: opGetManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedServiceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetManagedStats,
			operation: opGetManagedStats,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedStats(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateManagedContactProto,
			operation: opUpdateManagedContact,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateManagedContactProto(ctx, 4242, linode.UpdateManagedContactRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateManagedLinodeSettingsProto,
			operation: opUpdateManagedLinodeSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateManagedLinodeSettingsProto(ctx, 4242, linode.UpdateManagedLinodeSettingsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateManagedServiceProto,
			operation: opUpdateManagedService,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateManagedServiceProto(ctx, 4242, &linode.UpdateManagedServiceRequest{})

				return clientRouteError(err)
			},
		},
	})
}

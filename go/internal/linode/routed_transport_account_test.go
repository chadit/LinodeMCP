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
	labelGetAccount                = "GetAccount"
	labelGetAccountChildAccount    = "GetAccountChildAccount"
	labelGetAccountEvent           = "GetAccountEvent"
	labelGetAccountOAuthClient     = "GetAccountOAuthClient"
	labelGetAccountPaymentMethod   = "GetAccountPaymentMethod"
	labelGetAccountServiceTransfer = "GetAccountServiceTransfer"
	labelGetAccountSettings        = "GetAccountSettings"
	labelGetAccountUser            = "GetAccountUser"
	labelGetManagedCredential      = "GetManagedCredential"
	labelGetProfileApp             = "GetProfileApp"
)

// TestRoutedTransportAccountPart1 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart1(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: labelGetAccount,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccount(ctx)

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportAccountPart2 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart2(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: labelGetAccountChildAccount,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountChildAccount(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountEvent,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountEvent(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountOAuthClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountOAuthClient(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountPaymentMethod,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountPaymentMethod(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountServiceTransfer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountServiceTransfer(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountSettings(ctx)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountUser,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountUser(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: "GetAccountUserGrants",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountUserGrants(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetManagedCredential,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedCredential(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "CallRouteRawBodyRead",
			operation: thumbnailGetTool,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CallRouteRawBodyRead(ctx, thumbnailGetTool, []any{transportClientAlpha}, "image/png")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetProfileAppProto",
			operation: labelGetProfileApp,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileAppProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetProfileDeviceProto",
			operation: "GetProfileDevice",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileDeviceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportAccountPart3 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart3(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      "GetProfileTokenProto",
			operation: "GetProfileToken",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileTokenProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "CallRouteRawBody",
			operation: thumbnailUpdateTool,
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.CallRouteRawBody(
					ctx, thumbnailUpdateTool, []any{transportClientAlpha}, "image/png", []byte("png"),
				))
			},
		},
	})
}

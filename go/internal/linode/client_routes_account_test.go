package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesAccountPart3 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetAccount",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccount,
			response: clientRouteObjZip,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccount(ctx)

				return clientRouteProbe(err, func() any { return got.Zip })
			},
		},
		{
			name:     "GetAccountChildAccount",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountChildAccountsAlpha,
			response: clientRouteObjEuuid,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountChildAccount(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.EUUID })
			},
		},
		{
			name:     "GetAccountEvent",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountEvents4242,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountEvent(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetAccountOAuthClient",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountOauthClientsAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountOAuthClient(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
	})
}

// TestClientRoutesAccountPart4 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart4(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetAccountPaymentMethod",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountPaymentMethodsAlpha,
			response: clientRouteObjType,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountPaymentMethod(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Type })
			},
		},
		{
			name:     "GetAccountServiceTransfer",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountServiceTransfersAlpha,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountServiceTransfer(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetAccountSettings",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountSettings,
			response: clientRouteObjInterfacesForNewLinodes,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountSettings(ctx)

				return clientRouteProbe(err, func() any { return got.InterfacesForNewLinodes })
			},
		},
		{
			name:     "GetAccountUser",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountUsersAlpha,
			response: clientRouteObjEmail,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountUser(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Email })
			},
		},
	})
}

// TestClientRoutesAccountPart5 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart5(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetManagedCredential",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedCredentials4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedCredential(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetProfile",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfile,
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfile(ctx)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "GetProfileAppProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileApps4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileAppProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetProfileDeviceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileDevices4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileDeviceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetProfileTokenProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileTokens4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileTokenProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

// TestClientRoutesAccountPart8 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart8(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CallRouteRawBody",
			wantVerb: http.MethodPut,
			wantPath: "/account/oauth-clients/alpha/thumbnail",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.CallRouteRawBody(
					ctx, thumbnailUpdateTool, []any{transportClientAlpha}, "image/png", []byte("payload"),
				))
			},
		},
	})
}

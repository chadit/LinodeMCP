package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesAccountPart5 pins the JSON read primitive to the request it
// issues on the profile route and the value it decodes back.
func TestClientRoutesAccountPart5(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CallRouteJSON",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfile,
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := readProfile(ctx, client)

				return clientRouteProbe(err, func() any { return got.Username })
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

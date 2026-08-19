package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesLongview pins the Longview client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesLongview(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetLongviewClient",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLongviewClientsAlpha,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewClient(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetLongviewPlan",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLongviewPlan,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetLongviewPlan(ctx)

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
	})
}

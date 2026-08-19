package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesPlacement pins the placement group client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesPlacement(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetPlacementGroup",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathPlacementGroups4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetPlacementGroup(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
	})
}

package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesMonitorPart1 pins the monitor client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesMonitorPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetMonitorServiceAlertDefinition",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetMonitorServiceAlertDefinition(ctx, "alpha", 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
	})
}

package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesManagedPart1 pins the Managed services client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesManagedPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetManagedContact",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedContacts4242,
			response: clientRouteObjName,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedContact(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Name })
			},
		},
		{
			name:     "GetManagedLinodeSettings",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedLinodeSettings4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedLinodeSettings(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetManagedService",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedServices4242,
			response: clientRouteObjStatus,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedService(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Status })
			},
		},
	})
}

package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesStoragePart1 pins the volume and StackScript client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesStoragePart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetSSHKey",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileSshkeys4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetSSHKey(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetVolume",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVolumes4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVolume(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
	})
}

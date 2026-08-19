package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesComputePart2 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetImage",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImage(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     "GetImageShareGroup",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups4242,
			response: clientRouteObjUUID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroup(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.UUID })
			},
		},
		{
			name:     "GetImageShareGroupByToken",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroupsTokensAlphaSharegroup,
			response: clientRouteObjUUID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroupByToken(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.UUID })
			},
		},
		{
			name:     "GetInstance",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242,
			response: clientRouteObjHypervisor,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstance(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Hypervisor })
			},
		},
	})
}

// TestClientRoutesComputePart3 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetStackScript",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeStackscripts4242,
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetStackScript(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "GetType",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeTypesAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetType(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
	})
}

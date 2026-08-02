package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesIPv6Ranges pins the IPv6 range client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesIPv6Ranges(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "DeleteIPv6Range",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNetworkingIpv6Ranges20010db864,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteIPv6Range(ctx, "2001:0db8::/64"))
			},
		},
		{
			name:     "GetIPv6Range",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingIpv6Ranges20010db864,
			response: clientRouteObjRange,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetIPv6Range(ctx, "2001:0db8::/64")

				return clientRouteProbe(err, func() any { return got.Range })
			},
		},
		{
			name:     "GetIPv6RangeProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingIpv6Ranges20010db864,
			response: clientRouteProtoObjRange,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetIPv6RangeProto(ctx, "2001:0db8::/64")

				return clientRouteProbe(err, func() any { return got.GetRange() })
			},
		},
	})
}

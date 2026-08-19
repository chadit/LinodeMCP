package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesDNSPart1 pins the domain client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDNSPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetDomain",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains4242,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDomain(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetDomainRecord",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains4242Records8615,
			response: clientRouteObjProtocol,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDomainRecord(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Protocol })
			},
		},
		{
			name:     "ListDomainRecords",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains4242Records,
			response: clientRoutePageProtocol,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDomainRecords(ctx, 4242)

				return clientRouteProbe(err, func() any {
					return clientRouteList(got, func(item linode.DomainRecord) string { return item.Protocol })
				})
			},
		},
	})
}

package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesDNSPart1 pins the domain client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDNSPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CloneDomainProto",
			wantVerb: http.MethodPost,
			wantPath: "/domains/4242/clone",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CloneDomainProto(ctx, 4242, &linode.CloneDomainRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateDomainProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathDomains,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateDomainProto(ctx, &linode.CreateDomainRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateDomainRecordProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathDomains4242Records,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateDomainRecordProto(ctx, 4242, &linode.CreateDomainRecordRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteDomain",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathDomains4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteDomain(ctx, 4242))
			},
		},
		{
			name:     "DeleteDomainRecord",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathDomains4242Records8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteDomainRecord(ctx, 4242, 8615))
			},
		},
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
			name:     "GetDomainProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDomainProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
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
			name:     "GetDomainRecordProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains4242Records8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetDomainRecordProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ImportDomainProto",
			wantVerb: http.MethodPost,
			wantPath: "/domains/import",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ImportDomainProto(ctx, &linode.ImportDomainRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
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
		{
			name:     "ListDomainRecordsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains4242Records,
			response: clientRouteProtoPageType,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDomainRecordsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.DomainRecord).GetType) })
			},
		},
	})
}

// TestClientRoutesDNSPart2 pins the domain client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesDNSPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListDomainsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathDomains,
			response: clientRouteProtoPageDomain,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListDomainsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Domain).GetDomain) })
			},
		},
		{
			name:     "UpdateDomainProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathDomains4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateDomainProto(ctx, 4242, &linode.UpdateDomainRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateDomainRecordProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathDomains4242Records8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateDomainRecordProto(ctx, 4242, 8615, &linode.UpdateDomainRecordRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

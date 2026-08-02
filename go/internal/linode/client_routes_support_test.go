package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesSupport pins the support ticket client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesSupport(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CloseSupportTicket",
			wantVerb: http.MethodPost,
			wantPath: "/support/tickets/4242/close",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.CloseSupportTicket(ctx, 4242))
			},
		},
		{
			name:     "CreateSupportTicketProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathSupportTickets,
			response: clientRouteProtoObjClosableBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateSupportTicketProto(ctx, &linode.CreateSupportTicketRequest{})

				return clientRouteProbe(err, func() any { return got.GetClosable() })
			},
		},
		{
			name:     "CreateSupportTicketReplyProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathSupportTickets4242Replies,
			response: clientRouteProtoObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateSupportTicketReplyProto(ctx, 4242, &linode.CreateSupportTicketReplyRequest{})

				return clientRouteProbe(err, func() any { return got.GetCreated() })
			},
		},
		{
			name:     "GetSupportTicketProto",
			wantVerb: http.MethodGet,
			wantPath: "/support/tickets/4242",
			response: clientRouteProtoObjClosableBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetSupportTicketProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetClosable() })
			},
		},
		{
			name:     "ListSupportTicketRepliesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathSupportTickets4242Replies,
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListSupportTicketRepliesProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.SupportTicketReply).GetCreated) })
			},
		},
		{
			name:     "ListSupportTicketsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathSupportTickets,
			response: clientRouteProtoPageDescription,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListSupportTicketsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.SupportTicket).GetDescription) })
			},
		},
	})
}

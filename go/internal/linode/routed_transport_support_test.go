package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and the operation each one reports under. Every string is
// named because a method name repeats across this package's tables, and a
// plain method and its proto variant share one operation.
const (
	opCloseSupportTicket            = "CloseSupportTicket"
	opCreateSupportTicket           = "CreateSupportTicket"
	opCreateSupportTicketProto      = "CreateSupportTicketProto"
	opCreateSupportTicketReply      = "CreateSupportTicketReply"
	opCreateSupportTicketReplyProto = "CreateSupportTicketReplyProto"
	opGetSupportTicket              = "GetSupportTicket"
	opGetSupportTicketProto         = "GetSupportTicketProto"
)

// TestRoutedTransportSupport checks that each support method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportSupport(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCloseSupportTicket,
			operation: opCloseSupportTicket,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.CloseSupportTicket(ctx, 4242)
			},
		},
		{
			name:      opCreateSupportTicketProto,
			operation: opCreateSupportTicket,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateSupportTicketProto(ctx, &linode.CreateSupportTicketRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateSupportTicketReplyProto,
			operation: opCreateSupportTicketReply,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateSupportTicketReplyProto(ctx, 4242, &linode.CreateSupportTicketReplyRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opGetSupportTicketProto,
			operation: opGetSupportTicket,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetSupportTicketProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

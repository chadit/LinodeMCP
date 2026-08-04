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
	opCreateTag          = "CreateTag"
	opCreateTagProto     = "CreateTagProto"
	opDeleteTagX         = "DeleteTag"
	opListTaggedObjectsX = "ListTaggedObjects"
)

// TestRoutedTransportTags checks that each tags method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportTags(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCreateTagProto,
			operation: opCreateTag,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateTagProto(ctx, &linode.CreateTagRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteTagX,
			operation: opDeleteTagX,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteTag(ctx, "alpha")
			},
		},
		{
			name:      opListTaggedObjectsX,
			operation: opListTaggedObjectsX,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListTaggedObjects(ctx, "alpha", 4242, 4242)

				return clientRouteError(err)
			},
		},
	})
}

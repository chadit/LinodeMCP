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
	opGetBucketSSL                 = "GetBucketSSL"
	opGetObjectACL                 = "GetObjectACL"
	opGetObjectStorageBucket       = "GetObjectStorageBucket"
	opGetObjectStorageBucketAccess = "GetObjectStorageBucketAccess"
	opGetObjectStorageKey          = "GetObjectStorageKey"
)

// TestRoutedTransportObjectStorage checks that each object storage method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportObjectStorage(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetBucketSSL,
			operation: opGetBucketSSL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetBucketSSL(ctx, "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectACL,
			operation: opGetObjectACL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectACL(ctx, "alpha", "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageBucket,
			operation: opGetObjectStorageBucket,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageBucket(ctx, "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageBucketAccess,
			operation: opGetObjectStorageBucketAccess,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageBucketAccess(ctx, "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageKey,
			operation: opGetObjectStorageKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageKey(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

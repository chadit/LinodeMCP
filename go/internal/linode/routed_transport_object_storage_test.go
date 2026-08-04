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
	opAllowObjectStorageBucketAccess       = "AllowObjectStorageBucketAccess"
	opCancelObjectStorage                  = "CancelObjectStorage"
	opCreateObjectStorageBucket            = "CreateObjectStorageBucket"
	opCreateObjectStorageBucketProto       = "CreateObjectStorageBucketProto"
	opCreateObjectStorageKey               = "CreateObjectStorageKey"
	opCreateObjectStorageKeyProto          = "CreateObjectStorageKeyProto"
	opCreatePresignedURL                   = "CreatePresignedURL"
	opCreatePresignedURLProto              = "CreatePresignedURLProto"
	opDeleteBucketSSL                      = "DeleteBucketSSL"
	opDeleteObjectStorageBucket            = "DeleteObjectStorageBucket"
	opDeleteObjectStorageKey               = "DeleteObjectStorageKey"
	opGetBucketSSL                         = "GetBucketSSL"
	opGetBucketSSLProto                    = "GetBucketSSLProto"
	opGetObjectACL                         = "GetObjectACL"
	opGetObjectACLProto                    = "GetObjectACLProto"
	opGetObjectStorageBucket               = "GetObjectStorageBucket"
	opGetObjectStorageBucketAccess         = "GetObjectStorageBucketAccess"
	opGetObjectStorageBucketAccessProto    = "GetObjectStorageBucketAccessProto"
	opGetObjectStorageBucketProto          = "GetObjectStorageBucketProto"
	opGetObjectStorageKey                  = "GetObjectStorageKey"
	opGetObjectStorageKeyProto             = "GetObjectStorageKeyProto"
	opGetObjectStorageQuota                = "GetObjectStorageQuota"
	opGetObjectStorageQuotaProto           = "GetObjectStorageQuotaProto"
	opGetObjectStorageQuotaUsage           = "GetObjectStorageQuotaUsage"
	opGetObjectStorageQuotaUsageProto      = "GetObjectStorageQuotaUsageProto"
	opGetObjectStorageTransfer             = "GetObjectStorageTransfer"
	opGetObjectStorageTransferProto        = "GetObjectStorageTransferProto"
	opListObjectStorageBucketContents      = "ListObjectStorageBucketContents"
	opListObjectStorageBucketContentsProto = "ListObjectStorageBucketContentsProto"
	opUpdateObjectACL                      = "UpdateObjectACL"
	opUpdateObjectACLProto                 = "UpdateObjectACLProto"
	opUpdateObjectStorageBucketAccess      = "UpdateObjectStorageBucketAccess"
	opUpdateObjectStorageKey               = "UpdateObjectStorageKey"
	opUpdateObjectStorageKeyProto          = "UpdateObjectStorageKeyProto"
	opUploadBucketSSL                      = "UploadBucketSSL"
	opUploadBucketSSLProto                 = "UploadBucketSSLProto"
)

// TestRoutedTransportObjectStorage checks that each object storage method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportObjectStorage(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opAllowObjectStorageBucketAccess,
			operation: opAllowObjectStorageBucketAccess,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.AllowObjectStorageBucketAccess(ctx, "alpha", "alpha", linode.AllowObjectStorageBucketAccessRequest{})
			},
		},
		{
			name:      opCancelObjectStorage,
			operation: opCancelObjectStorage,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.CancelObjectStorage(ctx)
			},
		},
		{
			name:      opCreateObjectStorageBucketProto,
			operation: opCreateObjectStorageBucket,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateObjectStorageBucketProto(ctx, &linode.CreateObjectStorageBucketRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateObjectStorageKeyProto,
			operation: opCreateObjectStorageKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateObjectStorageKeyProto(ctx, linode.CreateObjectStorageKeyRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreatePresignedURLProto,
			operation: opCreatePresignedURL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreatePresignedURLProto(ctx, "alpha", "alpha", linode.PresignedURLRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteBucketSSL,
			operation: opDeleteBucketSSL,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteBucketSSL(ctx, "alpha", "alpha")
			},
		},
		{
			name:      opDeleteObjectStorageBucket,
			operation: opDeleteObjectStorageBucket,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteObjectStorageBucket(ctx, "alpha", "alpha")
			},
		},
		{
			name:      opDeleteObjectStorageKey,
			operation: opDeleteObjectStorageKey,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteObjectStorageKey(ctx, 4242)
			},
		},
		{
			name:      opGetBucketSSL,
			operation: opGetBucketSSL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetBucketSSL(ctx, "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetBucketSSLProto,
			operation: opGetBucketSSL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetBucketSSLProto(ctx, "alpha", "alpha")

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
			name:      opGetObjectACLProto,
			operation: opGetObjectACL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectACLProto(ctx, "alpha", "alpha", "alpha")

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
			name:      opGetObjectStorageBucketAccessProto,
			operation: opGetObjectStorageBucketAccess,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageBucketAccessProto(ctx, "alpha", "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageBucketProto,
			operation: opGetObjectStorageBucket,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageBucketProto(ctx, "alpha", "alpha")

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
		{
			name:      opGetObjectStorageKeyProto,
			operation: opGetObjectStorageKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageKeyProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageQuotaProto,
			operation: opGetObjectStorageQuota,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageQuotaProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageQuotaUsageProto,
			operation: opGetObjectStorageQuotaUsage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageQuotaUsageProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetObjectStorageTransferProto,
			operation: opGetObjectStorageTransfer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetObjectStorageTransferProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      opListObjectStorageBucketContentsProto,
			operation: opListObjectStorageBucketContents,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListObjectStorageBucketContentsProto(ctx, "alpha", "alpha", nil)

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateObjectACLProto,
			operation: opUpdateObjectACL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateObjectACLProto(ctx, "alpha", "alpha", linode.ObjectACLUpdateRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateObjectStorageBucketAccess,
			operation: opUpdateObjectStorageBucketAccess,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.UpdateObjectStorageBucketAccess(ctx, "alpha", "alpha", linode.UpdateObjectStorageBucketAccessRequest{})
			},
		},
		{
			name:      opUpdateObjectStorageKeyProto,
			operation: opUpdateObjectStorageKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateObjectStorageKeyProto(ctx, 4242, linode.UpdateObjectStorageKeyRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUploadBucketSSLProto,
			operation: opUploadBucketSSL,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UploadBucketSSLProto(ctx, "alpha", "alpha", linode.UploadBucketSSLRequest{})

				return clientRouteError(err)
			},
		},
	})
}

package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesObjectStoragePart1 pins the object storage client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesObjectStoragePart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AllowObjectStorageBucketAccess",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoAccess,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.AllowObjectStorageBucketAccess(ctx, "alpha", "bravo", linode.AllowObjectStorageBucketAccessRequest{}))
			},
		},
		{
			name:     "CancelObjectStorage",
			wantVerb: http.MethodPost,
			wantPath: "/object-storage/cancel",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.CancelObjectStorage(ctx))
			},
		},
		{
			name:     "CreateObjectStorageBucketProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathObjectStorageBuckets,
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateObjectStorageBucketProto(ctx, linode.CreateObjectStorageBucketRequest{})

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "CreateObjectStorageKeyProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathObjectStorageKeys,
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateObjectStorageKeyProto(ctx, linode.CreateObjectStorageKeyRequest{})

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "CreatePresignedURLProto",
			wantVerb: http.MethodPost,
			wantPath: "/object-storage/buckets/alpha/bravo/object-url",
			response: clientRouteProtoObjURL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreatePresignedURLProto(ctx, "alpha", "bravo", linode.PresignedURLRequest{})

				return clientRouteProbe(err, func() any { return got.GetUrl() })
			},
		},
		{
			name:     "DeleteBucketSSL",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoSSL,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteBucketSSL(ctx, "alpha", "bravo"))
			},
		},
		{
			name:     "DeleteObjectStorageBucket",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravo,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteObjectStorageBucket(ctx, "alpha", "bravo"))
			},
		},
		{
			name:     "DeleteObjectStorageKey",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathObjectStorageKeys4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteObjectStorageKey(ctx, 4242))
			},
		},
		{
			name:     "GetBucketSSL",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoSSL,
			response: clientRouteObjSSLBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetBucketSSL(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.SSL })
			},
		},
		{
			name:     "GetBucketSSLProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoSSL,
			response: clientRouteProtoObjSSLBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetBucketSSLProto(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.GetSsl() })
			},
		},
		{
			name:     "GetObjectACL",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoObjectACL,
			response: clientRouteObjACL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectACL(ctx, "alpha", "bravo", "charlie")

				return clientRouteProbe(err, func() any { return got.ACL })
			},
		},
		{
			name:     "GetObjectACLProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoObjectACL,
			response: clientRouteProtoObjACL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectACLProto(ctx, "alpha", "bravo", "charlie")

				return clientRouteProbe(err, func() any { return got.GetAcl() })
			},
		},
	})
}

// TestClientRoutesObjectStoragePart2 pins the object storage client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesObjectStoragePart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetObjectStorageBucket",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravo,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageBucket(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetObjectStorageBucketAccess",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoAccess,
			response: clientRouteObjACL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageBucketAccess(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.ACL })
			},
		},
		{
			name:     "GetObjectStorageBucketAccessProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoAccess,
			response: clientRouteProtoObjACL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageBucketAccessProto(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.GetAcl() })
			},
		},
		{
			name:     "GetObjectStorageBucketProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravo,
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageBucketProto(ctx, "alpha", "bravo")

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "GetObjectStorageKey",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageKeys4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageKey(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetObjectStorageKeyProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageKeys4242,
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageKeyProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "GetObjectStorageQuotaProto",
			wantVerb: http.MethodGet,
			wantPath: "/object-storage/quotas/alpha",
			response: clientRouteProtoObjQuotaID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetObjectStorageQuotaProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetQuotaId() })
			},
		},
		{
			name:     "ListObjectStorageBucketsByRegionProto",
			wantVerb: http.MethodGet,
			wantPath: "/object-storage/buckets/alpha",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListObjectStorageBucketsByRegionProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ObjectStorageBucket).GetLabel) })
			},
		},
		{
			name:     "ListObjectStorageBucketsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageBuckets,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListObjectStorageBucketsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ObjectStorageBucket).GetLabel) })
			},
		},
		{
			name:     "ListObjectStorageEndpointsProto",
			wantVerb: http.MethodGet,
			wantPath: "/object-storage/endpoints",
			response: clientRouteProtoPageRegion,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListObjectStorageEndpointsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ObjectStorageEndpoint).GetRegion) })
			},
		},
		{
			name:     "ListObjectStorageKeysProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathObjectStorageKeys,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListObjectStorageKeysProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ObjectStorageKey).GetLabel) })
			},
		},
		{
			name:     "ListObjectStorageQuotasProto",
			wantVerb: http.MethodGet,
			wantPath: "/object-storage/quotas",
			response: clientRouteProtoPageQuotaID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListObjectStorageQuotasProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ObjectStorageQuota).GetQuotaId) })
			},
		},
	})
}

// TestClientRoutesObjectStoragePart3 pins the object storage client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesObjectStoragePart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListObjectStorageTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/object-storage/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListObjectStorageTypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LinodeType).GetId) })
			},
		},
		{
			name:     "UpdateObjectACLProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoObjectACL,
			response: clientRouteProtoObjACL,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateObjectACLProto(ctx, "alpha", "bravo", linode.ObjectACLUpdateRequest{})

				return clientRouteProbe(err, func() any { return got.GetAcl() })
			},
		},
		{
			name:     "UpdateObjectStorageBucketAccess",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoAccess,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.UpdateObjectStorageBucketAccess(ctx, "alpha", "bravo", linode.UpdateObjectStorageBucketAccessRequest{}))
			},
		},
		{
			name:     "UpdateObjectStorageKeyProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathObjectStorageKeys4242,
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateObjectStorageKeyProto(ctx, 4242, linode.UpdateObjectStorageKeyRequest{})

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "UploadBucketSSLProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathObjectStorageBucketsAlphaBravoSSL,
			response: clientRouteProtoObjSSLBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UploadBucketSSLProto(ctx, "alpha", "bravo", linode.UploadBucketSSLRequest{})

				return clientRouteProbe(err, func() any { return got.GetSsl() })
			},
		},
	})
}

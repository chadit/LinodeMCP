package linode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesObjectStoragePart1 pins the object storage client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesObjectStoragePart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
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
	})
}

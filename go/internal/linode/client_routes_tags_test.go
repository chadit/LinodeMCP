package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesTags pins the tag client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesTags(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CreateTagProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathTags,
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateTagProto(ctx, &linode.CreateTagRequest{})

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "DeleteTag",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathTagsAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteTag(ctx, "alpha"))
			},
		},
		{
			name:     "ListTaggedObjectsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathTagsAlpha,
			response: clientRouteProtoPageType,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListTaggedObjectsProto(ctx, "alpha", 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.TaggedObject).GetType) })
			},
		},
		{
			name:     "ListTagsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathTags,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListTagsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Tag).GetLabel) })
			},
		},
	})
}

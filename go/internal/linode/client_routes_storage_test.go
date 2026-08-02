package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesStoragePart1 pins the volume and StackScript client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesStoragePart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AttachVolumeProto",
			wantVerb: http.MethodPost,
			wantPath: "/volumes/4242/attach",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AttachVolumeProto(ctx, 4242, linode.AttachVolumeRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CloneVolumeProto",
			wantVerb: http.MethodPost,
			wantPath: "/volumes/4242/clone",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CloneVolumeProto(ctx, 4242, linode.CloneVolumeRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateSSHKeyProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathProfileSshkeys,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateSSHKeyProto(ctx, linode.CreateSSHKeyRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateVolumeProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathVolumes,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateVolumeProto(ctx, &linode.CreateVolumeRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteSSHKey",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathProfileSshkeys4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteSSHKey(ctx, 4242))
			},
		},
		{
			name:     "DeleteVolume",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathVolumes4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteVolume(ctx, 4242))
			},
		},
		{
			name:     "DetachVolume",
			wantVerb: http.MethodPost,
			wantPath: "/volumes/4242/detach",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DetachVolume(ctx, 4242))
			},
		},
		{
			name:     "GetSSHKey",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileSshkeys4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetSSHKey(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetSSHKeyProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileSshkeys4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetSSHKeyProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetVolume",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVolumes4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVolume(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetVolumeProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVolumes4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetVolumeProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListSSHKeysProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileSshkeys,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListSSHKeysProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.SSHKey).GetLabel) })
			},
		},
	})
}

// TestClientRoutesStoragePart2 pins the volume and StackScript client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesStoragePart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListVolumeTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/volumes/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVolumeTypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LinodeType).GetId) })
			},
		},
		{
			name:     "ListVolumesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathVolumes,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVolumesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Volume).GetLabel) })
			},
		},
		{
			name:     "ResizeVolumeProto",
			wantVerb: http.MethodPost,
			wantPath: "/volumes/4242/resize",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ResizeVolumeProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateSSHKeyProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathProfileSshkeys4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateSSHKeyProto(ctx, 4242, linode.UpdateSSHKeyRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateVolumeProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathVolumes4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateVolumeProto(ctx, 4242, &linode.UpdateVolumeRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

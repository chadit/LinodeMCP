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
	opAttachVolume      = "AttachVolume"
	opAttachVolumeProto = "AttachVolumeProto"
	opCloneVolume       = "CloneVolume"
	opCloneVolumeProto  = "CloneVolumeProto"
	opCreateSSHKey      = "CreateSSHKey"
	opCreateSSHKeyProto = "CreateSSHKeyProto"
	opCreateVolume      = "CreateVolume"
	opCreateVolumeProto = "CreateVolumeProto"
	opDeleteSSHKey      = "DeleteSSHKey"
	opDeleteVolume      = "DeleteVolume"
	opDetachVolume      = "DetachVolume"
	opGetSSHKey         = "GetSSHKey"
	opGetSSHKeyProto    = "GetSSHKeyProto"
	opGetVolume         = "GetVolume"
	opGetVolumeProto    = "GetVolumeProto"
	opResizeVolume      = "ResizeVolume"
	opResizeVolumeProto = "ResizeVolumeProto"
	opUpdateSSHKey      = "UpdateSSHKey"
	opUpdateSSHKeyProto = "UpdateSSHKeyProto"
	opUpdateVolume      = "UpdateVolume"
	opUpdateVolumeProto = "UpdateVolumeProto"
)

// TestRoutedTransportStorage checks that each storage method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportStorage(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opAttachVolumeProto,
			operation: opAttachVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AttachVolumeProto(ctx, 4242, linode.AttachVolumeRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCloneVolumeProto,
			operation: opCloneVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CloneVolumeProto(ctx, 4242, linode.CloneVolumeRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateSSHKeyProto,
			operation: opCreateSSHKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateSSHKeyProto(ctx, linode.CreateSSHKeyRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateVolumeProto,
			operation: opCreateVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateVolumeProto(ctx, &linode.CreateVolumeRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteSSHKey,
			operation: opDeleteSSHKey,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteSSHKey(ctx, 4242)
			},
		},
		{
			name:      opDeleteVolume,
			operation: opDeleteVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteVolume(ctx, 4242)
			},
		},
		{
			name:      opDetachVolume,
			operation: opDetachVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DetachVolume(ctx, 4242)
			},
		},
		{
			name:      opGetSSHKey,
			operation: opGetSSHKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetSSHKey(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetSSHKeyProto,
			operation: opGetSSHKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetSSHKeyProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVolume,
			operation: opGetVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVolume(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetVolumeProto,
			operation: opGetVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetVolumeProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opResizeVolumeProto,
			operation: opResizeVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ResizeVolumeProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateSSHKeyProto,
			operation: opUpdateSSHKey,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateSSHKeyProto(ctx, 4242, linode.UpdateSSHKeyRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateVolumeProto,
			operation: opUpdateVolume,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateVolumeProto(ctx, 4242, &linode.UpdateVolumeRequest{})

				return clientRouteError(err)
			},
		},
	})
}

package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and operation labels, named because most of these strings appear
// in more than one case below.
const (
	opAddImageShareGroupImages           = "AddImageShareGroupImages"
	opAddImageShareGroupImagesProto      = "AddImageShareGroupImagesProto"
	opAddImageShareGroupMembers          = "AddImageShareGroupMembers"
	opAddImageShareGroupMembersProto     = "AddImageShareGroupMembersProto"
	opBootInstance                       = "BootInstance"
	opCreateImage                        = "CreateImage"
	opCreateImageProto                   = "CreateImageProto"
	opCreateImageShareGroup              = "CreateImageShareGroup"
	opCreateImageShareGroupProto         = "CreateImageShareGroupProto"
	opCreateImageShareGroupToken         = "CreateImageShareGroupToken"
	opCreateImageShareGroupTokenProto    = "CreateImageShareGroupTokenProto"
	opCreateInstance                     = "CreateInstance"
	opCreateInstanceProto                = "CreateInstanceProto"
	opCreateStackScript                  = "CreateStackScript"
	opCreateStackScriptProto             = "CreateStackScriptProto"
	opDeleteImage                        = "DeleteImage"
	opDeleteImageShareGroup              = "DeleteImageShareGroup"
	opDeleteImageShareGroupImage         = "DeleteImageShareGroupImage"
	opDeleteImageShareGroupMemberToken   = "DeleteImageShareGroupMemberToken"
	opDeleteImageShareGroupToken         = "DeleteImageShareGroupToken"
	opDeleteInstance                     = "DeleteInstance"
	opDeleteStackScript                  = "DeleteStackScript"
	opGetImage                           = "GetImage"
	opGetImageProto                      = "GetImageProto"
	opGetImageShareGroup                 = "GetImageShareGroup"
	opGetImageShareGroupByToken          = "GetImageShareGroupByToken"
	opGetImageShareGroupByTokenProto     = "GetImageShareGroupByTokenProto"
	opGetImageShareGroupMemberToken      = "GetImageShareGroupMemberToken"
	opGetImageShareGroupMemberTokenProto = "GetImageShareGroupMemberTokenProto"
	opGetImageShareGroupProto            = "GetImageShareGroupProto"
	opGetImageShareGroupToken            = "GetImageShareGroupToken"
	opGetImageShareGroupTokenProto       = "GetImageShareGroupTokenProto"
	opGetInstance                        = "GetInstance"
	opGetInstanceProto                   = "GetInstanceProto"
	opGetInstanceStatsByYearMonth        = "GetInstanceStatsByYearMonth"
	opGetInstanceStatsByYearMonthProto   = "GetInstanceStatsByYearMonthProto"
	opGetInstanceTransfer                = "GetInstanceTransfer"
	opGetInstanceTransferProto           = "GetInstanceTransferProto"
	opGetKernel                          = "GetKernel"
	opGetKernelProto                     = "GetKernelProto"
	opGetRegion                          = "GetRegion"
	opGetRegionProto                     = "GetRegionProto"
	opGetStackScript                     = "GetStackScript"
	opGetStackScriptProto                = "GetStackScriptProto"
	opGetType                            = "GetType"
	opGetTypeProto                       = "GetTypeProto"
	opRebootInstance                     = "RebootInstance"
	opReplicateImage                     = "ReplicateImage"
	opReplicateImageProto                = "ReplicateImageProto"
	opResizeInstance                     = "ResizeInstance"
	opShutdownInstance                   = "ShutdownInstance"
	opUpdateImage                        = "UpdateImage"
	opUpdateImageProto                   = "UpdateImageProto"
	opUpdateImageShareGroup              = "UpdateImageShareGroup"
	opUpdateImageShareGroupImage         = "UpdateImageShareGroupImage"
	opUpdateImageShareGroupImageProto    = "UpdateImageShareGroupImageProto"
	opUpdateImageShareGroupMember        = "UpdateImageShareGroupMember"
	opUpdateImageShareGroupMemberProto   = "UpdateImageShareGroupMemberProto"
	opUpdateImageShareGroupProto         = "UpdateImageShareGroupProto"
	opUpdateImageShareGroupToken         = "UpdateImageShareGroupToken"
	opUpdateImageShareGroupTokenProto    = "UpdateImageShareGroupTokenProto"
	opUpdateInstance                     = "UpdateInstance"
	opUpdateInstanceProto                = "UpdateInstanceProto"
	opUpdateStackScript                  = "UpdateStackScript"
	opUpdateStackScriptProto             = "UpdateStackScriptProto"
	opUploadImage                        = "UploadImage"
	opUploadImageProto                   = "UploadImageProto"
)

// TestRoutedTransportCompute checks that each compute method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
//
// The table is split in two because one function holding every case trips the
// maintainability-index check on sheer size, not on complexity.
func TestRoutedTransportCompute(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opAddImageShareGroupImagesProto,
			operation: opAddImageShareGroupImages,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AddImageShareGroupImagesProto(ctx, 4242, &linode.AddImageShareGroupImagesRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opAddImageShareGroupMembersProto,
			operation: opAddImageShareGroupMembers,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AddImageShareGroupMembersProto(ctx, 4242, &linode.AddImageShareGroupMembersRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opBootInstance,
			operation: opBootInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.BootInstance(ctx, 4242, nil)
			},
		},
		{
			name:      opCreateImageProto,
			operation: opCreateImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateImageProto(ctx, &linode.CreateImageRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateImageShareGroupProto,
			operation: opCreateImageShareGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateImageShareGroupProto(ctx, &linode.CreateImageShareGroupRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateImageShareGroupTokenProto,
			operation: opCreateImageShareGroupToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateImageShareGroupTokenProto(ctx, &linode.CreateImageShareGroupTokenRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateInstanceProto,
			operation: opCreateInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateInstanceProto(ctx, &linode.CreateInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateStackScriptProto,
			operation: opCreateStackScript,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateStackScriptProto(ctx, &linode.CreateStackScriptRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteImage,
			operation: opDeleteImage,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteImage(ctx, "alpha")
			},
		},
		{
			name:      opDeleteImageShareGroup,
			operation: opDeleteImageShareGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteImageShareGroup(ctx, 4242)
			},
		},
		{
			name:      opDeleteImageShareGroupImage,
			operation: opDeleteImageShareGroupImage,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteImageShareGroupImage(ctx, 4242, 4242)
			},
		},
		{
			name:      opDeleteImageShareGroupMemberToken,
			operation: opDeleteImageShareGroupMemberToken,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteImageShareGroupMemberToken(ctx, 4242, "alpha")
			},
		},
		{
			name:      opDeleteImageShareGroupToken,
			operation: opDeleteImageShareGroupToken,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteImageShareGroupToken(ctx, "alpha")
			},
		},
		{
			name:      opDeleteInstance,
			operation: opDeleteInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteInstance(ctx, 4242)
			},
		},
		{
			name:      opDeleteStackScript,
			operation: opDeleteStackScript,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteStackScript(ctx, 4242)
			},
		},
		{
			name:      opGetImage,
			operation: opGetImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImage(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageProto,
			operation: opGetImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroup,
			operation: opGetImageShareGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroup(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroupByToken,
			operation: opGetImageShareGroupByToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroupByToken(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroupByTokenProto,
			operation: opGetImageShareGroupByToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroupByTokenProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroupMemberTokenProto,
			operation: opGetImageShareGroupMemberToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroupMemberTokenProto(ctx, 4242, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroupProto,
			operation: opGetImageShareGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroupProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroupTokenProto,
			operation: opGetImageShareGroupToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroupTokenProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportComputeMore continues TestRoutedTransportCompute.
func TestRoutedTransportComputeMore(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetInstance,
			operation: opGetInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstance(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetInstanceProto,
			operation: opGetInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetInstanceStatsByYearMonthProto,
			operation: opGetInstanceStatsByYearMonth,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceStatsByYearMonthProto(ctx, 4242, 2024, 6)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetInstanceTransferProto,
			operation: opGetInstanceTransfer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceTransferProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetKernelProto,
			operation: opGetKernel,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetKernelProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetRegion,
			operation: opGetRegion,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetRegion(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetRegionProto,
			operation: opGetRegion,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetRegionProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetStackScript,
			operation: opGetStackScript,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetStackScript(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetStackScriptProto,
			operation: opGetStackScript,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetStackScriptProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetType,
			operation: opGetType,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetType(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetTypeProto,
			operation: opGetType,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetTypeProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opRebootInstance,
			operation: opRebootInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.RebootInstance(ctx, 4242, nil)
			},
		},
		{
			name:      opReplicateImageProto,
			operation: opReplicateImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ReplicateImageProto(ctx, "alpha", &linode.ReplicateImageRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opResizeInstance,
			operation: opResizeInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.ResizeInstance(ctx, 4242, linode.ResizeInstanceRequest{})
			},
		},
		{
			name:      opShutdownInstance,
			operation: opShutdownInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.ShutdownInstance(ctx, 4242)
			},
		},
		{
			name:      opUpdateImageProto,
			operation: opUpdateImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateImageProto(ctx, "alpha", &linode.UpdateImageRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateImageShareGroupImageProto,
			operation: opUpdateImageShareGroupImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateImageShareGroupImageProto(ctx, 4242, "alpha", &linode.UpdateImageShareGroupImageRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateImageShareGroupMemberProto,
			operation: opUpdateImageShareGroupMember,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateImageShareGroupMemberProto(ctx, 4242, "alpha", &linode.UpdateImageShareGroupMemberRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateImageShareGroupProto,
			operation: opUpdateImageShareGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateImageShareGroupProto(ctx, 4242, &linode.UpdateImageShareGroupRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateImageShareGroupTokenProto,
			operation: opUpdateImageShareGroupToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateImageShareGroupTokenProto(ctx, "alpha", &linode.UpdateImageShareGroupTokenRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateInstanceProto,
			operation: opUpdateInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceProto(ctx, 4242, &linode.UpdateInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateStackScriptProto,
			operation: opUpdateStackScript,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateStackScriptProto(ctx, 4242,
					&linode.UpdateStackScriptRequest{Label: testLabel()})

				return clientRouteError(err)
			},
		},
		{
			name:      opUploadImageProto,
			operation: opUploadImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, _, err := client.UploadImageProto(ctx, &linode.UploadImageRequest{})

				return clientRouteError(err)
			},
		},
	})
}

package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesComputePart1 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AddImageShareGroupImagesProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathImagesSharegroups4242Images,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AddImageShareGroupImagesProto(ctx, 4242, &linode.AddImageShareGroupImagesRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "AddImageShareGroupMembersProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathImagesSharegroups4242Members,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AddImageShareGroupMembersProto(ctx, 4242, &linode.AddImageShareGroupMembersRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "BootInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/boot",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.BootInstance(ctx, 4242, nil))
			},
		},
		{
			name:     "CreateImageProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathImages,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateImageProto(ctx, &linode.CreateImageRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateImageShareGroupProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathImagesSharegroups,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateImageShareGroupProto(ctx, &linode.CreateImageShareGroupRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateImageShareGroupTokenProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathImagesSharegroupsTokens,
			response: clientRouteProtoObjToken,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateImageShareGroupTokenProto(ctx, &linode.CreateImageShareGroupTokenRequest{})

				return clientRouteProbe(err, func() any { return got.GetToken() })
			},
		},
		{
			name:     "CreateInstanceProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLinodeInstances,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateInstanceProto(ctx, &linode.CreateInstanceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateStackScriptProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLinodeStackscripts,
			response: clientRouteProtoObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateStackScriptProto(ctx, &linode.CreateStackScriptRequest{})

				return clientRouteProbe(err, func() any { return got.GetUsername() })
			},
		},
		{
			name:     "DeleteImage",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathImagesAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteImage(ctx, "alpha"))
			},
		},
		{
			name:     "DeleteImageShareGroup",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathImagesSharegroups4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteImageShareGroup(ctx, 4242))
			},
		},
		{
			name:     "DeleteImageShareGroupImage",
			wantVerb: http.MethodDelete,
			wantPath: "/images/sharegroups/4242/images/8615",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteImageShareGroupImage(ctx, 4242, 8615))
			},
		},
		{
			name:     "DeleteImageShareGroupMemberToken",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathImagesSharegroups4242MembersAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteImageShareGroupMemberToken(ctx, 4242, "alpha"))
			},
		},
	})
}

// TestClientRoutesComputePart2 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "DeleteImageShareGroupToken",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathImagesSharegroupsTokensAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteImageShareGroupToken(ctx, "alpha"))
			},
		},
		{
			name:     "DeleteInstance",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeInstances4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteInstance(ctx, 4242))
			},
		},
		{
			name:     "DeleteStackScript",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeStackscripts4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteStackScript(ctx, 4242))
			},
		},
		{
			name:     "GetImage",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImage(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     "GetImageProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetImageShareGroup",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups4242,
			response: clientRouteObjUUID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroup(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.UUID })
			},
		},
		{
			name:     "GetImageShareGroupByToken",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroupsTokensAlphaSharegroup,
			response: clientRouteObjUUID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroupByToken(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.UUID })
			},
		},
		{
			name:     "GetImageShareGroupByTokenProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroupsTokensAlphaSharegroup,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroupByTokenProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetImageShareGroupMemberTokenProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups4242MembersAlpha,
			response: clientRouteProtoObjTokenUUID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroupMemberTokenProto(ctx, 4242, "alpha")

				return clientRouteProbe(err, func() any { return got.GetTokenUuid() })
			},
		},
		{
			name:     "GetImageShareGroupProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroupProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetImageShareGroupTokenProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroupsTokensAlpha,
			response: clientRouteProtoObjToken,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetImageShareGroupTokenProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetToken() })
			},
		},
		{
			name:     "GetInstance",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242,
			response: clientRouteObjHypervisor,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstance(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Hypervisor })
			},
		},
	})
}

// TestClientRoutesComputePart3 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetInstanceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetInstanceStatsByYearMonthProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/instances/4242/stats/2024/6",
			response: clientRouteProtoObjTitle,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceStatsByYearMonthProto(ctx, 4242, 2024, 6)

				return clientRouteProbe(err, func() any { return got.GetTitle() })
			},
		},
		{
			name:     "GetInstanceTransferProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/instances/4242/transfer",
			response: clientRouteProtoObjBillableInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceTransferProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetBillable() })
			},
		},
		{
			name:     "GetKernelProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/kernels/alpha",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetKernelProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetRegion",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathRegionsAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetRegion(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     "GetRegionAvailabilityProto",
			wantVerb: http.MethodGet,
			wantPath: "/regions/alpha/availability",
			response: clientRouteProtoArrayRegion,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetRegionAvailabilityProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.RegionAvailability).GetRegion) })
			},
		},
		{
			name:     "GetRegionProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathRegionsAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetRegionProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetStackScript",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeStackscripts4242,
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetStackScript(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "GetStackScriptProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeStackscripts4242,
			response: clientRouteProtoObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetStackScriptProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetUsername() })
			},
		},
		{
			name:     "GetType",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeTypesAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetType(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     "GetTypeProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeTypesAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetTypeProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListImageShareGroupTokensProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroupsTokens,
			response: clientRouteProtoPageToken,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListImageShareGroupTokensProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ImageShareGroupToken).GetToken) })
			},
		},
	})
}

// TestClientRoutesComputePart4 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart4(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListImageShareGroupsByImageProto",
			wantVerb: http.MethodGet,
			wantPath: "/images/alpha/sharegroups",
			response: clientRouteProtoPageUUID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListImageShareGroupsByImageProto(ctx, "alpha", 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ImageShareGroup).GetUuid) })
			},
		},
		{
			name:     "ListImageShareGroupsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups,
			response: clientRouteProtoPageUUID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListImageShareGroupsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ImageShareGroup).GetUuid) })
			},
		},
		{
			name:     "ListImagesByShareGroupProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups4242Images,
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListImagesByShareGroupProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Image).GetId) })
			},
		},
		{
			name:     "ListImagesByShareGroupTokenProto",
			wantVerb: http.MethodGet,
			wantPath: "/images/sharegroups/tokens/alpha/sharegroup/images",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListImagesByShareGroupTokenProto(ctx, "alpha", 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Image).GetId) })
			},
		},
		{
			name:     "ListImagesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImages,
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListImagesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Image).GetId) })
			},
		},
		{
			name:     "ListInstancesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstancesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Instance).GetLabel) })
			},
		},
		{
			name:     "ListKernelsProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/kernels",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListKernelsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Kernel).GetId) })
			},
		},
		{
			name:     "ListMembersByImageShareGroupProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathImagesSharegroups4242Members,
			response: clientRouteProtoPageTokenUUID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMembersByImageShareGroupProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ImageShareGroupMember).GetTokenUuid) })
			},
		},
		{
			name:     "ListRegionsAvailabilityProto",
			wantVerb: http.MethodGet,
			wantPath: "/regions/availability",
			response: clientRouteProtoPageRegion,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListRegionsAvailabilityProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.RegionAvailability).GetRegion) })
			},
		},
		{
			name:     "ListRegionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/regions",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListRegionsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Region).GetId) })
			},
		},
		{
			name:     "ListStackScriptsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeStackscripts,
			response: clientRouteProtoPageUsername,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListStackScriptsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.StackScript).GetUsername) })
			},
		},
		{
			name:     "ListTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListTypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.InstanceType).GetId) })
			},
		},
	})
}

// TestClientRoutesComputePart5 pins the compute client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesComputePart5(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "RebootInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/reboot",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RebootInstance(ctx, 4242, nil))
			},
		},
		{
			name:     "ReplicateImageProto",
			wantVerb: http.MethodPost,
			wantPath: "/images/alpha/regions",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ReplicateImageProto(ctx, "alpha", &linode.ReplicateImageRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ResizeInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/resize",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResizeInstance(ctx, 4242, linode.ResizeInstanceRequest{}))
			},
		},
		{
			name:     "ShutdownInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/shutdown",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ShutdownInstance(ctx, 4242))
			},
		},
		{
			name:     "UpdateImageProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathImagesAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateImageProto(ctx, "alpha", &linode.UpdateImageRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateImageShareGroupImageProto",
			wantVerb: http.MethodPut,
			wantPath: "/images/sharegroups/4242/images/alpha",
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateImageShareGroupImageProto(ctx, 4242, "alpha", &linode.UpdateImageShareGroupImageRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateImageShareGroupMemberProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathImagesSharegroups4242MembersAlpha,
			response: clientRouteProtoObjTokenUUID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateImageShareGroupMemberProto(ctx, 4242, "alpha", &linode.UpdateImageShareGroupMemberRequest{})

				return clientRouteProbe(err, func() any { return got.GetTokenUuid() })
			},
		},
		{
			name:     "UpdateImageShareGroupProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathImagesSharegroups4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateImageShareGroupProto(ctx, 4242, &linode.UpdateImageShareGroupRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateImageShareGroupTokenProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathImagesSharegroupsTokensAlpha,
			response: clientRouteProtoObjToken,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateImageShareGroupTokenProto(ctx, "alpha", &linode.UpdateImageShareGroupTokenRequest{})

				return clientRouteProbe(err, func() any { return got.GetToken() })
			},
		},
		{
			name:     "UpdateInstanceProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLinodeInstances4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateInstanceProto(ctx, 4242, &linode.UpdateInstanceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

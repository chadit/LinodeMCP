package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesInstanceDeepPart1 pins the instance sub-resource client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesInstanceDeepPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AddInstanceConfigInterfaceProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLinodeInstances4242Configs8615Interfaces,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AddInstanceConfigInterfaceProto(ctx, 4242, 8615, &linode.ConfigInterface{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "AddInstanceInterfaceProto",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/interfaces",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AddInstanceInterfaceProto(ctx, 4242, &linode.AddInstanceInterfaceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ApplyInstanceFirewalls",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/firewalls/apply",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ApplyInstanceFirewalls(ctx, 4242))
			},
		},
		{
			name:     "CancelInstanceBackups",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/backups/cancel",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.CancelInstanceBackups(ctx, 4242))
			},
		},
		{
			name:     "CloneInstanceDiskProto",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/disks/8615/clone",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CloneInstanceDiskProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CloneInstanceProto",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/clone",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CloneInstanceProto(ctx, 4242, &linode.CloneInstanceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateInstanceBackupProto",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/backups",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateInstanceBackupProto(ctx, 4242, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateInstanceConfigProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLinodeInstances4242Configs,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateInstanceConfigProto(ctx, 4242, &linode.CreateConfigRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateInstanceDiskProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathLinodeInstances4242Disks,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateInstanceDiskProto(ctx, 4242, &linode.CreateDiskRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteInstanceConfig",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeInstances4242Configs8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteInstanceConfig(ctx, 4242, 8615))
			},
		},
		{
			name:     "DeleteInstanceConfigInterface",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeInstances4242Configs8615Interfaces1379,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteInstanceConfigInterface(ctx, 4242, 8615, 1379))
			},
		},
		{
			name:     "DeleteInstanceDisk",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeInstances4242Disks8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteInstanceDisk(ctx, 4242, 8615))
			},
		},
	})
}

// TestClientRoutesInstanceDeepPart2 pins the instance sub-resource client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesInstanceDeepPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "DeleteInstanceIP",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeInstances4242IpsAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteInstanceIP(ctx, 4242, "alpha"))
			},
		},
		{
			name:     "DeleteInstanceInterface",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathLinodeInstances4242Interfaces8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteInstanceInterface(ctx, 4242, 8615))
			},
		},
		{
			name:     "EnableInstanceBackups",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/backups/enable",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.EnableInstanceBackups(ctx, 4242))
			},
		},
		{
			name:     "GetInstanceBackup",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Backups8615,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceBackup(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetInstanceBackupProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Backups8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceBackupProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetInstanceConfig",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs8615,
			response: clientRouteObjComments,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceConfig(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Comments })
			},
		},
		{
			name:     "GetInstanceConfigInterface",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs8615Interfaces1379,
			response: clientRouteObjPurpose,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceConfigInterface(ctx, 4242, 8615, 1379)

				return clientRouteProbe(err, func() any { return got.Purpose })
			},
		},
		{
			name:     "GetInstanceConfigInterfaceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs8615Interfaces1379,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceConfigInterfaceProto(ctx, 4242, 8615, 1379)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetInstanceConfigProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceConfigProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetInstanceDisk",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Disks8615,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceDisk(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetInstanceDiskProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Disks8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceDiskProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetInstanceIP",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242IpsAlpha,
			response: clientRouteObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceIP(ctx, 4242, "alpha")

				return clientRouteProbe(err, func() any { return got.Address })
			},
		},
	})
}

// TestClientRoutesInstanceDeepPart3 pins the instance sub-resource client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesInstanceDeepPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetInstanceIPProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242IpsAlpha,
			response: clientRouteProtoObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceIPProto(ctx, 4242, "alpha")

				return clientRouteProbe(err, func() any { return got.GetAddress() })
			},
		},
		{
			name:     "GetInstanceInterface",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Interfaces8615,
			response: clientRouteObjMacAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceInterface(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.MACAddress })
			},
		},
		{
			name:     "GetInstanceInterfaceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Interfaces8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceInterfaceProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetInstanceStatsProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/instances/4242/stats",
			response: clientRouteProtoObjTitle,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceStatsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetTitle() })
			},
		},
		{
			name:     "ListInstanceConfigInterfacesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs8615Interfaces,
			response: clientRouteProtoArrayPurpose,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceConfigInterfacesProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ConfigInterfaceResponse).GetPurpose) })
			},
		},
		{
			name:     "ListInstanceConfigs",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs,
			response: clientRoutePageComments,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceConfigs(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any {
					return clientRouteList(got, func(item linode.InstanceConfig) string { return item.Comments })
				})
			},
		},
		{
			name:     "ListInstanceConfigsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Configs,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceConfigsProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.InstanceConfig).GetLabel) })
			},
		},
		{
			name:     "ListInstanceDisks",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Disks,
			response: clientRoutePageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceDisks(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, func(item linode.InstanceDisk) string { return item.Label }) })
			},
		},
		{
			name:     "ListInstanceDisksProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Disks,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceDisksProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.InstanceDisk).GetLabel) })
			},
		},
		{
			name:     "ListInstanceFirewalls",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Firewalls,
			response: clientRoutePageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceFirewalls(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, func(item linode.Firewall) string { return item.Label }) })
			},
		},
		{
			name:     "ListInstanceFirewallsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Firewalls,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceFirewallsProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Firewall).GetLabel) })
			},
		},
		{
			name:     "ListInstanceInterfaceFirewallsProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/instances/4242/interfaces/8615/firewalls",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceInterfaceFirewallsProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Firewall).GetLabel) })
			},
		},
	})
}

// TestClientRoutesInstanceDeepPart4 pins the instance sub-resource client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesInstanceDeepPart4(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListInstanceInterfaceHistoryProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/instances/4242/interfaces/history",
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceInterfaceHistoryProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.InstanceInterfaceHistory).GetCreated) })
			},
		},
		{
			name:     "ListInstanceNodeBalancersProto",
			wantVerb: http.MethodGet,
			wantPath: "/linode/instances/4242/nodebalancers",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceNodeBalancersProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.NodeBalancer).GetLabel) })
			},
		},
		{
			name:     "ListInstanceVolumes",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Volumes,
			response: clientRoutePageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceVolumes(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, func(item linode.Volume) string { return item.Label }) })
			},
		},
		{
			name:     "ListInstanceVolumesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathLinodeInstances4242Volumes,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceVolumesProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Volume).GetLabel) })
			},
		},
		{
			name:     "MigrateInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/migrate",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.MigrateInstance(ctx, 4242, "alpha"))
			},
		},
		{
			name:     "MutateInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/mutate",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.MutateInstance(ctx, 4242, &linode.MutateInstanceRequest{}))
			},
		},
		{
			name:     "RebuildInstanceProto",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/rebuild",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.RebuildInstanceProto(ctx, 4242, &linode.RebuildInstanceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ReorderInstanceConfigInterfaces",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/configs/8615/interfaces/order",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ReorderInstanceConfigInterfaces(ctx, 4242, 8615, &linode.ReorderConfigInterfacesRequest{}))
			},
		},
		{
			name:     "RescueInstance",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/rescue",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RescueInstance(ctx, 4242, linode.RescueInstanceRequest{}))
			},
		},
		{
			name:     "ResetInstanceDiskPassword",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/disks/8615/password",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResetInstanceDiskPassword(ctx, 4242, 8615, "alpha"))
			},
		},
		{
			name:     "ResetInstancePassword",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/password",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResetInstancePassword(ctx, 4242, "alpha"))
			},
		},
		{
			name:     "ResizeInstanceDisk",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/disks/8615/resize",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.ResizeInstanceDisk(ctx, 4242, 8615, linode.ResizeDiskRequest{}))
			},
		},
	})
}

// TestClientRoutesInstanceDeepPart5 pins the instance sub-resource client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesInstanceDeepPart5(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "RestoreInstanceBackup",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/backups/8615/restore",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RestoreInstanceBackup(ctx, 4242, 8615, linode.RestoreBackupRequest{}))
			},
		},
		{
			name:     "UpdateInstanceConfigInterfaceProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLinodeInstances4242Configs8615Interfaces1379,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateInstanceConfigInterfaceProto(ctx, 4242, 8615, 1379, &linode.UpdateConfigInterfaceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateInstanceConfigProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLinodeInstances4242Configs8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateInstanceConfigProto(ctx, 4242, 8615, &linode.UpdateConfigRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateInstanceDiskProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLinodeInstances4242Disks8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateInstanceDiskProto(ctx, 4242, 8615, linode.UpdateDiskRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateInstanceFirewallsProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLinodeInstances4242Firewalls,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateInstanceFirewallsProto(ctx, 4242, 1, 25, &linode.UpdateInstanceFirewallsRequest{})

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Firewall).GetLabel) })
			},
		},
		{
			name:     "UpdateInstanceInterfaceProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathLinodeInstances4242Interfaces8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateInstanceInterfaceProto(ctx, 4242, 8615, &linode.UpdateInstanceInterfaceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpgradeLinodeInterfacesProto",
			wantVerb: http.MethodPost,
			wantPath: "/linode/instances/4242/upgrade-interfaces",
			response: clientRouteProtoObjMessage,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpgradeLinodeInterfacesProto(ctx, 4242, &linode.UpgradeLinodeInterfacesRequest{})

				return clientRouteProbe(err, func() any { return got.GetMessage() })
			},
		},
	})
}

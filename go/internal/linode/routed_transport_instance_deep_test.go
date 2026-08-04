package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Operation labels also used by client_routes_instance_deep_test.go and
// create_no_replay_test.go; one label names a plain method and its proto twin.
const (
	labelAddInstanceConfigInterfaceProto = "AddInstanceConfigInterfaceProto"
	labelCloneInstanceDiskProto          = "CloneInstanceDiskProto"
	labelCloneInstanceProto              = "CloneInstanceProto"
	labelCreateInstanceBackupProto       = "CreateInstanceBackupProto"
	labelCreateInstanceConfigProto       = "CreateInstanceConfigProto"
	labelCreateInstanceDiskProto         = "CreateInstanceDiskProto"
	labelGetInstanceBackup               = "GetInstanceBackup"
	labelGetInstanceConfig               = "GetInstanceConfig"
	labelGetInstanceConfigInterface      = "GetInstanceConfigInterface"
	labelGetInstanceDisk                 = "GetInstanceDisk"
	labelGetInstanceIP                   = "GetInstanceIP"
	labelGetInstanceInterface            = "GetInstanceInterface"
	labelGetInstanceInterfaceSettings    = "GetInstanceInterfaceSettings"
	labelListInstanceIPs                 = "ListInstanceIPs"
)

// TestRoutedTransportInstanceDeepPart1 checks that each method below reports a
// failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportInstanceDeepPart1(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      labelAddInstanceConfigInterfaceProto,
			operation: "AddInstanceConfigInterface",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AddInstanceConfigInterfaceProto(ctx, 4242, 4242, &linode.ConfigInterface{})

				return clientRouteError(err)
			},
		},
		{
			name:      "AddInstanceInterfaceProto",
			operation: "AddInstanceInterface",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AddInstanceInterfaceProto(ctx, 4242, &linode.AddInstanceInterfaceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "AllocateInstanceIPProto",
			operation: "AllocateInstanceIP",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AllocateInstanceIPProto(ctx, 4242, linode.AllocateIPRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "ApplyInstanceFirewalls",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ApplyInstanceFirewalls(ctx, 4242))
			},
		},
		{
			name: "CancelInstanceBackups",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.CancelInstanceBackups(ctx, 4242))
			},
		},
		{
			name:      labelCloneInstanceDiskProto,
			operation: "CloneInstanceDisk",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CloneInstanceDiskProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      labelCloneInstanceProto,
			operation: "CloneInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CloneInstanceProto(ctx, 4242, &linode.CloneInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      labelCreateInstanceBackupProto,
			operation: "CreateInstanceBackup",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateInstanceBackupProto(ctx, 4242, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      labelCreateInstanceConfigProto,
			operation: "CreateInstanceConfig",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateInstanceConfigProto(ctx, 4242, &linode.CreateConfigRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      labelCreateInstanceDiskProto,
			operation: "CreateInstanceDisk",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateInstanceDiskProto(ctx, 4242, &linode.CreateDiskRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "DeleteInstanceConfig",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteInstanceConfig(ctx, 4242, 4242))
			},
		},
		{
			name: "DeleteInstanceConfigInterface",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteInstanceConfigInterface(ctx, 4242, 4242, 4242))
			},
		},
		{
			name: "DeleteInstanceDisk",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteInstanceDisk(ctx, 4242, 4242))
			},
		},
		{
			name: "DeleteInstanceIP",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteInstanceIP(ctx, 4242, "alpha"))
			},
		},
		{
			name: "DeleteInstanceInterface",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteInstanceInterface(ctx, 4242, 4242))
			},
		},
		{
			name: "EnableInstanceBackups",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.EnableInstanceBackups(ctx, 4242))
			},
		},
		{
			name: labelGetInstanceBackup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceBackup(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceBackupProto",
			operation: labelGetInstanceBackup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceBackupProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetInstanceConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceConfig(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetInstanceConfigInterface,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceConfigInterface(ctx, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceConfigInterfaceProto",
			operation: labelGetInstanceConfigInterface,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceConfigInterfaceProto(ctx, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceConfigProto",
			operation: labelGetInstanceConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceConfigProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetInstanceDisk,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceDisk(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceDiskProto",
			operation: labelGetInstanceDisk,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceDiskProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetInstanceIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceIP(ctx, 4242, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceIPProto",
			operation: labelGetInstanceIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceIPProto(ctx, 4242, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetInstanceInterface,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceInterface(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceInterfaceProto",
			operation: labelGetInstanceInterface,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceInterfaceProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetInstanceInterfaceSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceInterfaceSettings(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetInstanceInterfaceSettingsProto",
			operation: labelGetInstanceInterfaceSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceInterfaceSettingsProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportInstanceDeepPart2 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportInstanceDeepPart2(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      "GetInstanceStatsProto",
			operation: "GetInstanceStats",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceStatsProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "ListInstanceBackupsProto",
			operation: "ListInstanceBackups",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceBackupsProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "ListInstanceConfigs",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceConfigs(ctx, 4242, 1, 100)

				return clientRouteError(err)
			},
		},
		{
			name: "ListInstanceDisks",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceDisks(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "ListInstanceFirewalls",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceFirewalls(ctx, 4242, 1, 100)

				return clientRouteError(err)
			},
		},
		{
			name: labelListInstanceIPs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceIPs(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "ListInstanceIPsProto",
			operation: labelListInstanceIPs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceIPsProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "ListInstanceVolumes",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceVolumes(ctx, 4242, 1, 100)

				return clientRouteError(err)
			},
		},
		{
			name: "MigrateInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.MigrateInstance(ctx, 4242, "alpha"))
			},
		},
		{
			name: "MutateInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.MutateInstance(ctx, 4242, &linode.MutateInstanceRequest{}))
			},
		},
		{
			name:      "RebuildInstanceProto",
			operation: "RebuildInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.RebuildInstanceProto(ctx, 4242, &linode.RebuildInstanceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "ReorderInstanceConfigInterfaces",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ReorderInstanceConfigInterfaces(ctx, 4242, 4242, &linode.ReorderConfigInterfacesRequest{}))
			},
		},
		{
			name: "RescueInstance",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RescueInstance(ctx, 4242, linode.RescueInstanceRequest{}))
			},
		},
		{
			name: "ResetInstanceDiskPassword",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ResetInstanceDiskPassword(ctx, 4242, 4242, "alpha"))
			},
		},
		{
			name: "ResetInstancePassword",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ResetInstancePassword(ctx, 4242, "alpha"))
			},
		},
		{
			name: "ResizeInstanceDisk",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.ResizeInstanceDisk(ctx, 4242, 4242, linode.ResizeDiskRequest{}))
			},
		},
		{
			name: "RestoreInstanceBackup",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RestoreInstanceBackup(ctx, 4242, 4242, linode.RestoreBackupRequest{}))
			},
		},
		{
			name:      "UpdateInstanceConfigInterfaceProto",
			operation: "UpdateInstanceConfigInterface",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceConfigInterfaceProto(ctx, 4242, 4242, 4242, &linode.UpdateConfigInterfaceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateInstanceConfigProto",
			operation: "UpdateInstanceConfig",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceConfigProto(ctx, 4242, 4242, &linode.UpdateConfigRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateInstanceDiskProto",
			operation: "UpdateInstanceDisk",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceDiskProto(ctx, 4242, 4242, linode.UpdateDiskRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateInstanceFirewallsProto",
			operation: "UpdateInstanceFirewalls",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceFirewallsProto(ctx, 4242, 1, 100, &linode.UpdateInstanceFirewallsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateInstanceIPProto",
			operation: "UpdateInstanceIP",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceIPProto(ctx, 4242, "alpha", linode.UpdateIPRDNSRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateInstanceInterfaceProto",
			operation: "UpdateInstanceInterface",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceInterfaceProto(ctx, 4242, 4242, &linode.UpdateInstanceInterfaceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateInstanceInterfaceSettingsProto",
			operation: "UpdateInstanceInterfaceSettings",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateInstanceInterfaceSettingsProto(ctx, 4242, &linode.UpdateInstanceInterfaceSettingsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpgradeLinodeInterfacesProto",
			operation: "UpgradeLinodeInterfaces",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpgradeLinodeInterfacesProto(ctx, 4242, &linode.UpgradeLinodeInterfacesRequest{})

				return clientRouteError(err)
			},
		},
	})
}

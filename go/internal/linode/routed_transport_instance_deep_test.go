package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Operation labels also used by client_routes_instance_deep_test.go and
// create_no_replay_test.go; one label names a plain method and its proto twin.
const (
	labelGetInstanceBackup            = "GetInstanceBackup"
	labelGetInstanceConfig            = "GetInstanceConfig"
	labelGetInstanceConfigInterface   = "GetInstanceConfigInterface"
	labelGetInstanceDisk              = "GetInstanceDisk"
	labelGetInstanceIP                = "GetInstanceIP"
	labelGetInstanceInterface         = "GetInstanceInterface"
	labelGetInstanceInterfaceSettings = "GetInstanceInterfaceSettings"
	labelListInstanceIPs              = "ListInstanceIPs"
)

// TestRoutedTransportInstanceDeepPart1 checks that each method below reports a
// failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportInstanceDeepPart1(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: labelGetInstanceBackup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceBackup(ctx, 4242, 4242)

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
			name: labelGetInstanceDisk,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceDisk(ctx, 4242, 4242)

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
			name: labelGetInstanceInterface,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstanceInterface(ctx, 4242, 4242)

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
	})
}

// TestRoutedTransportInstanceDeepPart2 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportInstanceDeepPart2(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
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
			name: "ListInstanceVolumes",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListInstanceVolumes(ctx, 4242, 1, 100)

				return clientRouteError(err)
			},
		},
	})
}

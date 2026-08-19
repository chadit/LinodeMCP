package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientRoutesInstanceDeepPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     labelGetInstanceBackup,
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
			name:     labelGetInstanceConfig,
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
			name:     labelGetInstanceConfigInterface,
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
			name:     labelGetInstanceDisk,
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
			name:     labelGetInstanceIP,
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

func TestClientRoutesInstanceDeepPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     labelGetInstanceInterface,
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
	})
}

// TestClientRoutesInstanceDeepPart4 pins the instance sub-resource client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesInstanceDeepPart4(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
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
	})
}

// Response bodies and paths for the interface-settings, IP-list, and backup-list
// cases below. They stay in this file rather than the shared route fixtures
// because no other resource's cases read these shapes.
const (
	instanceRouteIPv4Public       = "{\"ipv4\":{\"public\":[{\"address\":\"probe-value\"}]}}"
	instanceRouteNetworkHelper    = "{\"network_helper\":true}"
	instanceRoutePathIps          = "/linode/instances/4242/ips"
	instanceRoutePathIfaceSetting = "/linode/instances/4242/interfaces/settings"
)

// TestClientRoutesInstanceDeepPart6 pins the whole-resource reads and the IP
// writes, the instance methods whose route and decode nothing else exercises.
func TestClientRoutesInstanceDeepPart6(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     labelGetInstanceInterfaceSettings,
			wantVerb: http.MethodGet,
			wantPath: instanceRoutePathIfaceSetting,
			response: instanceRouteNetworkHelper,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetInstanceInterfaceSettings(ctx, 4242)

				return clientRouteProbe(err, func() any {
					return got.NetworkHelper != nil && *got.NetworkHelper
				})
			},
		},
		{
			name:     labelListInstanceIPs,
			wantVerb: http.MethodGet,
			wantPath: instanceRoutePathIps,
			response: instanceRouteIPv4Public,
			want:     "1:" + clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListInstanceIPs(ctx, 4242)

				return clientRouteProbe(err, func() any {
					return clientRouteList(got.IPv4.Public, func(ip linode.IPAddress) string { return ip.Address })
				})
			},
		},
	})
}

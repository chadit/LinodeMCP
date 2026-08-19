package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesNetworkingPart2 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetFirewall",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetFirewall(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetFirewallDevice",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242Devices8615,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetFirewallDevice(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetNetworkingIP",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingIps20301135,
			response: clientRouteObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNetworkingIP(ctx, clientRouteIPv4Fixture)

				return clientRouteProbe(err, func() any { return got.Address })
			},
		},
	})
}

// TestClientRoutesNetworkingPart3 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetNodeBalancer",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancer(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetNodeBalancerConfigNode",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes1379,
			response: clientRouteObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerConfigNode(ctx, 4242, 8615, 1379)

				return clientRouteProbe(err, func() any { return got.Address })
			},
		},
		{
			name:     "GetNodeBalancerConfig",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs8615,
			response: clientRouteObjProtocol,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerConfig(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.Protocol })
			},
		},
		{
			name:     "GetReservedIPRaw",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingReservedIps20301135,
			response: clientRouteObjProbe,
			want:     clientRouteObjProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetReservedIPRaw(ctx, clientRouteIPv4Fixture)

				return clientRouteProbe(err, func() any { return string(got) })
			},
		},
		{
			name:     "ListFirewallDevices",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242Devices,
			response: clientRoutePageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallDevices(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any {
					return clientRouteList(got.Data, func(item linode.FirewallDevice) string { return item.Created })
				})
			},
		},
	})
}

// TestClientRoutesNetworkingPart4 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart4(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListFirewallRules",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242Rules,
			response: clientRouteObjInboundPolicy,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallRules(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.InboundPolicy })
			},
		},
		{
			name:     "ListNodeBalancerConfigNodes",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes,
			response: clientRoutePageAddress,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerConfigNodes(ctx, 4242, 8615, 1, 25)

				return clientRouteProbe(err, func() any {
					return clientRouteList(got.Data, func(item linode.NodeBalancerConfigNode) string { return item.Address })
				})
			},
		},
		{
			name:     "ListNodeBalancerConfigs",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs,
			response: clientRoutePageCipherSuite,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerConfigs(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any {
					return clientRouteList(got, func(item linode.NodeBalancerConfig) string { return item.CipherSuite })
				})
			},
		},
	})
}

// TestClientRoutesNetworkingPart5 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart5(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListNodeBalancerFirewallsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Firewalls,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerFirewallsProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Firewall).GetLabel) })
			},
		},
		{
			name:     "ListVLANs",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingVlans,
			response: clientRoutePageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVLANs(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got.Data, func(item linode.VLAN) string { return item.Label }) })
			},
		},
	})
}

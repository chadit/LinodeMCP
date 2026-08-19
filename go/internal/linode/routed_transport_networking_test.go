package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and the operation each reports under; a proto variant reports
// under its plain method's operation. Named to keep goconst quiet.
const (
	opGetFirewall                 = "GetFirewall"
	opGetFirewallDevice           = "GetFirewallDevice"
	opGetNetworkingIP             = "GetNetworkingIP"
	opGetNodeBalancer             = "GetNodeBalancer"
	opGetNodeBalancerConfig       = "GetNodeBalancerConfig"
	opGetNodeBalancerConfigNode   = "GetNodeBalancerConfigNode"
	opGetReservedIP               = "GetReservedIP"
	opGetReservedIPRaw            = "GetReservedIPRaw"
	opListFirewallDevices         = "ListFirewallDevices"
	opListFirewallRules           = "ListFirewallRules"
	opListFirewallSettings        = "ListFirewallSettings"
	opListNodeBalancerConfigNodes = "ListNodeBalancerConfigNodes"
	opListNodeBalancerConfigs     = "ListNodeBalancerConfigs"
	opListVLANs                   = "ListVLANs"
)

// TestRoutedTransportNetworking checks that each networking method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
//
// The table is split in two because one function holding every case trips the
// maintainability-index check on sheer size, not on complexity.
func TestRoutedTransportNetworking(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetFirewall,
			operation: opGetFirewall,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetFirewall(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetFirewallDevice,
			operation: opGetFirewallDevice,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetFirewallDevice(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetNetworkingIP,
			operation: opGetNetworkingIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNetworkingIP(ctx, testIPv4)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetNodeBalancer,
			operation: opGetNodeBalancer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancer(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetNodeBalancerConfigNode,
			operation: opGetNodeBalancerConfigNode,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerConfigNode(ctx, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportNetworkingMore continues TestRoutedTransportNetworking.
func TestRoutedTransportNetworkingMore(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetNodeBalancerConfig,
			operation: opGetNodeBalancerConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerConfig(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetReservedIPRaw,
			operation: opGetReservedIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetReservedIPRaw(ctx, testIPv4)

				return clientRouteError(err)
			},
		},
		{
			name:      opListFirewallDevices,
			operation: opListFirewallDevices,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListFirewallDevices(ctx, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListFirewallRules,
			operation: opListFirewallRules,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListFirewallRules(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListFirewallSettings,
			operation: opListFirewallSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListFirewallSettings(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListNodeBalancerConfigNodes,
			operation: opListNodeBalancerConfigNodes,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListNodeBalancerConfigNodes(ctx, 4242, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListNodeBalancerConfigs,
			operation: opListNodeBalancerConfigs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListNodeBalancerConfigs(ctx, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListVLANs,
			operation: opListVLANs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListVLANs(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
	})
}

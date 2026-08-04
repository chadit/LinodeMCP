package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and the operation each reports under; a proto variant reports
// under its plain method's operation. Named to keep goconst quiet.
const (
	opAllocateNetworkingIP             = "AllocateNetworkingIP"
	opAllocateNetworkingIPProto        = "AllocateNetworkingIPProto"
	opAssignNetworkingIPs              = "AssignNetworkingIPs"
	opAssignNetworkingIPv4s            = "AssignNetworkingIPv4s"
	opCreateFirewall                   = "CreateFirewall"
	opCreateFirewallDevice             = "CreateFirewallDevice"
	opCreateFirewallDeviceProto        = "CreateFirewallDeviceProto"
	opCreateFirewallProto              = "CreateFirewallProto"
	opCreateIPv6Range                  = "CreateIPv6Range"
	opCreateIPv6RangeProto             = "CreateIPv6RangeProto"
	opCreateNodeBalancer               = "CreateNodeBalancer"
	opCreateNodeBalancerConfig         = "CreateNodeBalancerConfig"
	opCreateNodeBalancerConfigProto    = "CreateNodeBalancerConfigProto"
	opCreateNodeBalancerNode           = "CreateNodeBalancerNode"
	opCreateNodeBalancerNodeProto      = "CreateNodeBalancerNodeProto"
	opCreateNodeBalancerProto          = "CreateNodeBalancerProto"
	opCreateReservedIP                 = "CreateReservedIP"
	opCreateReservedIPRaw              = "CreateReservedIPRaw"
	opDeleteFirewall                   = "DeleteFirewall"
	opDeleteFirewallDevice             = "DeleteFirewallDevice"
	opDeleteNodeBalancer               = "DeleteNodeBalancer"
	opDeleteNodeBalancerConfig         = "DeleteNodeBalancerConfig"
	opDeleteNodeBalancerConfigNode     = "DeleteNodeBalancerConfigNode"
	opDeleteReservedIP                 = "DeleteReservedIP"
	opDeleteVLAN                       = "DeleteVLAN"
	opGetFirewall                      = "GetFirewall"
	opGetFirewallDevice                = "GetFirewallDevice"
	opGetFirewallDeviceProto           = "GetFirewallDeviceProto"
	opGetFirewallProto                 = "GetFirewallProto"
	opGetFirewallRuleVersion           = "GetFirewallRuleVersion"
	opGetFirewallRuleVersionProto      = "GetFirewallRuleVersionProto"
	opGetFirewallTemplate              = "GetFirewallTemplate"
	opGetFirewallTemplateProto         = "GetFirewallTemplateProto"
	opGetNetworkingIP                  = "GetNetworkingIP"
	opGetNetworkingIPProto             = "GetNetworkingIPProto"
	opGetNodeBalancer                  = "GetNodeBalancer"
	opGetNodeBalancerConfig            = "GetNodeBalancerConfig"
	opGetNodeBalancerConfigNode        = "GetNodeBalancerConfigNode"
	opGetNodeBalancerConfigNodeProto   = "GetNodeBalancerConfigNodeProto"
	opGetNodeBalancerConfigProto       = "GetNodeBalancerConfigProto"
	opGetNodeBalancerProto             = "GetNodeBalancerProto"
	opGetNodeBalancerStats             = "GetNodeBalancerStats"
	opGetNodeBalancerStatsProto        = "GetNodeBalancerStatsProto"
	opGetNodeBalancerVPCConfig         = "GetNodeBalancerVPCConfig"
	opGetNodeBalancerVPCConfigProto    = "GetNodeBalancerVPCConfigProto"
	opGetReservedIP                    = "GetReservedIP"
	opGetReservedIPRaw                 = "GetReservedIPRaw"
	opListFirewallDevices              = "ListFirewallDevices"
	opListFirewallRules                = "ListFirewallRules"
	opListFirewallRulesProto           = "ListFirewallRulesProto"
	opListFirewallSettings             = "ListFirewallSettings"
	opListFirewallSettingsProto        = "ListFirewallSettingsProto"
	opListNodeBalancerConfigNodes      = "ListNodeBalancerConfigNodes"
	opListNodeBalancerConfigs          = "ListNodeBalancerConfigs"
	opListNodeBalancerFirewalls        = "ListNodeBalancerFirewalls"
	opListReservedIPs                  = "ListReservedIPs"
	opListReservedIPsProto             = "ListReservedIPsProto"
	opListVLANs                        = "ListVLANs"
	opRebuildNodeBalancerConfig        = "RebuildNodeBalancerConfig"
	opRebuildNodeBalancerConfigProto   = "RebuildNodeBalancerConfigProto"
	opShareNetworkingIPs               = "ShareNetworkingIPs"
	opShareNetworkingIPv4s             = "ShareNetworkingIPv4s"
	opUpdateFirewall                   = "UpdateFirewall"
	opUpdateFirewallProto              = "UpdateFirewallProto"
	opUpdateFirewallRules              = "UpdateFirewallRules"
	opUpdateFirewallRulesProto         = "UpdateFirewallRulesProto"
	opUpdateFirewallSettings           = "UpdateFirewallSettings"
	opUpdateFirewallSettingsProto      = "UpdateFirewallSettingsProto"
	opUpdateNetworkingIP               = "UpdateNetworkingIP"
	opUpdateNetworkingIPProto          = "UpdateNetworkingIPProto"
	opUpdateNodeBalancer               = "UpdateNodeBalancer"
	opUpdateNodeBalancerConfig         = "UpdateNodeBalancerConfig"
	opUpdateNodeBalancerConfigProto    = "UpdateNodeBalancerConfigProto"
	opUpdateNodeBalancerFirewalls      = "UpdateNodeBalancerFirewalls"
	opUpdateNodeBalancerFirewallsProto = "UpdateNodeBalancerFirewallsProto"
	opUpdateNodeBalancerNode           = "UpdateNodeBalancerNode"
	opUpdateNodeBalancerNodeProto      = "UpdateNodeBalancerNodeProto"
	opUpdateNodeBalancerProto          = "UpdateNodeBalancerProto"
	opUpdateReservedIP                 = "UpdateReservedIP"
	opUpdateReservedIPRaw              = "UpdateReservedIPRaw"
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
			name:      opAllocateNetworkingIPProto,
			operation: opAllocateNetworkingIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AllocateNetworkingIPProto(ctx,
					linode.AllocateNetworkingIPRequest{Type: "ipv4", LinodeID: 4242})

				return clientRouteError(err)
			},
		},
		{
			name:      opAssignNetworkingIPs,
			operation: opAssignNetworkingIPs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AssignNetworkingIPs(ctx,
					linode.AssignNetworkingIPsRequest{
						Region:      accountTransferRegion,
						Assignments: []linode.IPAssignment{{Address: testIPv4, LinodeID: 4242}},
					})

				return clientRouteError(err)
			},
		},
		{
			name:      opAssignNetworkingIPv4s,
			operation: opAssignNetworkingIPv4s,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.AssignNetworkingIPv4s(ctx,
					linode.AssignNetworkingIPsRequest{
						Region:      accountTransferRegion,
						Assignments: []linode.IPAssignment{{Address: testIPv4, LinodeID: 4242}},
					})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateFirewallDeviceProto,
			operation: opCreateFirewallDevice,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateFirewallDeviceProto(ctx, 4242,
					&linode.CreateFirewallDeviceRequest{Type: accountMaintenanceEntityType, ID: 4242})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateFirewallProto,
			operation: opCreateFirewall,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateFirewallProto(ctx, linode.CreateFirewallRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateIPv6RangeProto,
			operation: opCreateIPv6Range,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateIPv6RangeProto(ctx,
					linode.CreateIPv6RangeRequest{PrefixLength: 64, RouteTarget: testIPv6Target})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateNodeBalancerConfigProto,
			operation: opCreateNodeBalancerConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateNodeBalancerConfigProto(ctx, 4242, &linode.CreateNodeBalancerConfigRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateNodeBalancerNodeProto,
			operation: opCreateNodeBalancerNode,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateNodeBalancerNodeProto(ctx, 4242, 4242, &linode.CreateNodeBalancerNodeRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateNodeBalancerProto,
			operation: opCreateNodeBalancer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateNodeBalancerProto(ctx, &linode.CreateNodeBalancerRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateReservedIPRaw,
			operation: opCreateReservedIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateReservedIPRaw(ctx, "alpha", []string{"alpha"})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteFirewall,
			operation: opDeleteFirewall,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteFirewall(ctx, 4242)
			},
		},
		{
			name:      opDeleteFirewallDevice,
			operation: opDeleteFirewallDevice,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteFirewallDevice(ctx, 4242, 4242)
			},
		},
		{
			name:      opDeleteNodeBalancer,
			operation: opDeleteNodeBalancer,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteNodeBalancer(ctx, 4242)
			},
		},
		{
			name:      opDeleteNodeBalancerConfig,
			operation: opDeleteNodeBalancerConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteNodeBalancerConfig(ctx, 4242, 4242)
			},
		},
		{
			name:      opDeleteNodeBalancerConfigNode,
			operation: opDeleteNodeBalancerConfigNode,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteNodeBalancerConfigNode(ctx, 4242, 4242, 4242)
			},
		},
		{
			name:      opDeleteReservedIP,
			operation: opDeleteReservedIP,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteReservedIP(ctx, testIPv4)
			},
		},
		{
			name:      opDeleteVLAN,
			operation: opDeleteVLAN,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteVLAN(ctx, "alpha", "alpha")
			},
		},
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
			name:      opGetFirewallDeviceProto,
			operation: opGetFirewallDevice,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetFirewallDeviceProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetFirewallProto,
			operation: opGetFirewall,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetFirewallProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetFirewallRuleVersionProto,
			operation: opGetFirewallRuleVersion,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetFirewallRuleVersionProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetFirewallTemplateProto,
			operation: opGetFirewallTemplate,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetFirewallTemplateProto(ctx, "public", 4242, 4242)

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
			name:      opGetNetworkingIPProto,
			operation: opGetNetworkingIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNetworkingIPProto(ctx, testIPv4)

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
		{
			name:      opGetNodeBalancerConfigNodeProto,
			operation: opGetNodeBalancerConfigNode,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerConfigNodeProto(ctx, 4242, 4242, 4242)

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
			name:      opGetNodeBalancerConfigProto,
			operation: opGetNodeBalancerConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerConfigProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetNodeBalancerProto,
			operation: opGetNodeBalancer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetNodeBalancerStatsProto,
			operation: opGetNodeBalancerStats,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerStatsProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetNodeBalancerVPCConfigProto,
			operation: opGetNodeBalancerVPCConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetNodeBalancerVPCConfigProto(ctx, 4242, 4242)

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
			name:      opListFirewallRulesProto,
			operation: opListFirewallRules,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListFirewallRulesProto(ctx, 4242)

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
			name:      opListFirewallSettingsProto,
			operation: opListFirewallSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListFirewallSettingsProto(ctx, 4242, 4242)

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
			name:      opListNodeBalancerFirewalls,
			operation: opListNodeBalancerFirewalls,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListNodeBalancerFirewalls(ctx, 4242, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opListReservedIPsProto,
			operation: opListReservedIPs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListReservedIPsProto(ctx, 4242, 4242)

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
		{
			name:      opRebuildNodeBalancerConfigProto,
			operation: opRebuildNodeBalancerConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.RebuildNodeBalancerConfigProto(ctx, 4242, 4242, &linode.RebuildNodeBalancerConfigRequest{Port: 80})

				return clientRouteError(err)
			},
		},
		{
			name:      opShareNetworkingIPs,
			operation: opShareNetworkingIPs,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ShareNetworkingIPs(ctx,
					linode.ShareNetworkingIPsRequest{LinodeID: 4242, IPs: []string{testIPv4}})

				return clientRouteError(err)
			},
		},
		{
			name:      opShareNetworkingIPv4s,
			operation: opShareNetworkingIPv4s,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ShareNetworkingIPv4s(ctx,
					linode.ShareNetworkingIPsRequest{LinodeID: 4242, IPs: []string{testIPv4}})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateFirewallProto,
			operation: opUpdateFirewall,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateFirewallProto(ctx, 4242, linode.UpdateFirewallRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateFirewallRulesProto,
			operation: opUpdateFirewallRules,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateFirewallRulesProto(ctx, 4242, &linode.FirewallRulesReplaceRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateFirewallSettingsProto,
			operation: opUpdateFirewallSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateFirewallSettingsProto(ctx, &linode.UpdateFirewallSettingsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateNetworkingIPProto,
			operation: opUpdateNetworkingIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateNetworkingIPProto(ctx, testIPv4,
					linode.UpdateNetworkingIPRequest{RDNS: "host.example.com"})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateNodeBalancerConfigProto,
			operation: opUpdateNodeBalancerConfig,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateNodeBalancerConfigProto(ctx, 4242, 4242, &linode.UpdateNodeBalancerConfigRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateNodeBalancerFirewallsProto,
			operation: opUpdateNodeBalancerFirewalls,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateNodeBalancerFirewallsProto(ctx, 4242, 4242, 4242, &linode.UpdateNodeBalancerFirewallsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateNodeBalancerNodeProto,
			operation: opUpdateNodeBalancerNode,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateNodeBalancerNodeProto(ctx, 4242, 4242, 4242, &linode.UpdateNodeBalancerNodeRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateNodeBalancerProto,
			operation: opUpdateNodeBalancer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateNodeBalancerProto(ctx, 4242, linode.UpdateNodeBalancerRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateReservedIPRaw,
			operation: opUpdateReservedIP,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateReservedIPRaw(ctx, testIPv4, []string{"alpha"})

				return clientRouteError(err)
			},
		},
	})
}

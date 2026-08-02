package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesNetworkingPart1 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AllocateNetworkingIPProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNetworkingIps,
			response: clientRouteProtoObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AllocateNetworkingIPProto(ctx, linode.AllocateNetworkingIPRequest{Type: "ipv4", LinodeID: 4242, Public: true})

				return clientRouteProbe(err, func() any { return got.GetAddress() })
			},
		},
		{
			name:     "AssignNetworkingIPs",
			wantVerb: http.MethodPost,
			wantPath: "/networking/ips/assign",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AssignNetworkingIPs(ctx, linode.AssignNetworkingIPsRequest{Region: "us-east", Assignments: []linode.IPAssignment{{Address: clientRouteIPv4Fixture, LinodeID: 4242}}})

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
			},
		},
		{
			name:     "AssignNetworkingIPv4s",
			wantVerb: http.MethodPost,
			wantPath: "/networking/ipv4/assign",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.AssignNetworkingIPv4s(ctx, linode.AssignNetworkingIPsRequest{Region: "us-east", Assignments: []linode.IPAssignment{{Address: clientRouteIPv4Fixture, LinodeID: 4242}}})

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
			},
		},
		{
			name:     "CreateFirewallDeviceProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNetworkingFirewalls4242Devices,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateFirewallDeviceProto(ctx, 4242, &linode.CreateFirewallDeviceRequest{Type: "linode", ID: 4242})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateFirewallProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNetworkingFirewalls,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateFirewallProto(ctx, linode.CreateFirewallRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateIPv6RangeProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNetworkingIpv6Ranges,
			response: clientRouteProtoObjRange,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateIPv6RangeProto(ctx, linode.CreateIPv6RangeRequest{PrefixLength: 64})

				return clientRouteProbe(err, func() any { return got.GetRange() })
			},
		},
		{
			name:     "CreateNodeBalancerConfigProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNodebalancers4242Configs,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateNodeBalancerConfigProto(ctx, 4242, &linode.CreateNodeBalancerConfigRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateNodeBalancerNodeProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateNodeBalancerNodeProto(ctx, 4242, 8615, &linode.CreateNodeBalancerNodeRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateNodeBalancerProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNodebalancers,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateNodeBalancerProto(ctx, linode.CreateNodeBalancerRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateReservedIPRaw",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathNetworkingReservedIps,
			response: clientRouteObjProbe,
			want:     clientRouteObjProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateReservedIPRaw(ctx, "alpha", []string{"one"})

				return clientRouteProbe(err, func() any { return string(got) })
			},
		},
		{
			name:     "DeleteFirewall",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNetworkingFirewalls4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteFirewall(ctx, 4242))
			},
		},
		{
			name:     "DeleteFirewallDevice",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNetworkingFirewalls4242Devices8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteFirewallDevice(ctx, 4242, 8615))
			},
		},
	})
}

// TestClientRoutesNetworkingPart2 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "DeleteNodeBalancer",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNodebalancers4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteNodeBalancer(ctx, 4242))
			},
		},
		{
			name:     "DeleteNodeBalancerConfig",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNodebalancers4242Configs8615,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteNodeBalancerConfig(ctx, 4242, 8615))
			},
		},
		{
			name:     "DeleteNodeBalancerConfigNode",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes1379,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteNodeBalancerConfigNode(ctx, 4242, 8615, 1379))
			},
		},
		{
			name:     "DeleteReservedIP",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathNetworkingReservedIps20301135,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteReservedIP(ctx, clientRouteIPv4Fixture))
			},
		},
		{
			name:     "DeleteVLAN",
			wantVerb: http.MethodDelete,
			wantPath: "/networking/vlans/alpha/bravo",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteVLAN(ctx, "alpha", "bravo"))
			},
		},
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
			name:     "GetFirewallDeviceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242Devices8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetFirewallDeviceProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetFirewallProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetFirewallProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetFirewallRuleVersionProto",
			wantVerb: http.MethodGet,
			wantPath: "/networking/firewalls/4242/history/rules/8615",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetFirewallRuleVersionProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetFirewallTemplateProto",
			wantVerb: http.MethodGet,
			wantPath: "/networking/firewalls/templates/public",
			response: clientRouteProtoObjSlug,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetFirewallTemplateProto(ctx, "public", 1, 25)

				return clientRouteProbe(err, func() any { return got.GetSlug() })
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
			name:     "GetNetworkingIPProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingIps20301135,
			response: clientRouteProtoObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNetworkingIPProto(ctx, clientRouteIPv4Fixture)

				return clientRouteProbe(err, func() any { return got.GetAddress() })
			},
		},
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
			name:     "GetNodeBalancerConfigNodeProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes1379,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerConfigNodeProto(ctx, 4242, 8615, 1379)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetNodeBalancerConfigProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerConfigProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetNodeBalancerProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetNodeBalancerStatsProto",
			wantVerb: http.MethodGet,
			wantPath: "/nodebalancers/4242/stats",
			response: clientRouteProtoObjTitle,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerStatsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetTitle() })
			},
		},
		{
			name:     "GetNodeBalancerVPCConfigProto",
			wantVerb: http.MethodGet,
			wantPath: "/nodebalancers/4242/vpcs/8615",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetNodeBalancerVPCConfigProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
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
		{
			name:     "ListFirewallDevicesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242Devices,
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallDevicesProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.FirewallDevice).GetCreated) })
			},
		},
		{
			name:     "ListFirewallRuleVersionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/networking/firewalls/4242/history",
			response: clientRouteFirewallSnapshot,
			want:     "1:probe-value",
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallRuleVersionsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.FirewallRuleVersion).GetLabel) })
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
			name:     "ListFirewallRulesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls4242Rules,
			response: clientRouteProtoObjInboundPolicy,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallRulesProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetInboundPolicy() })
			},
		},
		{
			name:     "ListFirewallTemplatesProto",
			wantVerb: http.MethodGet,
			wantPath: "/networking/firewalls/templates",
			response: clientRouteProtoPageSlug,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallTemplatesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.FirewallTemplate).GetSlug) })
			},
		},
		{
			name:     "ListFirewallsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingFirewalls,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListFirewallsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Firewall).GetLabel) })
			},
		},
		{
			name:     "ListIPv6PoolsProto",
			wantVerb: http.MethodGet,
			wantPath: "/networking/ipv6/pools",
			response: clientRouteProtoPageRange,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListIPv6PoolsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.IPv6Pool).GetRange) })
			},
		},
		{
			name:     "ListIPv6RangesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingIpv6Ranges,
			response: clientRouteProtoPageRange,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListIPv6RangesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.IPv6Range).GetRange) })
			},
		},
		{
			name:     "ListNetworkTransferPricesProto",
			wantVerb: http.MethodGet,
			wantPath: "/network-transfer/prices",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNetworkTransferPricesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LinodeType).GetId) })
			},
		},
		{
			name:     "ListNetworkingIPsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingIps,
			response: clientRouteProtoPageAddress,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNetworkingIPsProto(ctx, false)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.IPAddress).GetAddress) })
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
			name:     "ListNodeBalancerConfigNodesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes,
			response: clientRouteProtoPageAddress,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerConfigNodesProto(ctx, 4242, 8615, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.NodeBalancerConfigNode).GetAddress) })
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
		{
			name:     "ListNodeBalancerConfigsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Configs,
			response: clientRouteProtoPageProtocol,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerConfigsProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.NodeBalancerConfig).GetProtocol) })
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
			name:     "ListNodeBalancerFirewalls",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers4242Firewalls,
			response: clientRoutePageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerFirewalls(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, func(item linode.Firewall) string { return item.Label }) })
			},
		},
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
			name:     "ListNodeBalancerTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/nodebalancers/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerTypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LinodeType).GetId) })
			},
		},
		{
			name:     "ListNodeBalancerVPCsProto",
			wantVerb: http.MethodGet,
			wantPath: "/nodebalancers/4242/vpcs",
			response: clientRouteProtoPageIpv4Range,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancerVPCsProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.NodeBalancerVPCConfig).GetIpv4Range) })
			},
		},
		{
			name:     "ListNodeBalancersProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNodebalancers,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListNodeBalancersProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.NodeBalancer).GetLabel) })
			},
		},
		{
			name:     "ListReservedIPTypesProto",
			wantVerb: http.MethodGet,
			wantPath: "/networking/reserved/ips/types",
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListReservedIPTypesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ReservedIPType).GetId) })
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
		{
			name:     "ListVLANsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathNetworkingVlans,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListVLANsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.VLAN).GetLabel) })
			},
		},
		{
			name:     "RebuildNodeBalancerConfigProto",
			wantVerb: http.MethodPost,
			wantPath: "/nodebalancers/4242/configs/8615/rebuild",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.RebuildNodeBalancerConfigProto(ctx, 4242, 8615)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ShareNetworkingIPs",
			wantVerb: http.MethodPost,
			wantPath: "/networking/ips/share",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ShareNetworkingIPs(ctx, linode.ShareNetworkingIPsRequest{IPs: []string{clientRouteIPv4Fixture}, LinodeID: 4242})

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
			},
		},
		{
			name:     "ShareNetworkingIPv4s",
			wantVerb: http.MethodPost,
			wantPath: "/networking/ipv4/share",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ShareNetworkingIPv4s(ctx, linode.ShareNetworkingIPsRequest{IPs: []string{clientRouteIPv4Fixture}, LinodeID: 4242})

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
			},
		},
		{
			name:     "UpdateFirewallProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNetworkingFirewalls4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateFirewallProto(ctx, 4242, linode.UpdateFirewallRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

// TestClientRoutesNetworkingPart6 pins the networking client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesNetworkingPart6(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "UpdateFirewallRulesProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNetworkingFirewalls4242Rules,
			response: clientRouteProtoObjInboundPolicy,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateFirewallRulesProto(ctx, 4242, &linode.FirewallRulesReplaceRequest{})

				return clientRouteProbe(err, func() any { return got.GetInboundPolicy() })
			},
		},
		{
			name:     "UpdateNetworkingIPProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNetworkingIps20301135,
			response: clientRouteProtoObjAddress,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateNetworkingIPProto(ctx, clientRouteIPv4Fixture, linode.UpdateNetworkingIPRequest{RDNS: "host.example.com"})

				return clientRouteProbe(err, func() any { return got.GetAddress() })
			},
		},
		{
			name:     "UpdateNodeBalancerConfigProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNodebalancers4242Configs8615,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateNodeBalancerConfigProto(ctx, 4242, 8615, &linode.UpdateNodeBalancerConfigRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateNodeBalancerFirewallsProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNodebalancers4242Firewalls,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateNodeBalancerFirewallsProto(ctx, 4242, 1, 25, &linode.UpdateNodeBalancerFirewallsRequest{})

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.Firewall).GetLabel) })
			},
		},
		{
			name:     "UpdateNodeBalancerNodeProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNodebalancers4242Configs8615Nodes1379,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateNodeBalancerNodeProto(ctx, 4242, 8615, 1379, &linode.UpdateNodeBalancerNodeRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateNodeBalancerProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNodebalancers4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateNodeBalancerProto(ctx, 4242, linode.UpdateNodeBalancerRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateReservedIPRaw",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathNetworkingReservedIps20301135,
			response: clientRouteObjProbe,
			want:     clientRouteObjProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateReservedIPRaw(ctx, clientRouteIPv4Fixture, []string{"one"})

				return clientRouteProbe(err, func() any { return string(got) })
			},
		},
	})
}

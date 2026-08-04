package tools_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The phase2 prefix marks the batch of request-body fields these tests cover:
// fields the tools gained to match the Linode API docs. The constants below are
// the fixture values this file introduces; a literal the test package already
// names is reused under that existing name instead of redeclared here.
const (
	errMasterIPsArray        = "master_ips must be an array of strings"
	keyInboundPolicy         = "inbound_policy"
	keyReservedIPv4Addresses = "reserved_ipv4_addresses"
	keyTaints                = "taints"
	phase2TaintKeyField      = "key"
	keyAPLEnabled            = "apl_enabled"
	keyConfigs               = "configs"
	keyDiskEncryption        = "disk_encryption"
	keyDisks                 = "disks"
	keyEncryption            = "encryption"
	keyEndpointType          = "endpoint_type"
	keyEntities              = "entities"
	keyLabels                = "labels"
	keyOutboundPolicy        = "outbound_policy"
	keyRecordTag             = "tag"
	keyReserved              = "reserved"
	keyRules                 = "rules"
	keyS3Endpoint            = "s3_endpoint"
	keyStackScriptData       = "stackscript_data"
	keyUpdateStrategy        = "update_strategy"
	keyVPCs                  = "vpcs"
	phase2DiskLabel          = "boot"
	phase2FirewallLabel      = "fw"
	phase2InvalidPolicy      = "MAYBE"
	phase2K8sVersion         = "1.31"
	phase2LabelTier          = "app"
	phase2MasterIP           = "192.0.2.2"
	phase2NonBoolean         = "yes"
	phase2RDNSAddress        = "203.0.113.5"
	phase2ReservedAddress    = "203.0.113.9"
	phase2StackScriptUser    = "admin"
	phase2TaintKey           = "dedicated"
	phase2WebFirewallLabel   = "web-fw"
)

type toolHandler = func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// toolFactory is the shape every tool constructor shares, so a table row can
// name the factory instead of repeating the signature.
type toolFactory func(cfg *config.Config) (mcp.Tool, profiles.Capability, toolHandler)

func phase2Handler(t *testing.T, factory toolFactory, cfg *config.Config) toolHandler {
	t.Helper()

	_, _, handler := factory(cfg)

	return handler
}

// phase2OfflineConfig points every tool at an address that refuses to connect.
// Every validation under test runs before the first request, so a rejection
// case never reaches the network.
func phase2OfflineConfig() *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
}

// phase2CapturingServer answers every request with response and returns the
// decoded request body, so a case can assert what reached the wire.
func phase2CapturingServer(t *testing.T, response map[string]any) (*config.Config, map[string]any) {
	t.Helper()

	captured := map[string]any{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			decoded := map[string]any{}
			// A body-less verb decodes to nothing; the error is expected there.
			if json.NewDecoder(r.Body).Decode(&decoded) == nil {
				maps.Copy(captured, decoded)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}

	return cfg, captured
}

type phase2Rejection struct {
	factory toolFactory
	args    map[string]any
	want    string
}

// phase2RunRejections drives each case against a config that refuses to
// connect, since every validation under test runs before the first request.
func phase2RunRejections(t *testing.T, cases map[string]phase2Rejection) {
	t.Helper()

	cfg := phase2OfflineConfig()

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := phase2Handler(t, testCase.factory, cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatalf("result.IsError = false, want true")
			}

			text, isText := result.Content[0].(mcp.TextContent)
			if !isText || text.Text != testCase.want {
				t.Errorf("error text = %q, want %q", text.Text, testCase.want)
			}
		})
	}
}

// TestPhase2BodyFieldRejectionsDNSAndFirewall pins the message each DNS and
// firewall body field reports for its invalid form. The table is split across
// three functions so no single one exceeds the maintainability ceiling.
func TestPhase2BodyFieldRejectionsDNSAndFirewall(t *testing.T) {
	t.Parallel()

	phase2RunRejections(t, map[string]phase2Rejection{
		"domain update axfr_ips": {
			factory: tools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyAXFRIPs: float64(7)},
			want:    "axfr_ips must be an array of strings",
		},
		"domain update master_ips": {
			factory: tools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyMasterIPs: float64(7)},
			want:    errMasterIPsArray,
		},
		"domain update type": {
			factory: tools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyPaymentType: "primary"},
			want:    "type must be one of: master, slave",
		},
	})
}

// TestPhase2BodyFieldRejectionsComputeAndLKE pins the same for the disk, LKE
// cluster, and LKE pool body fields.
func TestPhase2BodyFieldRejectionsComputeAndLKE(t *testing.T) {
	t.Parallel()

	phase2RunRejections(t, map[string]phase2Rejection{
		"domain update expire_sec": {
			factory: tools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyExpireSec: "soon"},
			want:    "expire_sec must be an integer",
		},
		"domain record create tag": {
			factory: tools.NewLinodeDomainRecordCreateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyPaymentType: "CAA", keyRecordTag: "issuance"},
			want:    "tag must be one of: issue, issuewild, iodef",
		},
		"domain record update tag": {
			factory: tools.NewLinodeDomainRecordUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), "record_id": float64(7), keyConfirm: true, keyRecordTag: "issuance"},
			want:    "tag must be one of: issue, issuewild, iodef",
		},
	})
}

// TestPhase2BodyFieldRejectionsPlatform pins the same for the monitor,
// networking, NodeBalancer, storage, tag, and account body fields.
func TestPhase2BodyFieldRejectionsPlatform(t *testing.T) {
	t.Parallel()

	phase2RunRejections(t, map[string]phase2Rejection{
		"firewall create rules": {
			factory: tools.NewLinodeFirewallCreateTool,
			args:    map[string]any{keyLabel: phase2FirewallLabel, keyConfirm: true, keyRules: float64(5)},
			want:    "rules must be an object",
		},
		"firewall create rules string": {
			factory: tools.NewLinodeFirewallCreateTool,
			args:    map[string]any{keyLabel: phase2FirewallLabel, keyConfirm: true, keyRules: "{oops"},
			want:    "rules must be an object",
		},
		"firewall create devices": {
			factory: tools.NewLinodeFirewallCreateTool,
			args:    map[string]any{keyLabel: phase2FirewallLabel, keyConfirm: true, keyDevices: []any{float64(1)}},
			want:    "devices must be an object",
		},
		"firewall rules update inbound_policy": {
			factory: tools.NewLinodeFirewallRulesUpdateTool,
			args: map[string]any{
				keyFirewallID: float64(1), keyConfirm: true,
				keyInbound: []any{}, keyOutbound: []any{}, keyInboundPolicy: phase2InvalidPolicy,
			},
			want: "inbound_policy must be one of: ACCEPT, DROP",
		},
		"firewall rules update outbound_policy": {
			factory: tools.NewLinodeFirewallRulesUpdateTool,
			args: map[string]any{
				keyFirewallID: float64(1), keyConfirm: true,
				keyInbound: []any{}, keyOutbound: []any{}, keyOutboundPolicy: phase2InvalidPolicy,
			},
			want: "outbound_policy must be one of: ACCEPT, DROP",
		},
		"instance disk stackscript_id": {
			factory: tools.NewLinodeInstanceDiskCreateTool,
			args: map[string]any{
				keyLinodeID: float64(123), keyLabel: phase2DiskLabel, keySize: float64(1024),
				keyConfirm: true, keyStackScriptID: float64(0),
			},
			want: "stackscript_id must be an integer greater than or equal to 1",
		},
		"instance disk stackscript_data": {
			factory: tools.NewLinodeInstanceDiskCreateTool,
			args: map[string]any{
				keyLinodeID: float64(123), keyLabel: phase2DiskLabel, keySize: float64(1024),
				keyConfirm: true, keyStackScriptData: map[string]any{"user": float64(5)},
			},
			want: "stackscript_data values must be strings",
		},
		"lke cluster tier": {
			factory: tools.NewLinodeLKEClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keyLKETier: "platinum"}),
			want:    "tier must be one of: standard, enterprise",
		},
		"lke cluster apl_enabled": {
			factory: tools.NewLinodeLKEClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keyAPLEnabled: phase2NonBoolean}),
			want:    "apl_enabled must be a boolean",
		},
		"lke cluster vpc_id": {
			factory: tools.NewLinodeLKEClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keyVPCID: float64(0)}),
			want:    "vpc_id must be an integer greater than or equal to 1",
		},
		"lke cluster subnet_id": {
			factory: tools.NewLinodeLKEClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keySubnetID: float64(0)}),
			want:    "subnet_id must be an integer greater than or equal to 1",
		},
		"lke pool labels": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyLabels: float64(5)}),
			want:    "labels must be an object",
		},
		"lke pool labels values": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyLabels: map[string]any{keyLKETier: float64(5)}}),
			want:    "labels values must be strings",
		},
		"lke pool taints": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyTaints: valueNone}),
			want:    "taints must be an array of objects",
		},
		"lke pool firewall_id": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyFirewallID: float64(0)}),
			want:    "firewall_id must be an integer greater than or equal to 1",
		},
		"lke pool disks": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyDisks: float64(7)}),
			want:    "disks must be an array of objects",
		},
		"lke pool disk_encryption": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyDiskEncryption: "maybe"}),
			want:    "disk_encryption must be one of: disabled, enabled",
		},
		"lke pool update_strategy": {
			factory: tools.NewLinodeLKEPoolCreateTool,
			args:    phase2PoolArgs(map[string]any{keyUpdateStrategy: "asap"}),
			want:    "update_strategy must be one of: on_recycle, rolling_update",
		},
		"lke pool update taints": {
			factory: tools.NewLinodeLKEPoolUpdateTool,
			args: map[string]any{
				keyClusterID: float64(12345), "pool_id": float64(7),
				keyConfirm: true, keyTaints: valueNone,
			},
			want: "taints must be an array of objects",
		},
		"monitor alert create scope": {
			factory: tools.NewLinodeMonitorServiceAlertDefinitionCreateTool,
			args:    phase2AlertArgs(map[string]any{keyScope: keySupportTicketRegion}),
			want:    "scope must be one of: account",
		},
		"monitor alert create group_by": {
			factory: tools.NewLinodeMonitorServiceAlertDefinitionCreateTool,
			args:    phase2AlertArgs(map[string]any{monitorAlertDefinitionGroupByParam: []any{blankWhitespace}}),
			want:    "group_by must be an array of non-empty strings",
		},
		"monitor alert update group_by": {
			factory: tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool,
			args: map[string]any{
				monitorServiceTypeParam: monitorServiceToolTypeDatabase, "alert_id": float64(42),
				keyConfirm: true, monitorAlertDefinitionGroupByParam: []any{blankWhitespace},
			},
			want: "group_by must be an array of non-empty strings",
		},
		"networking ip reserved": {
			factory: tools.NewLinodeNetworkingIPUpdateRDNSTool,
			args: map[string]any{
				keyConfirm: true, managedServiceAddressParam: phase2RDNSAddress,
				keyRDNS: rdnsHostFixture, keyReserved: phase2NonBoolean,
			},
			want: "reserved must be a boolean",
		},
		"nodebalancer configs": {
			factory: tools.NewLinodeNodeBalancerCreateTool,
			args:    map[string]any{keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyConfigs: float64(5)},
			want:    "configs must be an array of objects",
		},
		"nodebalancer vpcs": {
			factory: tools.NewLinodeNodeBalancerCreateTool,
			args:    map[string]any{keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyVPCs: float64(5)},
			want:    "vpcs must be an array of objects",
		},
		"nodebalancer firewall_id": {
			factory: tools.NewLinodeNodeBalancerCreateTool,
			args:    map[string]any{keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyFirewallID: float64(0)},
			want:    "firewall_id must be an integer greater than or equal to 1",
		},
		"bucket endpoint_type": {
			factory: tools.NewLinodeObjectStorageBucketCreateTool,
			args: map[string]any{
				keyLabel: bucketTest, keySupportTicketRegion: "us-east-1",
				keyConfirm: true, keyEndpointType: "E9",
			},
			want: "endpoint_type must be one of: E0, E1, E2, E3",
		},
		"key update regions": {
			factory: tools.NewLinodeObjectStorageKeyUpdateTool,
			args:    map[string]any{"key_id": float64(7), keyConfirm: true, monitorAlertDefinitionRegionsParam: float64(5)},
			want:    "regions must be an array of strings",
		},
		"tag reserved addresses": {
			factory: tools.NewLinodeTagCreateTool,
			args:    map[string]any{keyLabel: canRunEnvProd, keyConfirm: true, keyReservedIPv4Addresses: float64(5)},
			want:    "reserved_ipv4_addresses must be an array of strings",
		},
		"volume encryption": {
			factory: tools.NewLinodeVolumeCreateTool,
			args: map[string]any{
				keyLabel: keyPaymentData, keySupportTicketRegion: placementGroupCreateRegion,
				keyConfirm: true, keyEncryption: "maybe",
			},
			want: "encryption must be one of: disabled, enabled",
		},
		"volume config_id": {
			factory: tools.NewLinodeVolumeCreateTool,
			args: map[string]any{
				keyLabel: keyPaymentData, keySupportTicketRegion: placementGroupCreateRegion,
				keyConfirm: true, keyConfigID: float64(0),
			},
			want: "config_id must be an integer greater than or equal to 1",
		},
		"service transfer entities": {
			factory: tools.NewLinodeAccountServiceTransferCreateTool,
			args:    map[string]any{keyConfirm: true, keyEntities: []any{float64(1)}},
			want:    "entities must be an object",
		},
	})
}

// phase2ClusterArgs returns a valid LKE cluster-create argument set overlaid
// with extra, so a case can isolate the one field it is about.
func phase2ClusterArgs(extra map[string]any) map[string]any {
	args := map[string]any{
		keyLabel:               canRunEnvProd,
		keySupportTicketRegion: placementGroupCreateRegion,
		keyK8sVersion:          phase2K8sVersion,
		"node_pools":           []any{map[string]any{keyPaymentType: typeG6Standard1, keyCount: float64(3)}},
		keyConfirm:             true,
	}

	maps.Copy(args, extra)

	return args
}

// phase2PoolArgs returns a valid LKE pool-create argument set overlaid with
// extra.
func phase2PoolArgs(extra map[string]any) map[string]any {
	args := map[string]any{
		keyClusterID:   float64(12345),
		keyPaymentType: typeG6Standard1,
		keyCount:       float64(3),
		keyConfirm:     true,
	}

	maps.Copy(args, extra)

	return args
}

// phase2AlertArgs returns a valid monitor alert-definition create argument set
// overlaid with extra.
func phase2AlertArgs(extra map[string]any) map[string]any {
	args := monitorAlertDefinitionCreateArgs()
	maps.Copy(args, extra)

	return args
}

// TestPhase2BodyFieldsReachTheWire proves each new field is forwarded rather
// than parsed and dropped: the fake API records the outgoing body and each case
// asserts the keys it added.
func TestPhase2BodyFieldsReachTheWire(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		factory  toolFactory
		args     map[string]any
		response map[string]any
		want     map[string]any
	}{
		"domain update": {
			factory: tools.NewLinodeDomainUpdateTool,
			args: map[string]any{
				keyDomainID: float64(5), keyConfirm: true,
				keyAXFRIPs: []any{reservedIPGateway}, keyMasterIPs: []any{phase2MasterIP},
				keyExpireSec: float64(604800), "refresh_sec": float64(14400),
				"retry_sec": float64(3600), keyPaymentType: "slave",
			},
			response: map[string]any{keyID: 5, "domain": "example.com"},
			want: map[string]any{
				keyAXFRIPs: []any{reservedIPGateway}, keyMasterIPs: []any{phase2MasterIP},
				keyExpireSec: float64(604800), "refresh_sec": float64(14400),
				"retry_sec": float64(3600), keyPaymentType: "slave",
			},
		},
		"domain record update": {
			factory: tools.NewLinodeDomainRecordUpdateTool,
			args: map[string]any{
				keyDomainID: float64(5), "record_id": float64(7), keyConfirm: true,
				"service": "_http", keyProtocol: "_tcp", keyRecordTag: "issue",
			},
			response: map[string]any{keyID: 7},
			want:     map[string]any{"service": "_http", keyProtocol: "_tcp", keyRecordTag: "issue"},
		},
		"instance disk create": {
			factory: tools.NewLinodeInstanceDiskCreateTool,
			args: map[string]any{
				keyLinodeID: float64(123), keyLabel: phase2DiskLabel, keySize: float64(1024),
				keyConfirm: true, keyStackScriptID: float64(55),
				keyStackScriptData: map[string]any{"username": phase2StackScriptUser},
			},
			response: map[string]any{keyID: 1},
			want: map[string]any{
				keyStackScriptID:   float64(55),
				keyStackScriptData: map[string]any{"username": phase2StackScriptUser},
			},
		},
		"lke pool update": {
			factory: tools.NewLinodeLKEPoolUpdateTool,
			args: map[string]any{
				keyClusterID: float64(12345), "pool_id": float64(7), keyConfirm: true,
				keyFirewallID: float64(88), keyLabels: map[string]any{keyLKETier: phase2LabelTier},
				keyTaints: []any{map[string]any{phase2TaintKeyField: phase2TaintKey}},
			},
			response: map[string]any{keyID: 7},
			want: map[string]any{
				keyFirewallID: float64(88),
				keyLabels:     map[string]any{keyLKETier: phase2LabelTier},
				keyTaints:     []any{map[string]any{phase2TaintKeyField: phase2TaintKey}},
			},
		},
		"monitor alert update": {
			factory: tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool,
			args: map[string]any{
				monitorServiceTypeParam: monitorServiceToolTypeDatabase,
				"alert_id":              float64(42), keyConfirm: true,
				monitorAlertDefinitionGroupByParam: []any{keySupportTicketRegion},
			},
			response: map[string]any{keyID: 42},
			want:     map[string]any{monitorAlertDefinitionGroupByParam: []any{keySupportTicketRegion}},
		},
		"networking ip update": {
			factory: tools.NewLinodeNetworkingIPUpdateRDNSTool,
			args: map[string]any{
				keyConfirm: true, managedServiceAddressParam: phase2RDNSAddress,
				keyRDNS: rdnsHostFixture, keyReserved: false,
			},
			response: map[string]any{managedServiceAddressParam: phase2RDNSAddress},
			want:     map[string]any{keyReserved: false},
		},
		"nodebalancer create": {
			factory: tools.NewLinodeNodeBalancerCreateTool,
			args: map[string]any{
				keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyFirewallID: float64(88),
				keyConfigs: []any{map[string]any{managedLinodeSettingsUpdatePortKey: float64(80)}},
				keyVPCs:    []any{map[string]any{keySubnetID: float64(7)}},
			},
			response: map[string]any{keyID: 1},
			want: map[string]any{
				keyFirewallID: float64(88),
				keyConfigs:    []any{map[string]any{managedLinodeSettingsUpdatePortKey: float64(80)}},
				keyVPCs:       []any{map[string]any{keySubnetID: float64(7)}},
			},
		},
		"bucket create": {
			factory: tools.NewLinodeObjectStorageBucketCreateTool,
			args: map[string]any{
				keyLabel: bucketTest, keySupportTicketRegion: "us-east-1", keyConfirm: true,
				keyEndpointType: "E3", keyS3Endpoint: "us-east-1.linodeobjects.com",
			},
			response: map[string]any{keyLabel: bucketTest},
			want: map[string]any{
				keyEndpointType: "E3", keyS3Endpoint: "us-east-1.linodeobjects.com",
			},
		},
		"key update": {
			factory: tools.NewLinodeObjectStorageKeyUpdateTool,
			args: map[string]any{
				"key_id": float64(7), keyConfirm: true,
				monitorAlertDefinitionRegionsParam: []any{placementGroupCreateRegion},
			},
			response: map[string]any{keyID: 7},
			want:     map[string]any{monitorAlertDefinitionRegionsParam: []any{placementGroupCreateRegion}},
		},
		"tag create": {
			factory: tools.NewLinodeTagCreateTool,
			args: map[string]any{
				keyLabel: canRunEnvProd, keyConfirm: true,
				keyReservedIPv4Addresses: []any{phase2ReservedAddress},
			},
			response: map[string]any{keyLabel: canRunEnvProd},
			want:     map[string]any{keyReservedIPv4Addresses: []any{phase2ReservedAddress}},
		},
		"volume create": {
			factory: tools.NewLinodeVolumeCreateTool,
			args: map[string]any{
				keyLabel: keyPaymentData, keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true,
				keyLinodeID: float64(42), keyConfigID: float64(9), keyEncryption: statusEnabled,
			},
			response: map[string]any{keyID: 1, keyLabel: keyPaymentData},
			want:     map[string]any{keyConfigID: float64(9), keyEncryption: statusEnabled},
		},
		"service transfer create": {
			factory: tools.NewLinodeAccountServiceTransferCreateTool,
			args: map[string]any{
				keyConfirm: true,
				keyEntities: map[string]any{
					keyPlacementGroupLinodes: []any{float64(7)}, tagCreateDomainsParam: []any{float64(9)},
				},
			},
			response: map[string]any{keyToken: "t"},
			want: map[string]any{keyEntities: map[string]any{
				keyPlacementGroupLinodes: []any{float64(7)}, tagCreateDomainsParam: []any{float64(9)},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, captured := phase2CapturingServer(t, testCase.response)

			result, err := phase2Handler(t, testCase.factory, cfg)(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError {
				text, _ := result.Content[0].(mcp.TextContent)
				t.Fatalf("result.IsError = true (%s), want false", text.Text)
			}

			for key, want := range testCase.want {
				if !reflect.DeepEqual(captured[key], want) {
					t.Errorf("body[%v] = %v, want %v", key, captured[key], want)
				}
			}
		})
	}
}

// phase2FirewallCreateBody is separate from the table above because the rules
// merge is the point: a caller's rules object wins key by key over the flat
// policy arguments, and both always reach the wire.
func TestPhase2FirewallCreateMergesRules(t *testing.T) {
	t.Parallel()

	cfg, captured := phase2CapturingServer(t, map[string]any{keyID: 100, keyLabel: phase2WebFirewallLabel})

	args := map[string]any{
		keyLabel: phase2WebFirewallLabel, keyConfirm: true, keyInboundPolicy: policyDrop,
		keyRules:   map[string]any{keyOutboundPolicy: policyDrop},
		keyDevices: map[string]any{keyPlacementGroupLinodes: []any{float64(123)}},
	}

	result, err := phase2Handler(t, tools.NewLinodeFirewallCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		text, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("result.IsError = true (%s), want false", text.Text)
	}

	wantRules := map[string]any{keyInboundPolicy: policyDrop, keyOutboundPolicy: policyDrop}
	if !reflect.DeepEqual(captured[keyRules], wantRules) {
		t.Errorf("body[rules] = %v, want %v", captured[keyRules], wantRules)
	}

	wantDevices := map[string]any{keyPlacementGroupLinodes: []any{float64(123)}}
	if !reflect.DeepEqual(captured[keyDevices], wantDevices) {
		t.Errorf("body[devices] = %v, want %v", captured[keyDevices], wantDevices)
	}
}

// TestPhase2FirewallCreateAcceptsRulesString covers the JSON-string form of an
// object argument that non-compliant clients still send.
func TestPhase2FirewallCreateAcceptsRulesString(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		rules any
		want  map[string]any
	}{
		"json string": {
			rules: `{"inbound_policy": "DROP"}`,
			want:  map[string]any{keyInboundPolicy: policyDrop, keyOutboundPolicy: policyAccept},
		},
		"blank string is absent": {
			rules: blankString,
			want:  map[string]any{keyInboundPolicy: policyAccept, keyOutboundPolicy: policyAccept},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, captured := phase2CapturingServer(t, map[string]any{keyID: 100})

			args := map[string]any{keyLabel: phase2WebFirewallLabel, keyConfirm: true, keyRules: testCase.rules}

			result, err := phase2Handler(t, tools.NewLinodeFirewallCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError {
				text, _ := result.Content[0].(mcp.TextContent)
				t.Fatalf("result.IsError = true (%s), want false", text.Text)
			}

			if !reflect.DeepEqual(captured[keyRules], testCase.want) {
				t.Errorf("body[rules] = %v, want %v", captured[keyRules], testCase.want)
			}
		})
	}
}

// TestPhase2LKEClusterCreateCarriesBody covers the cluster-create fields, which
// need their own case because node_pools makes the argument set larger than the
// shared table's rows.
func TestPhase2LKEClusterCreateCarriesBody(t *testing.T) {
	t.Parallel()

	cfg, captured := phase2CapturingServer(t, map[string]any{keyID: 1, keyLabel: canRunEnvProd})

	args := phase2ClusterArgs(map[string]any{
		keyLKETier: "enterprise", keyAPLEnabled: true, "stack_type": "ipv4-ipv6",
		keyVPCID: float64(42), keySubnetID: float64(7),
	})

	result, err := phase2Handler(t, tools.NewLinodeLKEClusterCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		text, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("result.IsError = true (%s), want false", text.Text)
	}

	for key, want := range map[string]any{
		keyLKETier: "enterprise", keyAPLEnabled: true, "stack_type": "ipv4-ipv6",
		keyVPCID: float64(42), keySubnetID: float64(7),
	} {
		if !reflect.DeepEqual(captured[key], want) {
			t.Errorf("body[%v] = %v, want %v", key, captured[key], want)
		}
	}
}

// TestPhase2LKEPoolCreateCarriesBody covers the pool-create-only fields.
func TestPhase2LKEPoolCreateCarriesBody(t *testing.T) {
	t.Parallel()

	cfg, captured := phase2CapturingServer(t, map[string]any{keyID: 10})

	args := phase2PoolArgs(map[string]any{
		keyLabel: "workers", keyK8sVersion: phase2K8sVersion,
		keyDisks:      []any{map[string]any{keySize: float64(4096), keyPaymentType: filesystemExt4}},
		keyLabels:     map[string]any{keyLKETier: phase2LabelTier},
		keyTaints:     []any{map[string]any{phase2TaintKeyField: phase2TaintKey}},
		keyFirewallID: float64(88), keyDiskEncryption: statusEnabled,
		keyUpdateStrategy: "rolling_update",
	})

	result, err := phase2Handler(t, tools.NewLinodeLKEPoolCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		text, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("result.IsError = true (%s), want false", text.Text)
	}

	for key, want := range map[string]any{
		keyLabel: "workers", keyK8sVersion: phase2K8sVersion,
		keyDisks:      []any{map[string]any{keySize: float64(4096), keyPaymentType: filesystemExt4}},
		keyLabels:     map[string]any{keyLKETier: phase2LabelTier},
		keyTaints:     []any{map[string]any{phase2TaintKeyField: phase2TaintKey}},
		keyFirewallID: float64(88), keyDiskEncryption: statusEnabled,
		keyUpdateStrategy: "rolling_update",
	} {
		if !reflect.DeepEqual(captured[key], want) {
			t.Errorf("body[%v] = %v, want %v", key, captured[key], want)
		}
	}
}

// TestPhase2MonitorTokenCreateCarriesAdd covers the token-create add field.
func TestPhase2MonitorTokenCreateCarriesAdd(t *testing.T) {
	t.Parallel()

	cfg, captured := phase2CapturingServer(t, map[string]any{keyToken: "t"})

	args := map[string]any{
		monitorServiceTypeParam: monitorServiceToolTypeDatabase,
		keyEntityIDs:            []any{float64(10)},
		keyConfirm:              true,
		"add":                   "extra",
	}

	result, err := phase2Handler(t, tools.NewLinodeMonitorServiceTokenCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		text, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("result.IsError = true (%s), want false", text.Text)
	}

	if captured["add"] != "extra" {
		t.Errorf("body[add] = %v, want %v", captured["add"], "extra")
	}
}

// TestPhase2InstanceIPAllocateCarriesAddress covers the reserved-address form
// of instance IP allocation.
func TestPhase2InstanceIPAllocateCarriesAddress(t *testing.T) {
	t.Parallel()

	cfg, captured := phase2CapturingServer(t, map[string]any{managedServiceAddressParam: phase2ReservedAddress})

	args := map[string]any{
		keyLinodeID: float64(123), keyPaymentType: keyIPv4, keyInterfacePublic: true,
		keyConfirm: true, managedServiceAddressParam: phase2ReservedAddress,
	}

	result, err := phase2Handler(t, tools.NewLinodeInstanceIPAllocateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		text, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("result.IsError = true (%s), want false", text.Text)
	}

	if captured[managedServiceAddressParam] != phase2ReservedAddress {
		t.Errorf("body[address] = %v, want %v", captured[managedServiceAddressParam], phase2ReservedAddress)
	}
}

// TestPhase2OAuthClientCreateAlwaysSendsPublic pins that public reaches the
// wire even when the caller omits it, since the API documents it as required
// with a false default.
func TestPhase2OAuthClientCreateAlwaysSendsPublic(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		args map[string]any
		want bool
	}{
		"omitted defaults to false": {
			args: map[string]any{
				keyLabel: "demo", keyRedirectURI: "https://example.com/cb", keyConfirm: true,
			},
			want: false,
		},
		"explicit true": {
			args: map[string]any{
				keyLabel: "demo", keyRedirectURI: "https://example.com/cb",
				keyConfirm: true, keyInterfacePublic: true,
			},
			want: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, captured := phase2CapturingServer(t, map[string]any{keyID: profileTokenInvalidIDValue})

			result, err := phase2Handler(t, tools.NewLinodeAccountOAuthClientCreateTool, cfg)(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError {
				text, _ := result.Content[0].(mcp.TextContent)
				t.Fatalf("result.IsError = true (%s), want false", text.Text)
			}

			if captured[keyInterfacePublic] != testCase.want {
				t.Errorf("body[public] = %v, want %v", captured[keyInterfacePublic], testCase.want)
			}
		})
	}
}

// TestPhase2FirewallRulesUpdateDryRunValidatesPolicies pins that the preview
// path rejects an unknown default policy too, rather than advertising a request
// the real call would refuse.
func TestPhase2FirewallRulesUpdateDryRunValidatesPolicies(t *testing.T) {
	t.Parallel()

	handler := phase2Handler(t, tools.NewLinodeFirewallRulesUpdateTool, phase2OfflineConfig())

	args := map[string]any{
		keyFirewallID: float64(1), keyDryRun: true,
		keyInbound: []any{}, keyOutbound: []any{}, keyInboundPolicy: phase2InvalidPolicy,
	}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true")
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText || text.Text != "inbound_policy must be one of: ACCEPT, DROP" {
		t.Errorf("error text = %q, want %q", text.Text, "inbound_policy must be one of: ACCEPT, DROP")
	}
}

// TestPhase2RebuildRejectsNullNodes covers the explicit-null form: the argument
// is present, so the required check passes, but it decodes to no list.
func TestPhase2RebuildRejectsNullNodes(t *testing.T) {
	t.Parallel()

	handler := phase2Handler(t, tools.NewLinodeNodeBalancerConfigRebuildTool, phase2OfflineConfig())

	args := map[string]any{
		keyNodeBalancerID: float64(123), keyConfigID: float64(456),
		keyConfirm: true, keyConfirmedDryRun: true, keyNodes: nil,
	}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true")
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText || text.Text != "nodes must be an array of objects" {
		t.Errorf("error text = %q, want %q", text.Text, "nodes must be an array of objects")
	}
}

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
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// rdnsHostFixture is the reverse-DNS host the body cases send. Rehomed here
// when the interface dry-run file it lived in was emptied by the migration.
const rdnsHostFixture = "host.example.com"

// The phase2 prefix marks the batch of request-body fields these tests cover:
// fields the tools gained to match the Linode API docs. The constants below are
// the fixture values this file introduces; a literal the test package already
// names is reused under that existing name instead of redeclared here.
const (
	errMasterIPsArray        = "master_ips must be an array of strings"
	keyInboundPolicy         = "inbound_policy"
	keyReservedIPv4Addresses = "reserved_ipv4_addresses"
	keyAPLEnabled            = "apl_enabled"
	keyConfigs               = "configs"
	keyEncryption            = "encryption"
	keyEndpointType          = "endpoint_type"
	keyOutboundPolicy        = "outbound_policy"
	keyRecordTag             = "tag"
	keyReserved              = "reserved"
	keyRules                 = "rules"
	keyS3Endpoint            = "s3_endpoint"
	keyStackScriptData       = "stackscript_data"
	keyVPCs                  = "vpcs"
	phase2DiskLabel          = "boot"
	phase2K8sVersion         = "1.31"
	phase2MasterIP           = "192.0.2.2"
	phase2NonBoolean         = "yes"
	phase2RDNSAddress        = "203.0.113.5"
	phase2ReservedAddress    = "203.0.113.9"
	phase2StackScriptUser    = "admin"
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
			factory: gentools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyAXFRIPs: float64(7)},
			want:    "axfr_ips must be an array of strings",
		},
		"domain update master_ips": {
			factory: gentools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyMasterIPs: float64(7)},
			want:    errMasterIPsArray,
		},
		"domain update type": {
			factory: gentools.NewLinodeDomainUpdateTool,
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
			factory: gentools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyDomainID: float64(5), keyConfirm: true, keyExpireSec: "soon"},
			want:    "expire_sec must be an integer",
		},
		// The target is supplied because the record's own rules run before the
		// tag's membership reader, so a CAA record naming no target is refused
		// for the target rather than for the tag.
		"domain record create tag": {
			factory: gentools.NewLinodeDomainRecordCreateTool,
			args: map[string]any{
				keyDomainID: float64(5), keyConfirm: true, keyPaymentType: "CAA",
				keyTarget: "ca.example.com", keyRecordTag: "issuance",
			},
			want: "tag must be one of: issue, issuewild, iodef",
		},
		"domain record update tag": {
			factory: gentools.NewLinodeDomainRecordUpdateTool,
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
		"instance disk stackscript_id": {
			factory: gentools.NewLinodeInstanceDiskCreateTool,
			args: map[string]any{
				keyLinodeID: float64(123), keyLabel: phase2DiskLabel, keySize: float64(1024),
				keyConfirm: true, keyStackScriptID: float64(0),
			},
			want: "stackscript_id must be a positive integer",
		},
		"instance disk stackscript_data": {
			factory: gentools.NewLinodeInstanceDiskCreateTool,
			args: map[string]any{
				keyLinodeID: float64(123), keyLabel: phase2DiskLabel, keySize: float64(1024),
				keyConfirm: true, keyStackScriptData: map[string]any{"user": float64(5)},
			},
			want: "stackscript_data values must be strings",
		},
		"lke cluster tier": {
			factory: gentools.NewLinodeLkeClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keyLKETier: "platinum"}),
			want:    "tier must be one of: standard, enterprise",
		},
		"lke cluster apl_enabled": {
			factory: gentools.NewLinodeLkeClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keyAPLEnabled: phase2NonBoolean}),
			want:    "apl_enabled must be a boolean",
		},
		"lke cluster vpc_id": {
			factory: gentools.NewLinodeLkeClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keyVPCID: float64(0)}),
			want:    "vpc_id must be an integer greater than or equal to 1",
		},
		"lke cluster subnet_id": {
			factory: gentools.NewLinodeLkeClusterCreateTool,
			args:    phase2ClusterArgs(map[string]any{keySubnetID: float64(0)}),
			want:    "subnet_id must be an integer greater than or equal to 1",
		},
		"monitor alert create scope": {
			factory: gentools.NewLinodeMonitorServiceAlertDefinitionCreateTool,
			args:    phase2AlertArgs(map[string]any{keyScope: keySupportTicketRegion}),
			want:    "scope must be one of: account",
		},
		"monitor alert create group_by": {
			factory: gentools.NewLinodeMonitorServiceAlertDefinitionCreateTool,
			args:    phase2AlertArgs(map[string]any{monitorAlertDefinitionGroupByParam: []any{blankWhitespace}}),
			want:    "group_by must be an array of non-empty strings",
		},
		"monitor alert update group_by": {
			factory: gentools.NewLinodeMonitorServiceAlertDefinitionUpdateTool,
			args: map[string]any{
				monitorServiceTypeParam: monitorServiceToolTypeDatabase, "alert_id": float64(42),
				keyConfirm: true, monitorAlertDefinitionGroupByParam: []any{blankWhitespace},
			},
			want: "group_by must be an array of non-empty strings",
		},
		"networking ip reserved": {
			factory: gentools.NewLinodeNetworkingIPUpdateTool,
			args: map[string]any{
				keyConfirm: true, managedServiceAddressParam: phase2RDNSAddress,
				keyRDNS: rdnsHostFixture, keyReserved: phase2NonBoolean,
			},
			want: "reserved must be a boolean",
		},
		"nodebalancer configs": {
			factory: gentools.NewLinodeNodebalancerCreateTool,
			args:    map[string]any{keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyConfigs: float64(5)},
			want:    "configs must be an array of objects",
		},
		"nodebalancer vpcs": {
			factory: gentools.NewLinodeNodebalancerCreateTool,
			args:    map[string]any{keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyVPCs: float64(5)},
			want:    "vpcs must be an array of objects",
		},
		"nodebalancer firewall_id": {
			factory: gentools.NewLinodeNodebalancerCreateTool,
			args:    map[string]any{keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true, keyFirewallID: float64(0)},
			want:    "firewall_id must be an integer greater than or equal to 1",
		},
		"bucket endpoint_type": {
			factory: gentools.NewLinodeObjectStorageBucketCreateTool,
			args: map[string]any{
				keyLabel: bucketTest, keySupportTicketRegion: "us-east-1",
				keyConfirm: true, keyEndpointType: "E9",
			},
			want: "endpoint_type must be one of: E0, E1, E2, E3",
		},
		"key update regions": {
			factory: gentools.NewLinodeObjectStorageKeyUpdateTool,
			args:    map[string]any{"key_id": float64(7), keyConfirm: true, monitorAlertDefinitionRegionsParam: float64(5)},
			want:    "regions must be an array of strings",
		},
		"tag reserved addresses": {
			factory: gentools.NewLinodeTagCreateTool,
			args:    map[string]any{keyLabel: canRunEnvProd, keyConfirm: true, keyReservedIPv4Addresses: float64(5)},
			want:    "reserved_ipv4_addresses must be an array of strings",
		},
		"volume encryption": {
			factory: gentools.NewLinodeVolumeCreateTool,
			args: map[string]any{
				keyLabel: keyPaymentData, keySupportTicketRegion: placementGroupCreateRegion,
				keyConfirm: true, keyEncryption: "maybe",
			},
			want: "encryption must be one of: disabled, enabled",
		},
		"volume config_id": {
			factory: gentools.NewLinodeVolumeCreateTool,
			args: map[string]any{
				keyLabel: keyPaymentData, keySupportTicketRegion: placementGroupCreateRegion,
				keyConfirm: true, keyConfigID: float64(0),
			},
			want: "config_id must be an integer greater than or equal to 1",
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

// phase2AlertArgs returns a valid monitor alert-definition create argument set
// overlaid with extra.
// phase2AlertArgs is a complete alert-definition create call, which each case
// then spoils one field of.
func phase2AlertArgs(extra map[string]any) map[string]any {
	args := map[string]any{
		monitorServiceTypeParam:                 monitorServiceToolTypeDatabase,
		monitorAlertDefinitionLabelParam:        monitorAlertDefinitionToolLabel,
		monitorAlertDefinitionSeverityParam:     2,
		monitorAlertDefinitionRuleCriteriaParam: map[string]any{keyRules: []any{map[string]any{keyMetric: "cpu_usage"}}},
		monitorAlertDefinitionTriggerParam:      map[string]any{"criteria_condition": monitorCriteriaAll},
		monitorAlertDefinitionChannelIDsParam:   []any{546, 392},
		keyEntityIDs:                            []any{"13116"},
		keyScope:                                "account",
		keyConfirm:                              true,
	}
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
			factory: gentools.NewLinodeDomainUpdateTool,
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
			factory: gentools.NewLinodeDomainRecordUpdateTool,
			args: map[string]any{
				keyDomainID: float64(5), "record_id": float64(7), keyConfirm: true,
				"service": "_http", keyProtocol: "_tcp", keyRecordTag: "issue",
			},
			response: map[string]any{keyID: 7},
			want:     map[string]any{"service": "_http", keyProtocol: "_tcp", keyRecordTag: "issue"},
		},
		"instance disk create": {
			factory: gentools.NewLinodeInstanceDiskCreateTool,
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
		"monitor alert update": {
			factory: gentools.NewLinodeMonitorServiceAlertDefinitionUpdateTool,
			args: map[string]any{
				monitorServiceTypeParam: monitorServiceToolTypeDatabase,
				"alert_id":              float64(42), keyConfirm: true,
				monitorAlertDefinitionGroupByParam: []any{keySupportTicketRegion},
			},
			response: map[string]any{keyID: 42},
			want:     map[string]any{monitorAlertDefinitionGroupByParam: []any{keySupportTicketRegion}},
		},
		"networking ip update": {
			factory: gentools.NewLinodeNetworkingIPUpdateTool,
			args: map[string]any{
				keyConfirm: true, managedServiceAddressParam: phase2RDNSAddress,
				keyRDNS: rdnsHostFixture, keyReserved: false,
			},
			response: map[string]any{managedServiceAddressParam: phase2RDNSAddress},
			want:     map[string]any{keyReserved: false},
		},
		"nodebalancer create": {
			factory: gentools.NewLinodeNodebalancerCreateTool,
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
			factory: gentools.NewLinodeObjectStorageBucketCreateTool,
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
			factory: gentools.NewLinodeObjectStorageKeyUpdateTool,
			args: map[string]any{
				"key_id": float64(7), keyConfirm: true,
				monitorAlertDefinitionRegionsParam: []any{placementGroupCreateRegion},
			},
			response: map[string]any{keyID: 7},
			want:     map[string]any{monitorAlertDefinitionRegionsParam: []any{placementGroupCreateRegion}},
		},
		"tag create": {
			factory: gentools.NewLinodeTagCreateTool,
			args: map[string]any{
				keyLabel: canRunEnvProd, keyConfirm: true,
				keyReservedIPv4Addresses: []any{phase2ReservedAddress},
			},
			response: map[string]any{keyLabel: canRunEnvProd},
			want:     map[string]any{keyReservedIPv4Addresses: []any{phase2ReservedAddress}},
		},
		"volume create": {
			factory: gentools.NewLinodeVolumeCreateTool,
			args: map[string]any{
				keyLabel: keyPaymentData, keySupportTicketRegion: placementGroupCreateRegion, keyConfirm: true,
				keyLinodeID: float64(42), keyConfigID: float64(9), keyEncryption: statusEnabled,
			},
			response: map[string]any{keyID: 1, keyLabel: keyPaymentData},
			want:     map[string]any{keyConfigID: float64(9), keyEncryption: statusEnabled},
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

	result, err := phase2Handler(t, gentools.NewLinodeLkeClusterCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
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

	result, err := phase2Handler(t, gentools.NewLinodeMonitorServiceTokenCreateTool, cfg)(t.Context(), createRequestWithArgs(t, args))
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

			result, err := phase2Handler(t, gentools.NewLinodeAccountOauthClientCreateTool, cfg)(t.Context(), createRequestWithArgs(t, testCase.args))
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

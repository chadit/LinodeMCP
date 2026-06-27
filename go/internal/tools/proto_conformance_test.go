package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestInstanceProtoCanonicalOutput pins the proto-canonical serialization the
// instance read handlers emit. The Python instance_to_response_dict output is
// verified identical for the same input, so these invariants keep the two
// implementations in lockstep: a repeated field is always present (here,
// interfaces), and an unset message field is omitted (here, last_successful).
func TestInstanceProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"id":123,"label":"x","status":"running",` +
		`"specs":{},"alerts":{},"backups":{"schedule":{}}}`

	var inst linodev1.Instance
	if err := protojson.Unmarshal([]byte(fixture), &inst); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&inst)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["interfaces"]; !present {
		t.Error("interfaces must always be present (proto repeated under EmitDefaultValues)")
	}

	backups, isObject := out["backups"].(map[string]any)
	if !isObject {
		t.Fatal("backups missing or not an object")
	}

	if _, present := backups["last_successful"]; present {
		t.Error("last_successful must be omitted when unset (proto message presence)")
	}
}

// TestVolumeProtoCanonicalOutput pins the proto-canonical serialization the
// volume_get handler emits: linode_id and linode_label are omitted when the
// volume is detached (optional message fields unset), matching the Python
// _volume_to_dict output for the same input.
func TestVolumeProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const detached = `{"id":5,"label":"v","status":"active","size":20,` +
		`"region":"us-east","filesystem_path":"/dev/disk/by-id/x"}`

	var vol linodev1.Volume
	if err := protojson.Unmarshal([]byte(detached), &vol); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&linodev1.VolumeGetResponse{Volume: &vol})
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out struct {
		Volume map[string]any `json:"volume"`
	}
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out.Volume["linode_id"]; present {
		t.Error("linode_id must be omitted when the volume is detached (optional field unset)")
	}

	if _, present := out.Volume["filesystem_path"]; !present {
		t.Error("filesystem_path must be present")
	}
}

// TestVpcProtoCanonicalOutput pins the proto-canonical serialization the vpc_get
// handler emits: subnets is always present (a repeated field), and the nested
// subnet / linode / interface shape round-trips. The Python vpc_get output (a
// raw passthrough of the same API fields) must match this for the modeled set.
func TestVpcProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"id":7,"label":"net","description":"","region":"us-east",` +
		`"subnets":[{"id":1,"label":"sub","ipv4":"10.0.0.0/24","linodes":` +
		`[{"id":9,"interfaces":[{"id":3,"active":true,"config_id":5}]}],` +
		`"created":"c","updated":"u"}],"created":"c","updated":"u"}`

	var vpc linodev1.Vpc
	if err := protojson.Unmarshal([]byte(fixture), &vpc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&vpc)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	subnets, isArray := out["subnets"].([]any)
	if !isArray {
		t.Fatal("subnets must always be present as an array")
	}

	if len(subnets) != 1 {
		t.Fatalf("expected 1 subnet, got %d", len(subnets))
	}
}

// TestPlacementGroupProtoCanonicalOutput pins the proto-canonical serialization
// the placement group write handlers emit: members is always present (a repeated
// field), and migrations is omitted when the group is not migrating (an optional
// message). The Python placement_group_to_response_dict output matches this.
func TestPlacementGroupProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"id":3,"label":"pg","region":"us-east",` +
		`"placement_group_type":"anti_affinity:local",` +
		`"placement_group_policy":"strict","is_compliant":true}`

	var group linodev1.PlacementGroup
	if err := protojson.Unmarshal([]byte(fixture), &group); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&group)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["members"]; !present {
		t.Error("members must always be present (proto repeated under EmitDefaultValues)")
	}

	if _, present := out["migrations"]; present {
		t.Error("migrations must be omitted when unset (proto optional message)")
	}
}

// TestImageProtoCanonicalOutput pins the proto-canonical serialization the
// image_get handler emits: expiry and eol are omitted when unset (optional
// strings), and capabilities and tags are always present (repeated). The Python
// image_to_response_dict output matches this for the same input.
func TestImageProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"id":"private/9","label":"l","type":"manual","status":"available"}`

	var image linodev1.Image
	if err := protojson.Unmarshal([]byte(fixture), &image); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&image)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["expiry"]; present {
		t.Error("expiry must be omitted when unset (proto optional string)")
	}

	if _, present := out["capabilities"]; !present {
		t.Error("capabilities must always be present (proto repeated)")
	}
}

// TestNodeBalancerProtoCanonicalOutput pins the proto-canonical serialization the
// nodebalancer_get handler emits: the nested transfer message round-trips with
// in/out/total, and tags is always present (a repeated field). The real API
// always returns transfer, so the fixture includes it; the Python
// nodebalancer_to_response_dict output matches this for the same input.
func TestNodeBalancerProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"id":5,"label":"nb","region":"us-east",` +
		`"hostname":"x.nodebalancer.linode.com","ipv4":"1.2.3.4",` +
		`"transfer":{"in":0.5,"out":0.3,"total":0.8}}`

	var nb linodev1.NodeBalancer
	if err := protojson.Unmarshal([]byte(fixture), &nb); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&nb)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	transfer, isObject := out["transfer"].(map[string]any)
	if !isObject {
		t.Fatal("transfer must round-trip as an object")
	}

	if _, present := transfer["in"]; !present {
		t.Error("transfer.in must be present")
	}

	if _, present := out["tags"]; !present {
		t.Error("tags must always be present (proto repeated)")
	}
}

// TestLKEClusterProtoCanonicalOutput pins the proto-canonical serialization the
// lke_cluster_get handler emits: the nested control_plane round-trips, and tags
// is always present (a repeated field). The real API always returns control_plane,
// so the fixture includes it; the Python lke_cluster_to_response_dict output
// matches this for the same input.
func TestLKEClusterProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"id":7,"label":"k8s","region":"us-east","k8s_version":"1.30",` +
		`"status":"ready","control_plane":{"high_availability":true}}`

	var cluster linodev1.LKECluster
	if err := protojson.Unmarshal([]byte(fixture), &cluster); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&cluster)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	controlPlane, isObject := out["control_plane"].(map[string]any)
	if !isObject {
		t.Fatal("control_plane must round-trip as an object")
	}

	if controlPlane["high_availability"] != true {
		t.Errorf("control_plane.high_availability = %v, want true", controlPlane["high_availability"])
	}

	if _, present := out["tags"]; !present {
		t.Error("tags must always be present (proto repeated)")
	}
}

// TestAccountProtoCanonicalOutput pins the proto-canonical serialization the
// account_get handler emits: capabilities and active_promotions are always present
// (repeated fields), even when the account has none. The Python
// account_to_response_dict output matches this for the same input.
func TestAccountProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"first_name":"a","last_name":"b","email":"e","balance":0}`

	var account linodev1.Account
	if err := protojson.Unmarshal([]byte(fixture), &account); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&account)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["capabilities"]; !present {
		t.Error("capabilities must always be present (proto repeated)")
	}

	if _, present := out["active_promotions"]; !present {
		t.Error("active_promotions must always be present (proto repeated)")
	}
}

// TestAccountSettingsProtoCanonicalOutput pins the proto-canonical serialization
// the account_settings_get handler emits: longview_subscription and object_storage
// are omitted when unset (optional strings), while the bool fields are always
// present. The Python account_settings_to_response_dict output matches this.
func TestAccountSettingsProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"backups_enabled":true,"managed":false,"network_helper":true,` +
		`"interfaces_for_new_linodes":"legacy_config","maintenance_policy":"linode/migrate"}`

	var settings linodev1.AccountSettings
	if err := protojson.Unmarshal([]byte(fixture), &settings); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&settings)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["longview_subscription"]; present {
		t.Error("longview_subscription must be omitted when unset (proto optional string)")
	}

	if _, present := out["backups_enabled"]; !present {
		t.Error("backups_enabled must always be present (proto bool)")
	}
}

// TestAccountTransferProtoCanonicalOutput pins the proto-canonical serialization
// the account_transfer_get handler emits: region_transfers is always present (a
// repeated field), even when the account has none. The Python
// account_transfer_to_response_dict output matches this.
func TestAccountTransferProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"billable":0,"quota":11000,"used":5}`

	var transfer linodev1.AccountTransfer
	if err := protojson.Unmarshal([]byte(fixture), &transfer); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&transfer)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["region_transfers"]; !present {
		t.Error("region_transfers must always be present (proto repeated)")
	}
}

// TestObjectStorageKeyProtoCanonicalOutput pins the proto-canonical serialization
// the object_storage_key_get handler emits: bucket_access and regions are always
// present (repeated fields), even when the key has none. The Python
// object_storage_key_to_response_dict output matches this.
func TestObjectStorageKeyProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	const fixture = `{"label":"k","access_key":"AK","id":7,"limited":true}`

	var key linodev1.ObjectStorageKey
	if err := protojson.Unmarshal([]byte(fixture), &key); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&key)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if _, present := out["bucket_access"]; !present {
		t.Error("bucket_access must always be present (proto repeated)")
	}

	if _, present := out["regions"]; !present {
		t.Error("regions must always be present (proto repeated)")
	}
}

// TestSupportTicketProtoCanonicalOutput pins the proto-canonical serialization of
// the support ticket get handler. The Python support_ticket_to_response_dict output
// is verified identical for the same input: attachments is always present (repeated),
// entity and closed are omitted when unset (message presence / proto optional), and
// the entity id passes through as its JSON value (google.protobuf.Value).
func TestSupportTicketProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	marshal := func(t *testing.T, fixture string) map[string]any {
		t.Helper()

		var ticket linodev1.SupportTicket
		if err := protojson.Unmarshal([]byte(fixture), &ticket); err != nil {
			t.Fatalf("decode fixture: %v", err)
		}

		result, err := tools.MarshalProtoToolResponse(&ticket)
		if err != nil {
			t.Fatalf("MarshalProtoToolResponse: %v", err)
		}

		text, isText := result.Content[0].(mcp.TextContent)
		if !isText {
			t.Fatal("result content is not TextContent")
		}

		var out map[string]any
		if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}

		return out
	}

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshal(t, `{"id":7,"summary":"s","closed":"2024-01-02",`+
			`"attachments":[{"filename":"a.txt","id":1,"size":10}],`+
			`"entity":{"id":555,"label":"web","type":"linode","url":"/x"}}`)

		attachments, isArray := out["attachments"].([]any)
		if !isArray || len(attachments) != 1 {
			t.Fatalf("attachments not a 1-element array: %v", out["attachments"])
		}

		entity, isObject := out["entity"].(map[string]any)
		if !isObject {
			t.Fatal("entity must be present when set (proto message presence)")
		}

		if entity["id"] != float64(555) {
			t.Errorf("entity.id = %v, want 555 (google.protobuf.Value passthrough)", entity["id"])
		}

		if out["closed"] != "2024-01-02" {
			t.Errorf("closed = %v, want 2024-01-02", out["closed"])
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshal(t, `{"id":7,"summary":"s"}`)

		list, isArray := out["attachments"].([]any)
		if !isArray || len(list) != 0 {
			t.Errorf("attachments must be an empty array, got %v", out["attachments"])
		}

		if _, present := out["entity"]; present {
			t.Error("entity must be omitted when unset (proto message presence)")
		}

		if _, present := out["closed"]; present {
			t.Error("closed must be omitted when unset (proto optional)")
		}
	})
}

// TestFirewallDeviceProtoCanonicalOutput pins the recursive FirewallDeviceEntity
// serialization. entity is always present (proto message set from the API), and
// parent_entity is a nullable self-reference: omitted when nil, present and
// recursively shaped when set. The Python firewall_device_to_response_dict mirror
// follows the same omit-when-None rule.
func TestFirewallDeviceProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	marshal := func(t *testing.T, fixture string) map[string]any {
		t.Helper()

		var device linodev1.FirewallDevice
		if err := protojson.Unmarshal([]byte(fixture), &device); err != nil {
			t.Fatalf("decode fixture: %v", err)
		}

		result, err := tools.MarshalProtoToolResponse(&device)
		if err != nil {
			t.Fatalf("MarshalProtoToolResponse: %v", err)
		}

		text, isText := result.Content[0].(mcp.TextContent)
		if !isText {
			t.Fatal("result content is not TextContent")
		}

		var out map[string]any
		if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}

		return out
	}

	t.Run("flat entity", func(t *testing.T) {
		t.Parallel()

		out := marshal(t, `{"id":456,"entity":{"id":123,"label":"web","type":"linode","url":"/x"},`+
			`"created":"2024-01-02","updated":"2024-01-03"}`)

		entity, isObject := out["entity"].(map[string]any)
		if !isObject {
			t.Fatal("entity must be present when set (proto message presence)")
		}

		if entity["label"] != imageUploadTagWeb {
			t.Errorf("entity.label = %v, want web", entity["label"])
		}

		if _, present := entity["parent_entity"]; present {
			t.Error("parent_entity must be omitted when unset (proto message presence)")
		}
	})

	t.Run("nested parent_entity", func(t *testing.T) {
		t.Parallel()

		out := marshal(t, `{"id":456,"entity":{"id":123,"label":"web","type":"linode","url":"/x",`+
			`"parent_entity":{"id":99,"label":"parent","type":"nodebalancer","url":"/p"}}}`)

		entity, isObject := out["entity"].(map[string]any)
		if !isObject {
			t.Fatal("entity must be present")
		}

		parent, isObject := entity["parent_entity"].(map[string]any)
		if !isObject {
			t.Fatal("parent_entity must be present and recursively shaped when set")
		}

		if parent["label"] != "parent" {
			t.Errorf("parent_entity.label = %v, want parent", parent["label"])
		}
	})
}

func marshalLKENodePoolFixture(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var pool linodev1.LKENodePool
	if err := protojson.Unmarshal([]byte(fixture), &pool); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&pool)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	return out
}

func assertLKENodePoolNodes(t *testing.T, out map[string]any) {
	t.Helper()

	nodes, isArray := out["nodes"].([]any)
	if !isArray || len(nodes) != 1 {
		t.Fatalf("nodes not a 1-element array: %v", out["nodes"])
	}

	node, _ := nodes[0].(map[string]any)
	if node["id"] != "node-123" {
		t.Errorf("nodes[0].id = %v, want node-123 (reused LKENode)", node["id"])
	}
}

// TestLKENodePoolProtoCanonicalOutput pins the LKENodePool serialization: repeated
// disks, a nullable autoscaler message (omitted when nil), repeated nodes (reusing
// the LKENode message), and repeated tags. The Python lke_node_pool_to_response_dict
// mirror follows the same omit-when-None / always-list rules.
func TestLKENodePoolProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshalLKENodePoolFixture(t, `{"id":5,"cluster_id":9,"type":"g6-standard-1","count":3,`+
			`"disks":[{"size":50,"type":"ext4"}],`+
			`"autoscaler":{"enabled":true,"min":1,"max":5},`+
			`"nodes":[{"id":"node-123","instance_id":111,"status":"ready"}],`+
			`"tags":["prod"]}`)

		disks, isArray := out["disks"].([]any)
		if !isArray || len(disks) != 1 {
			t.Fatalf("disks not a 1-element array: %v", out["disks"])
		}

		autoscaler, isObject := out["autoscaler"].(map[string]any)
		if !isObject || autoscaler["enabled"] != true {
			t.Errorf("autoscaler missing or enabled != true: %v", out["autoscaler"])
		}

		assertLKENodePoolNodes(t, out)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshalLKENodePoolFixture(t, `{"id":5,"cluster_id":9}`)

		if _, present := out["autoscaler"]; present {
			t.Error("autoscaler must be omitted when unset (proto message presence)")
		}

		for _, key := range []string{"disks", "nodes", "tags"} {
			list, isArray := out[key].([]any)
			if !isArray || len(list) != 0 {
				t.Errorf("%s must be an empty array, got %v", key, out[key])
			}
		}
	})
}

func marshalFirewallFixture(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var firewall linodev1.Firewall
	if err := protojson.Unmarshal([]byte(fixture), &firewall); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&firewall)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	return out
}

func assertFirewallInboundRule(t *testing.T, out map[string]any) {
	t.Helper()

	rules, isObject := out["rules"].(map[string]any)
	if !isObject {
		t.Fatal("rules must be present (proto message presence)")
	}

	inbound, isArray := rules["inbound"].([]any)
	if !isArray || len(inbound) != 1 {
		t.Fatalf("rules.inbound not a 1-element array: %v", rules["inbound"])
	}

	rule, _ := inbound[0].(map[string]any)

	addresses, isObject := rule["addresses"].(map[string]any)
	if !isObject {
		t.Fatal("rule.addresses must be present (proto message presence)")
	}

	ipv4, isArray := addresses["ipv4"].([]any)
	if !isArray || len(ipv4) != 1 {
		t.Errorf("rule.addresses.ipv4 not a 1-element array: %v", addresses["ipv4"])
	}
}

// TestFirewallProtoCanonicalOutput pins the Firewall serialization: a nested
// FirewallRules message carrying repeated FirewallRule (each with a nested
// FirewallAddresses of repeated ipv4/ipv6) plus repeated tags. The Python
// firewall_to_response_dict mirror walks the same nested structure unwrapped.
func TestFirewallProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshalFirewallFixture(t, `{"id":55,"label":"web","status":"enabled",`+
			`"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT",`+
			`"inbound":[{"action":"ACCEPT","protocol":"TCP","ports":"22",`+
			`"addresses":{"ipv4":["0.0.0.0/0"]},"label":"ssh"}]},`+
			`"tags":["prod"]}`)

		if out["label"] != imageUploadTagWeb {
			t.Errorf("label = %v, want web", out["label"])
		}

		assertFirewallInboundRule(t, out)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshalFirewallFixture(t, `{"id":55,"label":"web","rules":{}}`)

		rules, isObject := out["rules"].(map[string]any)
		if !isObject {
			t.Fatal("rules must be present (proto message presence)")
		}

		for _, key := range []string{"inbound", "outbound"} {
			list, isArray := rules[key].([]any)
			if !isArray || len(list) != 0 {
				t.Errorf("rules.%s must be an empty array, got %v", key, rules[key])
			}
		}

		tags, isArray := out["tags"].([]any)
		if !isArray || len(tags) != 0 {
			t.Errorf("tags must be an empty array, got %v", out["tags"])
		}
	})
}

func marshalConfigInterfaceFixture(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var iface linodev1.ConfigInterfaceResponse
	if err := protojson.Unmarshal([]byte(fixture), &iface); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&iface)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	return out
}

// TestConfigInterfaceProtoCanonicalOutput pins the single ConfigInterfaceResponse
// serialization: a nullable ipv4 message (omitted when nil), optional scalars
// (label/ipam_address/subnet_id/vpc_id omitted when unset), and a repeated
// ip_ranges (always a list). The Python config_interface_to_response_dict mirror
// follows the same omit-when-None / always-list rules.
func TestConfigInterfaceProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshalConfigInterfaceFixture(t, `{"id":7,"active":true,"purpose":"vpc",`+
			`"label":"eth0","subnet_id":3,"ipv4":{"vpc":"10.0.0.5"},`+
			`"ip_ranges":["2001:db8::/64"]}`)

		ipv4, isObject := out["ipv4"].(map[string]any)
		if !isObject || ipv4["vpc"] != "10.0.0.5" {
			t.Errorf("ipv4.vpc = %v, want 10.0.0.5", out["ipv4"])
		}

		ranges, isArray := out["ip_ranges"].([]any)
		if !isArray || len(ranges) != 1 {
			t.Errorf("ip_ranges not a 1-element array: %v", out["ip_ranges"])
		}

		if out["label"] != "eth0" {
			t.Errorf("label = %v, want eth0", out["label"])
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshalConfigInterfaceFixture(t, `{"id":7,"active":true,"purpose":"public"}`)

		if _, present := out["ipv4"]; present {
			t.Error("ipv4 must be omitted when unset (proto message presence)")
		}

		if _, present := out["label"]; present {
			t.Error("label must be omitted when unset (proto optional)")
		}

		ranges, isArray := out["ip_ranges"].([]any)
		if !isArray || len(ranges) != 0 {
			t.Errorf("ip_ranges must be an empty array, got %v", out["ip_ranges"])
		}
	})
}

func marshalAccountEventFixture(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var event linodev1.AccountEvent
	if err := protojson.Unmarshal([]byte(fixture), &event); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&event)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	return out
}

// TestAccountEventProtoCanonicalOutput pins the AccountEvent serialization: the
// nullable nested entity/secondary_entity (omitted when nil), optional scalars
// (duration/percent_complete/rate/time_remaining omitted when unset), and the
// passthrough Value entity id (a number or a string). The Python
// account_event_to_response_dict mirror follows the same omit-when-None rules.
func TestAccountEventProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshalAccountEventFixture(t, `{"id":42,"action":"linode_boot",`+
			`"duration":3.5,"percent_complete":100,`+
			`"entity":{"id":55,"label":"web","type":"linode","url":"/v4/linode/instances/55"},`+
			`"secondary_entity":{"id":"img-1","label":"img","type":"image","url":"/v4/images/img-1"}}`)

		entity, isObject := out["entity"].(map[string]any)
		if !isObject || entity["id"] != float64(55) {
			t.Errorf("entity.id = %v, want 55", out["entity"])
		}

		secondary, isObject := out["secondary_entity"].(map[string]any)
		if !isObject || secondary["id"] != "img-1" {
			t.Errorf("secondary_entity.id = %v, want img-1", out["secondary_entity"])
		}

		if out["duration"] != 3.5 {
			t.Errorf("duration = %v, want 3.5", out["duration"])
		}

		if out["percent_complete"] != float64(100) {
			t.Errorf("percent_complete = %v, want 100", out["percent_complete"])
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshalAccountEventFixture(t, `{"id":42,"action":"linode_boot","seen":true}`)

		omitted := []string{
			"entity", "secondary_entity", "duration",
			"percent_complete", "rate", "time_remaining",
		}
		for _, key := range omitted {
			if _, present := out[key]; present {
				t.Errorf("%s must be omitted when unset", key)
			}
		}

		if out["seen"] != true {
			t.Errorf("seen = %v, want true", out["seen"])
		}
	})
}

func marshalInterfaceSettingsFixture(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var settings linodev1.InstanceInterfaceSettings
	if err := protojson.Unmarshal([]byte(fixture), &settings); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&settings)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	return out
}

// TestInstanceInterfaceSettingsProtoCanonicalOutput pins the serialization: a
// nullable default_route message (omitted when nil) carrying optional bools, and
// an optional network_helper bool (omitted when unset). The Python
// instance_interface_settings_to_response_dict mirror follows the same rules.
func TestInstanceInterfaceSettingsProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshalInterfaceSettingsFixture(t,
			`{"default_route":{"ipv4":true},"network_helper":false}`)

		route, isObject := out["default_route"].(map[string]any)
		if !isObject || route["ipv4"] != true {
			t.Errorf("default_route.ipv4 = %v, want true", out["default_route"])
		}

		if _, present := route["ipv6"]; present {
			t.Error("default_route.ipv6 must be omitted when unset (proto optional)")
		}

		if out["network_helper"] != false {
			t.Errorf("network_helper = %v, want false", out["network_helper"])
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshalInterfaceSettingsFixture(t, `{}`)

		if _, present := out["default_route"]; present {
			t.Error("default_route must be omitted when nil (proto message presence)")
		}

		if _, present := out["network_helper"]; present {
			t.Error("network_helper must be omitted when unset (proto optional)")
		}
	})
}

func marshalAccountUserFixture(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var user linodev1.AccountUser
	if err := protojson.Unmarshal([]byte(fixture), &user); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(&user)
	if err != nil {
		t.Fatalf("MarshalProtoToolResponse: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	return out
}

// TestAccountUserProtoCanonicalOutput pins the AccountUser serialization: a
// nullable last_login message (omitted when nil), optional password_created and
// verified_phone_number strings (omitted when unset), and a repeated ssh_keys
// (always a list). The Python account_user_to_response_dict mirror follows the
// same rules.
func TestAccountUserProtoCanonicalOutput(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		out := marshalAccountUserFixture(t, `{"username":"alice","email":"a@b.co",`+
			`"restricted":true,"ssh_keys":["ssh-rsa AAAA"],`+
			`"last_login":{"login_datetime":"2026-01-01T00:00:00","status":"successful"}}`)

		login, isObject := out["last_login"].(map[string]any)
		if !isObject || login["status"] != "successful" {
			t.Errorf("last_login.status = %v, want successful", out["last_login"])
		}

		keys, isArray := out["ssh_keys"].([]any)
		if !isArray || len(keys) != 1 {
			t.Errorf("ssh_keys not a 1-element array: %v", out["ssh_keys"])
		}

		if out["restricted"] != true {
			t.Errorf("restricted = %v, want true", out["restricted"])
		}

		if _, present := out["password_created"]; present {
			t.Error("password_created must be omitted when unset (proto optional)")
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := marshalAccountUserFixture(t, `{"username":"bob"}`)

		if _, present := out["last_login"]; present {
			t.Error("last_login must be omitted when nil (proto message presence)")
		}

		if _, present := out["verified_phone_number"]; present {
			t.Error("verified_phone_number must be omitted when unset (proto optional)")
		}

		keys, isArray := out["ssh_keys"].([]any)
		if !isArray || len(keys) != 0 {
			t.Errorf("ssh_keys must be an empty array, got %v", out["ssh_keys"])
		}
	})
}

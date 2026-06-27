package server_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// newCapabilityTestServer builds a Server with the canonical test config so
// the invariant tests below can iterate the registered tool set.
func newCapabilityTestServer(t *testing.T) *server.Server {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      serverNameTest,
			LogLevel:  logLevelInfo,
			Transport: transportStdio,
			Host:      hostLocalhost,
			Port:      8080,
		},
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenShort},
			},
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	return srv
}

// schemaHasBooleanProp reports whether the input schema declares a property
// named name with type "boolean". Used by the capability-and-confirm
// invariant test to detect whether a tool requires explicit confirmation.
//
// The schema is the mcp-go ToolInputSchema struct; Properties is a
// map[string]any whose entries are JSON-Schema-shaped map[string]any. We
// look for an entry whose nested "type" field is the literal string
// "boolean".
func schemaHasBooleanProp(schema map[string]any, name string) bool {
	entry, found := schema[name]
	if !found {
		return false
	}

	props, isMap := entry.(map[string]any)
	if !isMap {
		return false
	}

	typeVal, isString := props["type"].(string)

	return isString && typeVal == "boolean"
}

// toolSchemaProps returns a registered tool's input-schema properties.
// Programmatically built tools populate the structured InputSchema, but
// proto-backed tools built with NewToolWithRawSchema carry only
// RawInputSchema and leave the structured Properties empty, so fall back to
// parsing the raw JSON schema for those.
func toolSchemaProps(t *testing.T, info *server.ToolInfo) map[string]any {
	t.Helper()

	if len(info.InputSchema.Properties) > 0 {
		return info.InputSchema.Properties
	}

	if len(info.RawInputSchema) == 0 {
		return nil
	}

	var parsed struct {
		Properties map[string]any `json:"properties"`
	}

	if err := json.Unmarshal(info.RawInputSchema, &parsed); err != nil {
		t.Fatalf("parse raw input schema for %s: %v", info.Name, err)
	}

	return parsed.Properties
}

// toolSchemaRequired returns a registered tool's required-field list from
// whichever schema form the tool uses (see toolSchemaProps).
func toolSchemaRequired(t *testing.T, info *server.ToolInfo) []string {
	t.Helper()

	if len(info.InputSchema.Properties) > 0 {
		return info.InputSchema.Required
	}

	if len(info.RawInputSchema) == 0 {
		return nil
	}

	var parsed struct {
		Required []string `json:"required"`
	}

	if err := json.Unmarshal(info.RawInputSchema, &parsed); err != nil {
		t.Fatalf("parse raw input schema for %s: %v", info.Name, err)
	}

	return parsed.Required
}

// TestNoCapabilityUnknownInRegistry enforces the Phase 1+ steady-state
// invariant: every registered tool must carry a real Capability tag. A tool
// landing in the registry with CapUnknown is a tagging bug (developer added
// a tool and forgot the capability, or rebased a stale branch). Phase 1's
// temporary allowlist exempted this; that exemption is gone now and any new
// tool must declare its capability at registration time.
func TestNoCapabilityUnknownInRegistry(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	tools := srv.ToolInfos()
	if len(tools) == 0 {
		t.Fatal("tools is empty")
	}

	var untagged []string

	for _, info := range tools {
		if info.Capability == profiles.CapUnknown {
			untagged = append(untagged, info.Name)
		}
	}

	if len(untagged) != 0 {
		t.Errorf("untagged = %v, want empty", untagged)
	}
}

func TestDeprecatedAccountEntityTransferDeleteToolRemoved(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make([]string, 0, len(srv.Tools()))
	for _, tool := range srv.Tools() {
		names = append(names, tool.Name())
	}

	if slices.Contains(names, "linode_account_entity_transfer_delete") {
		t.Errorf("names should not contain %v", "linode_account_entity_transfer_delete")
	}

	if !slices.Contains(names, "linode_account_service_transfer_delete") {
		t.Errorf("names does not contain %v", "linode_account_service_transfer_delete")
	}
}

// TestCapabilityAndConfirmInvariants enforces the relationship between a
// tool's capability tag and its confirm parameter:
//
//   - CapRead tools must NOT declare confirm (reads never mutate state)
//   - CapWrite, CapDestroy tools MUST declare confirm (mutators always
//     require explicit confirmation)
//   - CapAdmin is exempt: it is a privilege tier (account/child_account
//     scope), orthogonal to mutation. An Admin tool may be a read
//     (linode_managed_credential_get) with no confirm or a mutation
//     (linode_account_update) that carries one. This check runs over the
//     active read-only profile, which excludes Admin tools regardless.
//   - CapMeta is exempt (some meta tools have confirm, some don't)
//   - CapUnknown is exempt (still on the allowlist; gated by the first test)
//
// This test is trivially satisfied in the Setup PR because every tool is
// CapUnknown. Category PRs that assign real capabilities will start
// exercising real cases here.
func TestCapabilityAndConfirmInvariants(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	tools := srv.ToolInfos()
	if len(tools) == 0 {
		t.Fatal("tools is empty")
	}

	for _, info := range tools {
		hasConfirm := schemaHasBooleanProp(info.InputSchema.Properties, "confirm")

		switch info.Capability {
		case profiles.CapRead:
			if hasConfirm {
				t.Error("hasConfirm = true, want false")
			}
		case profiles.CapWrite, profiles.CapDestroy:
			if !hasConfirm {
				t.Error("hasConfirm = false, want true")
			}
		case profiles.CapMeta, profiles.CapUnknown, profiles.CapAdmin:
			// CapMeta tools may either have or omit confirm (some edit state,
			// some are pure reads). CapAdmin is a privilege tier that spans
			// reads and mutations, so confirm is not required here (and this
			// check runs over the default profile, which excludes Admin
			// anyway). CapUnknown is gated by the allowlist test above.
		}
	}
}

// dryRunPendingTools returns the mutating tools (CapWrite/CapDestroy/CapAdmin)
// that do not yet wire the dry_run preview branch. This is a RATCHET, not a
// permanent exemption: as each tool gains its dry_run branch, delete it from
// this set. TestCapabilityAndDryRunInvariants fails in three directions, so
// the set can only shrink:
//
//   - a listed tool that already advertises dry_run fails (stale entry, remove it)
//   - a listed name that matches no registered tool fails (renamed/deleted tool)
//   - an unlisted mutator that lacks dry_run fails (new tool, wire it or list it)
//
// When this set is empty, every Phase 1 mutator advertises dry_run and the
// check collapses into a pure steady-state invariant.
func dryRunPendingTools() map[string]struct{} {
	return map[string]struct{}{}
}

// TestCapabilityAndDryRunInvariants is the Phase 1 dry-run coverage ratchet.
// Every CapWrite/CapDestroy/CapAdmin tool must advertise the dry_run
// parameter unless it sits on the pending allowlist. The allowlist is
// checked for staleness so it can only shrink: listed tools that already
// advertise dry_run, and listed names with no matching tool, both fail.
func TestCapabilityAndDryRunInvariants(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	infos := srv.AllToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	pending := dryRunPendingTools()
	registered := make(map[string]struct{}, len(infos))

	for _, info := range infos {
		registered[info.Name] = struct{}{}

		switch info.Capability {
		case profiles.CapWrite, profiles.CapDestroy, profiles.CapAdmin:
			hasDryRun := schemaHasBooleanProp(toolSchemaProps(t, &info), "dry_run")
			_, isPending := pending[info.Name]

			if hasDryRun != !isPending {
				t.Errorf("tool %s: hasDryRun = %v, want %v", info.Name, hasDryRun, !isPending)
			}
		case profiles.CapRead, profiles.CapMeta, profiles.CapUnknown:
		}
	}

	for name := range pending {
		_, exists := registered[name]
		if !exists {
			t.Error("exists = false, want true")
		}
	}
}

func TestLinodeInstanceStatsToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var found bool

	for _, info := range srv.ToolInfos() {
		if info.Name == "linode_instance_stats_get" {
			found = true

			if info.Capability != profiles.CapRead {
				t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
			}

			if _, ok := info.InputSchema.Properties["linode_id"]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", "linode_id")
			}

			if !slices.Contains(info.InputSchema.Required, "linode_id") {
				t.Errorf("info.InputSchema.Required does not contain %v", "linode_id")
			}
		}
	}

	if !found {
		t.Error("found = false, want true")
	}
}

func TestLinodeNodeBalancerStatsToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var found bool

	for _, info := range srv.ToolInfos() {
		if info.Name == "linode_nodebalancer_stats_get" {
			found = true

			if info.Capability != profiles.CapRead {
				t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
			}

			if _, ok := info.InputSchema.Properties["nodebalancer_id"]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", "nodebalancer_id")
			}

			if !slices.Contains(info.InputSchema.Required, "nodebalancer_id") {
				t.Errorf("info.InputSchema.Required does not contain %v", "nodebalancer_id")
			}
		}
	}

	if !found {
		t.Error("found = false, want true")
	}
}

func TestLinodeNetworkingIPsToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	found := make(map[string]bool, len(infos))

	for _, info := range infos {
		switch info.Name {
		case "linode_networking_ip_list":
			found[info.Name] = true

			if info.Capability != profiles.CapRead {
				t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
			}

			if _, ok := toolSchemaProps(t, &info)["skip_ipv6_rdns"]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", "skip_ipv6_rdns")
			}
		case "linode_networking_ip_get":
			found[info.Name] = true

			if info.Capability != profiles.CapRead {
				t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
			}

			if _, ok := toolSchemaProps(t, &info)["address"]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", "address")
			}
		}
	}

	if !found["linode_networking_ip_list"] {
		t.Error("expected condition to be true")
	}

	if !found["linode_networking_ip_get"] {
		t.Error("expected condition to be true")
	}
}

func TestLinodeFirewallTemplatesToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var foundList bool

	for _, info := range srv.ToolInfos() {
		if info.Name != "linode_firewall_template_list" {
			continue
		}

		foundList = true

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		if _, ok := info.InputSchema.Properties["page"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "page")
		}

		if _, ok := info.InputSchema.Properties["page_size"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "page_size")
		}
	}

	if !foundList {
		t.Error("foundList = false, want true")
	}
}

func TestLinodeFirewallTemplateGetToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var foundGet bool

	for _, info := range srv.ToolInfos() {
		if info.Name != "linode_firewall_template_get" {
			continue
		}

		foundGet = true

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		if _, ok := info.InputSchema.Properties["slug"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "slug")
		}

		if !slices.Contains(info.InputSchema.Required, "slug") {
			t.Errorf("info.InputSchema.Required does not contain %v", "slug")
		}

		if _, ok := info.InputSchema.Properties["page"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "page")
		}

		if _, ok := info.InputSchema.Properties["page_size"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "page_size")
		}
	}

	if !foundGet {
		t.Error("foundGet = false, want true")
	}
}

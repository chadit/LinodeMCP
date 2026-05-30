package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
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
	require.NoError(t, err, "server should construct cleanly")
	require.NotNil(t, srv, "server must not be nil")

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
	require.NotEmpty(t, tools, "server must register at least one tool")

	var untagged []string

	for _, info := range tools {
		if info.Capability == profiles.CapUnknown {
			untagged = append(untagged, info.Name)
		}
	}

	assert.Empty(
		t, untagged,
		"tools registered with CapUnknown (tag them with profiles.CapRead/Write/Destroy/Admin/Meta): %v",
		untagged,
	)
}

// TestCapabilityAndConfirmInvariants enforces the relationship between a
// tool's capability tag and its confirm parameter:
//
//   - CapRead tools must NOT declare confirm (reads never mutate state)
//   - CapWrite, CapDestroy, CapAdmin tools MUST declare confirm (mutators
//     always require explicit confirmation)
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
	require.NotEmpty(t, tools, "server must register at least one tool")

	for _, info := range tools {
		hasConfirm := schemaHasBooleanProp(info.InputSchema.Properties, "confirm")

		switch info.Capability {
		case profiles.CapRead:
			assert.False(
				t, hasConfirm,
				"tool %q is CapRead but declares the confirm parameter; remove confirm or fix capability",
				info.Name,
			)
		case profiles.CapWrite, profiles.CapDestroy, profiles.CapAdmin:
			assert.True(
				t, hasConfirm,
				"tool %q is %s but does not declare the confirm parameter; mutators must require explicit confirmation",
				info.Name, info.Capability,
			)
		case profiles.CapMeta, profiles.CapUnknown:
			// CapMeta tools may either have or omit confirm (some edit state,
			// some are pure reads). CapUnknown is gated by the allowlist test
			// above; this PR ships with every tool here.
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
	require.NotEmpty(t, infos, "server must expose its full tool catalog")

	pending := dryRunPendingTools()
	registered := make(map[string]struct{}, len(infos))

	for _, info := range infos {
		registered[info.Name] = struct{}{}

		switch info.Capability {
		case profiles.CapWrite, profiles.CapDestroy, profiles.CapAdmin:
			hasDryRun := schemaHasBooleanProp(info.InputSchema.Properties, "dry_run")
			_, isPending := pending[info.Name]

			assert.Equalf(
				t, !isPending, hasDryRun,
				"tool %q (%s): advertises dry_run=%v, on pending allowlist=%v. Mutators must advertise dry_run unless explicitly pending; remove it from dryRunPendingTools once wired",
				info.Name, info.Capability, hasDryRun, isPending,
			)
		case profiles.CapRead, profiles.CapMeta, profiles.CapUnknown:
		}
	}

	for name := range pending {
		_, exists := registered[name]
		assert.Truef(
			t, exists,
			"dryRunPendingTools lists %q but no such tool is registered; remove the stale allowlist entry",
			name,
		)
	}
}

func TestLinodeInstanceStatsToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var found bool

	for _, info := range srv.ToolInfos() {
		if info.Name == "linode_instance_stats_get" {
			found = true

			assert.Equal(t, profiles.CapRead, info.Capability, "stats tool should be read-only")
			assert.Contains(t, info.InputSchema.Properties, "linode_id", "stats tool should declare linode_id")
			assert.Contains(t, info.InputSchema.Required, "linode_id", "stats tool should require linode_id")
		}
	}

	assert.True(t, found, "server should register the instance stats tool")
}

func TestLinodeNetworkingIPsToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	found := make(map[string]bool, len(infos))

	for _, info := range infos {
		switch info.Name {
		case "linode_networking_ips_list":
			found[info.Name] = true

			assert.Equal(t, profiles.CapRead, info.Capability, "networking IPs list tool should be read-only")
			assert.Contains(t, info.InputSchema.Properties, "skip_ipv6_rdns", "networking IPs list tool should declare skip_ipv6_rdns")
		case "linode_networking_ip_get":
			found[info.Name] = true

			assert.Equal(t, profiles.CapRead, info.Capability, "networking IP get tool should be read-only")
			assert.Contains(t, info.InputSchema.Properties, "address", "networking IP get tool should declare address")
		}
	}

	assert.True(t, found["linode_networking_ips_list"], "server should register the networking IPs list tool")
	assert.True(t, found["linode_networking_ip_get"], "server should register the networking IP get tool")
}

func TestLinodeFirewallTemplatesToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var (
		foundList bool
		foundGet  bool
	)

	for _, info := range srv.ToolInfos() {
		if info.Name == "linode_firewall_templates_list" {
			foundList = true

			assert.Equal(t, profiles.CapRead, info.Capability, "firewall templates tool should be read-only")
			assert.Contains(t, info.InputSchema.Properties, "page", "firewall templates tool should declare page")
			assert.Contains(t, info.InputSchema.Properties, "page_size", "firewall templates tool should declare page_size")
		}

		if info.Name == "linode_firewall_template_get" {
			foundGet = true

			assert.Equal(t, profiles.CapRead, info.Capability, "firewall template get tool should be read-only")
			assert.Contains(t, info.InputSchema.Properties, "slug", "firewall template get tool should declare slug")
			assert.Contains(t, info.InputSchema.Required, "slug", "firewall template get tool should require slug")
			assert.Contains(t, info.InputSchema.Properties, "page", "firewall template get tool should declare page")
			assert.Contains(t, info.InputSchema.Properties, "page_size", "firewall template get tool should declare page_size")
		}
	}

	assert.True(t, foundList, "server should register the firewall templates list tool")
	assert.True(t, foundGet, "server should register the firewall template get tool")
}

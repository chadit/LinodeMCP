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

func TestLinodeFirewallTemplatesToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	var found bool

	for _, info := range srv.ToolInfos() {
		if info.Name == "linode_firewall_templates_list" {
			found = true

			assert.Equal(t, profiles.CapRead, info.Capability, "firewall templates tool should be read-only")
			assert.Contains(t, info.InputSchema.Properties, "page", "firewall templates tool should declare page")
			assert.Contains(t, info.InputSchema.Properties, "page_size", "firewall templates tool should declare page_size")
		}
	}

	assert.True(t, found, "server should register the firewall templates tool")
}

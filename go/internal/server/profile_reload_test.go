package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
)

// TestReloadProfileAddsAndRemovesTools is the core Phase 5 contract: when
// the active profile changes, tools allowed by the new profile but excluded
// by the old one must be registered, and tools allowed by the old profile
// but excluded by the new one must be deregistered. mcp-go emits
// notifications/tools/list_changed automatically when WithToolCapabilities
// is set, so a successful swap also signals connected clients.
func TestReloadProfileAddsAndRemovesTools(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	requireNoError(t, err, "default-profile server must construct cleanly")
	requireNotNil(t, srv)

	before := toolNames(srv)
	requireContains(t, before, toolInstancesList,
		"default profile must start with read tools")
	requireNotContains(t, before, toolInstanceCreate,
		"default profile must start without write tools")

	full := fullAccessConfig()

	requireNoError(t, srv.ReloadProfile(full),
		"reload to full-access on healthy config must succeed")

	after := toolNames(srv)
	assertEqual(t, profiles.BuiltinFullAccess, srv.ActiveProfile().Name,
		"ActiveProfile must reflect the reloaded value")
	assertContains(t, after, toolInstanceCreate,
		"reload to full-access must add write tools")
	assertContains(t, after, toolInstancesList,
		"reload to full-access must keep read tools")
	assertGreater(t, len(after), len(before),
		"full-access registers strictly more tools than default")

	// Reload back to default; the write tool added above must come off.
	requireNoError(t, srv.ReloadProfile(baseTestConfig()),
		"reload back to default must succeed")

	back := toolNames(srv)
	assertEqual(t, profiles.BuiltinDefault, srv.ActiveProfile().Name)
	assertNotContains(t, back, toolInstanceCreate,
		"reload back to default must remove write tools")
	assertContains(t, back, toolInstancesList,
		"reload back to default must keep read tools")
	assertLen(t, back, len(before),
		"reload round-trip must restore the original tool count")
}

// TestReloadProfileDisabledBuiltinIsNoOp confirms that reloading into a
// disabled built-in returns an error and leaves the server's running
// profile and tool set untouched. A failed reload must never partially
// mutate the live registration set.
func TestReloadProfileDisabledBuiltinIsNoOp(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	requireNoError(t, err)
	requireNotNil(t, srv)

	originalProfile := srv.ActiveProfile().Name
	originalTools := toolNames(srv)

	bad := baseTestConfig()
	bad.ActiveProfile = profiles.BuiltinComputeAdmin
	bad.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{
		profiles.BuiltinComputeAdmin: {Disabled: true},
	}

	err = srv.ReloadProfile(bad)
	requireError(t, err, "reload into disabled built-in must fail")
	requireErrorIs(t, err, profiles.ErrActiveProfileDisabled,
		"error must wrap profiles.ErrActiveProfileDisabled")

	assertEqual(t, originalProfile, srv.ActiveProfile().Name,
		"failed reload must leave ActiveProfile untouched")
	assertElementsMatch(t, originalTools, toolNames(srv),
		"failed reload must leave the registered tool set untouched")
}

// TestReloadProfileUnknownIsNoOp verifies the same no-op semantics when
// the target profile name doesn't exist in either built-ins or
// user-defined profiles.
func TestReloadProfileUnknownIsNoOp(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	requireNoError(t, err)
	requireNotNil(t, srv)

	originalProfile := srv.ActiveProfile().Name
	originalTools := toolNames(srv)

	bad := baseTestConfig()
	bad.ActiveProfile = "definitely-not-a-real-profile"

	err = srv.ReloadProfile(bad)
	requireError(t, err)
	requireErrorIs(t, err, profiles.ErrActiveProfileUnknown,
		"error must wrap profiles.ErrActiveProfileUnknown")

	assertEqual(t, originalProfile, srv.ActiveProfile().Name)
	assertElementsMatch(t, originalTools, toolNames(srv))
}

// TestReloadProfileNilConfigRejected confirms ReloadProfile guards against
// a nil config the same way New does: the error is ErrConfigNil and no
// state is touched.
func TestReloadProfileNilConfigRejected(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	requireNoError(t, err)
	requireNotNil(t, srv)

	originalTools := toolNames(srv)

	err = srv.ReloadProfile(nil)
	requireError(t, err)
	requireErrorIs(t, err, server.ErrConfigNil,
		"nil-config reload must return ErrConfigNil")
	assertElementsMatch(t, originalTools, toolNames(srv),
		"nil-config reload must not mutate the tool set")
}

// TestReloadProfileToSingleUserDefinedTool exercises the user-defined
// profile path: a reload into a profile that lists exactly one tool must
// shrink the registered set to that single tool. Confirms wildcard-free
// allow lists work through the reload path the same way they work at
// startup.
func TestReloadProfileToSingleUserDefinedTool(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	requireNoError(t, err)
	requireNotNil(t, srv)

	requireGreater(t, len(srv.Tools()), 1,
		"full-access must start with more than one tool")

	cfg := baseTestConfig()
	cfg.ActiveProfile = profileSingleTool
	cfg.Profiles = map[string]config.UserProfileConfig{
		profileSingleTool: {
			Description:  "Reload-test profile with exactly one allowed tool.",
			AllowedTools: []string{toolVolumesList},
		},
	}

	requireNoError(t, srv.ReloadProfile(cfg))

	names := toolNames(srv)
	assertElementsMatch(t, []string{toolVolumesList}, names,
		"reload into single-tool profile must shrink the registered set to just that tool")
	assertEqual(t, profileSingleTool, srv.ActiveProfile().Name)
}

// TestReloadProfileRepeatedReloadsConverge confirms the reload path is
// idempotent under repeated identical reloads (no leaks, no drift) and
// that a back-to-back swap between two profiles ends at the second one
// regardless of how many cycles occurred. Guards against accumulation
// bugs in the registered-name map.
func TestReloadProfileRepeatedReloadsConverge(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	requireNoError(t, err)

	full := fullAccessConfig()
	defaultCfg := baseTestConfig()

	for range 3 {
		requireNoError(t, srv.ReloadProfile(full))
		requireNoError(t, srv.ReloadProfile(defaultCfg))
	}

	requireNoError(t, srv.ReloadProfile(full))

	assertEqual(t, profiles.BuiltinFullAccess, srv.ActiveProfile().Name)
	assertContains(t, toolNames(srv), toolInstanceCreate,
		"after a final reload to full-access, write tools must be live")

	// Compare against a fresh full-access server to verify the cycle
	// did not lose or duplicate tools.
	fresh, err := server.New(fullAccessConfig())
	requireNoError(t, err)
	assertElementsMatch(t, toolNames(fresh), toolNames(srv),
		"reloaded full-access tool set must equal a freshly constructed one")
}

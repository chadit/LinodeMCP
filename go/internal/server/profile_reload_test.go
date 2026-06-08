package server_test

import (
	"errors"
	"slices"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	before := toolNames(srv)
	if !slices.Contains(before, toolInstancesList) {
		t.Fatalf("before does not contain %v", toolInstancesList)
	}

	if slices.Contains(before, toolInstanceCreate) {
		t.Fatalf("before should not contain %v", toolInstanceCreate)
	}

	full := fullAccessConfig()

	if err := srv.ReloadProfile(full); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := toolNames(srv)
	if srv.ActiveProfile().Name != profiles.BuiltinFullAccess {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profiles.BuiltinFullAccess)
	}

	if !slices.Contains(after, toolInstanceCreate) {
		t.Errorf("after does not contain %v", toolInstanceCreate)
	}

	if !slices.Contains(after, toolInstancesList) {
		t.Errorf("after does not contain %v", toolInstancesList)
	}

	if len(after) <= len(before) {
		t.Errorf("got %v, want > %v", len(after), len(before))
	}

	// Reload back to default; the write tool added above must come off.
	if err := srv.ReloadProfile(baseTestConfig()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	back := toolNames(srv)
	if srv.ActiveProfile().Name != profiles.BuiltinDefault {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profiles.BuiltinDefault)
	}

	if slices.Contains(back, toolInstanceCreate) {
		t.Errorf("back should not contain %v", toolInstanceCreate)
	}

	if !slices.Contains(back, toolInstancesList) {
		t.Errorf("back does not contain %v", toolInstancesList)
	}

	if len(back) != len(before) {
		t.Errorf("len(back) = %d, want %d", len(back), len(before))
	}
}

// TestReloadProfileDisabledBuiltinIsNoOp confirms that reloading into a
// disabled built-in returns an error and leaves the server's running
// profile and tool set untouched. A failed reload must never partially
// mutate the live registration set.
func TestReloadProfileDisabledBuiltinIsNoOp(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	originalProfile := srv.ActiveProfile().Name
	originalTools := toolNames(srv)

	bad := baseTestConfig()
	bad.ActiveProfile = profiles.BuiltinComputeAdmin
	bad.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{
		profiles.BuiltinComputeAdmin: {Disabled: true},
	}

	err = srv.ReloadProfile(bad)
	if !errors.Is(err, profiles.ErrActiveProfileDisabled) {
		t.Fatalf("error = %v, want %v", err, profiles.ErrActiveProfileDisabled)
	}

	if srv.ActiveProfile().Name != originalProfile {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, originalProfile)
	}
	{
		gotEls := slices.Clone(toolNames(srv))
		wantEls := slices.Clone(originalTools)

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", toolNames(srv), originalTools)
		}
	}
}

// TestReloadProfileUnknownIsNoOp verifies the same no-op semantics when
// the target profile name doesn't exist in either built-ins or
// user-defined profiles.
func TestReloadProfileUnknownIsNoOp(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	originalProfile := srv.ActiveProfile().Name
	originalTools := toolNames(srv)

	bad := baseTestConfig()
	bad.ActiveProfile = "definitely-not-a-real-profile"

	err = srv.ReloadProfile(bad)
	if !errors.Is(err, profiles.ErrActiveProfileUnknown) {
		t.Fatalf("error = %v, want %v", err, profiles.ErrActiveProfileUnknown)
	}

	if srv.ActiveProfile().Name != originalProfile {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, originalProfile)
	}
	{
		gotEls := slices.Clone(toolNames(srv))
		wantEls := slices.Clone(originalTools)

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", toolNames(srv), originalTools)
		}
	}
}

// TestReloadProfileNilConfigRejected confirms ReloadProfile guards against
// a nil config the same way New does: the error is ErrConfigNil and no
// state is touched.
func TestReloadProfileNilConfigRejected(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	originalTools := toolNames(srv)

	err = srv.ReloadProfile(nil)
	if !errors.Is(err, server.ErrConfigNil) {
		t.Fatalf("error = %v, want %v", err, server.ErrConfigNil)
	}
	{
		gotEls := slices.Clone(toolNames(srv))
		wantEls := slices.Clone(originalTools)

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", toolNames(srv), originalTools)
		}
	}
}

// TestReloadProfileToSingleUserDefinedTool exercises the user-defined
// profile path: a reload into a profile that lists exactly one tool must
// shrink the registered set to that single tool. Confirms wildcard-free
// allow lists work through the reload path the same way they work at
// startup.
func TestReloadProfileToSingleUserDefinedTool(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	if len(srv.Tools()) <= 1 {
		t.Fatalf("got %v, want > %v", len(srv.Tools()), 1)
	}

	cfg := baseTestConfig()
	cfg.ActiveProfile = profileSingleTool
	cfg.Profiles = map[string]config.UserProfileConfig{
		profileSingleTool: {
			Description:  "Reload-test profile with exactly one allowed tool.",
			AllowedTools: []string{toolVolumesList},
		},
	}

	if err := srv.ReloadProfile(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := toolNames(srv)
	{
		gotEls := slices.Clone(names)
		wantEls := slices.Clone([]string{toolVolumesList})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", names, []string{toolVolumesList})
		}
	}

	if srv.ActiveProfile().Name != profileSingleTool {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profileSingleTool)
	}
}

// TestReloadProfileRepeatedReloadsConverge confirms the reload path is
// idempotent under repeated identical reloads (no leaks, no drift) and
// that a back-to-back swap between two profiles ends at the second one
// regardless of how many cycles occurred. Guards against accumulation
// bugs in the registered-name map.
func TestReloadProfileRepeatedReloadsConverge(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	full := fullAccessConfig()
	defaultCfg := baseTestConfig()

	for range 3 {
		if err := srv.ReloadProfile(full); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := srv.ReloadProfile(defaultCfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if err := srv.ReloadProfile(full); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv.ActiveProfile().Name != profiles.BuiltinFullAccess {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profiles.BuiltinFullAccess)
	}

	if !slices.Contains(toolNames(srv), toolInstanceCreate) {
		t.Errorf("toolNames(srv) does not contain %v", toolInstanceCreate)
	}

	// Compare against a fresh full-access server to verify the cycle
	// did not lose or duplicate tools.
	fresh, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		gotEls := slices.Clone(toolNames(srv))
		wantEls := slices.Clone(toolNames(fresh))

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", toolNames(srv), toolNames(fresh))
		}
	}
}

package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// loaderCatalog returns a small fixed catalog for loader tests. Keeping it
// independent of the built-in profile fixture lets each test reason about
// exact membership without cross-test coupling.
func loaderCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: "hello", Capability: profiles.CapMeta},
		{Name: "version", Capability: profiles.CapMeta},
		{Name: toolProfile, Capability: profiles.CapRead},
		{Name: toolAccount, Capability: profiles.CapRead},
		{Name: toolInstancesList, Capability: profiles.CapRead},
		{Name: "linode_instance_get", Capability: profiles.CapRead},
		{Name: "linode_instance_create", Capability: profiles.CapWrite},
		{Name: toolInstanceDelete, Capability: profiles.CapDestroy},
		{Name: toolVolumesList, Capability: profiles.CapRead},
		{Name: toolVolumeCreate, Capability: profiles.CapWrite},
		{Name: toolVolumeClone, Capability: profiles.CapWrite},
		{Name: toolVolumeDelete, Capability: profiles.CapDestroy},
		{Name: toolVolumeResize, Capability: profiles.CapWrite},
	}
}

func TestResolveActiveProfileDefaultsWhenUnset(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err)
	assertEqual(t, profiles.BuiltinDefault, got.Name, "empty ActiveProfile must fall back to default")
	assertFalse(t, got.Disabled, "resolved default must not be disabled")
	assertNotEmpty(t, got.AllowedTools, "default profile should resolve to at least one read tool")
}

func TestResolveActiveProfileSelectsBuiltin(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: profiles.BuiltinComputeAdmin}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err)
	assertEqual(t, profiles.BuiltinComputeAdmin, got.Name)
	assertContains(t, got.AllowedTools, "linode_instance_create",
		"compute-admin must include instance writes from the catalog")
}

func TestResolveActiveProfileRefusesDisabledBuiltin(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: profiles.BuiltinComputeAdmin,
		ProfilesBuiltinOverrides: map[string]config.BuiltinOverride{
			profiles.BuiltinComputeAdmin: {Disabled: true},
		},
	}

	_, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireError(t, err)
	assertErrorIs(t, err, profiles.ErrActiveProfileDisabled)
}

func TestResolveActiveProfileUserDefinedLiteral(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: "my-prof",
		Profiles: map[string]config.UserProfileConfig{
			"my-prof": {
				Description:  "single tool",
				AllowedTools: []string{toolVolumesList},
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err)
	assertEqual(t, "my-prof", got.Name)
	assertEqual(t, "single tool", got.Description)
	assertEqual(t, []string{toolVolumesList}, got.AllowedTools)
}

func TestResolveActiveProfileWildcardExpansion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: "volumes",
		Profiles: map[string]config.UserProfileConfig{
			"volumes": {
				AllowedTools: []string{"linode_volume_*"},
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err)
	assertElementsMatch(
		t,
		[]string{toolVolumesList, toolVolumeCreate, toolVolumeClone, toolVolumeDelete, toolVolumeResize},
		got.AllowedTools,
		"linode_volume_* must expand to every catalog entry with that prefix",
	)
}

func TestResolveActiveProfileDeniedWinsOverAllowed(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: "volumes-no-delete",
		Profiles: map[string]config.UserProfileConfig{
			"volumes-no-delete": {
				AllowedTools: []string{"linode_volume_*"},
				DeniedTools:  []string{toolVolumeDelete},
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err)
	assertNotContains(t, got.AllowedTools, toolVolumeDelete,
		"explicit deny must remove a tool the allowed wildcard would have included")
	assertContains(t, got.AllowedTools, toolVolumeCreate)
	assertContains(t, got.AllowedTools, toolVolumeResize)
}

func TestResolveActiveProfileUnknownName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: "nonexistent"}

	_, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireError(t, err)
	assertErrorIs(t, err, profiles.ErrActiveProfileUnknown)
}

func TestResolveActiveProfileStarWildcardMatchesEverything(t *testing.T) {
	t.Parallel()

	catalog := loaderCatalog()
	cfg := &config.Config{
		ActiveProfile: "everything",
		Profiles: map[string]config.UserProfileConfig{
			"everything": {
				AllowedTools: []string{"*"},
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, catalog)

	requireNoError(t, err)

	wantNames := make([]string, 0, len(catalog))
	for _, descriptor := range catalog {
		wantNames = append(wantNames, descriptor.Name)
	}

	assertElementsMatch(t, wantNames, got.AllowedTools, "* alone must match every registered tool")
}

func TestResolveActiveProfileWildcardMatchingNothing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: "empty",
		Profiles: map[string]config.UserProfileConfig{
			"empty": {
				AllowedTools: []string{"zzz_*"},
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err, "unmatched wildcards must warn, not error")
	assertEmpty(t, got.AllowedTools, "unmatched wildcard must produce an empty resolved list")
}

func TestResolveActiveProfileOverrideIgnoredForUserDefined(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: profileNameCustom,
		Profiles: map[string]config.UserProfileConfig{
			profileNameCustom: {
				AllowedTools: []string{toolVolumesList},
			},
		},
		ProfilesBuiltinOverrides: map[string]config.BuiltinOverride{
			profileNameCustom: {Disabled: true},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err, "builtin override must not disable a user-defined profile")
	assertEqual(t, profileNameCustom, got.Name)
	assertFalse(t, got.Disabled)
	assertEqual(t, []string{toolVolumesList}, got.AllowedTools)
}

func TestResolveActiveProfileUserDefinedPropagatesSettings(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: "scoped",
		Profiles: map[string]config.UserProfileConfig{
			"scoped": {
				Description:         "scoped to dev",
				AllowedTools:        []string{toolVolumesList},
				AllowedEnvironments: []string{"dev"},
				RequiredTokenScopes: []string{"volumes:read_only"},
				AllowYolo:           true,
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	requireNoError(t, err)
	assertEqual(t, []string{"dev"}, got.AllowedEnvironments)
	assertEqual(t, []string{"volumes:read_only"}, got.RequiredTokenScopes)
	assertTrue(t, got.AllowYolo, "user-defined AllowYolo must propagate to the resolved profile")
}

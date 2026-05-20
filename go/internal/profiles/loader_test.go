package profiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		{Name: toolVolumeDelete, Capability: profiles.CapDestroy},
		{Name: toolVolumeResize, Capability: profiles.CapWrite},
	}
}

func TestResolveActiveProfileDefaultsWhenUnset(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	require.NoError(t, err)
	assert.Equal(t, profiles.BuiltinDefault, got.Name, "empty ActiveProfile must fall back to default")
	assert.False(t, got.Disabled, "resolved default must not be disabled")
	assert.NotEmpty(t, got.AllowedTools, "default profile should resolve to at least one read tool")
}

func TestResolveActiveProfileSelectsBuiltin(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: profiles.BuiltinComputeAdmin}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	require.NoError(t, err)
	assert.Equal(t, profiles.BuiltinComputeAdmin, got.Name)
	assert.Contains(t, got.AllowedTools, "linode_instance_create",
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

	require.Error(t, err)
	assert.ErrorIs(t, err, profiles.ErrActiveProfileDisabled)
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

	require.NoError(t, err)
	assert.Equal(t, "my-prof", got.Name)
	assert.Equal(t, "single tool", got.Description)
	assert.Equal(t, []string{toolVolumesList}, got.AllowedTools)
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

	require.NoError(t, err)
	assert.ElementsMatch(
		t,
		[]string{toolVolumesList, toolVolumeCreate, toolVolumeDelete, toolVolumeResize},
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

	require.NoError(t, err)
	assert.NotContains(t, got.AllowedTools, toolVolumeDelete,
		"explicit deny must remove a tool the allowed wildcard would have included")
	assert.Contains(t, got.AllowedTools, toolVolumeCreate)
	assert.Contains(t, got.AllowedTools, toolVolumeResize)
}

func TestResolveActiveProfileUnknownName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: "nonexistent"}

	_, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())

	require.Error(t, err)
	assert.ErrorIs(t, err, profiles.ErrActiveProfileUnknown)
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

	require.NoError(t, err)

	wantNames := make([]string, 0, len(catalog))
	for _, descriptor := range catalog {
		wantNames = append(wantNames, descriptor.Name)
	}

	assert.ElementsMatch(t, wantNames, got.AllowedTools, "* alone must match every registered tool")
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

	require.NoError(t, err, "unmatched wildcards must warn, not error")
	assert.Empty(t, got.AllowedTools, "unmatched wildcard must produce an empty resolved list")
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

	require.NoError(t, err, "builtin override must not disable a user-defined profile")
	assert.Equal(t, profileNameCustom, got.Name)
	assert.False(t, got.Disabled)
	assert.Equal(t, []string{toolVolumesList}, got.AllowedTools)
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

	require.NoError(t, err)
	assert.Equal(t, []string{"dev"}, got.AllowedEnvironments)
	assert.Equal(t, []string{"volumes:read_only"}, got.RequiredTokenScopes)
	assert.True(t, got.AllowYolo, "user-defined AllowYolo must propagate to the resolved profile")
}

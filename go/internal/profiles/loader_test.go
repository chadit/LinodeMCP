package profiles_test

import (
	"errors"
	"reflect"
	"slices"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != profiles.BuiltinDefault {
		t.Errorf("got.Name = %v, want %v", got.Name, profiles.BuiltinDefault)
	}

	if got.Disabled {
		t.Error("got.Disabled = true, want false")
	}

	if len(got.AllowedTools) == 0 {
		t.Error("got.AllowedTools is empty")
	}
}

func TestResolveActiveProfileSelectsBuiltin(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: profiles.BuiltinComputeAdmin}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != profiles.BuiltinComputeAdmin {
		t.Errorf("got.Name = %v, want %v", got.Name, profiles.BuiltinComputeAdmin)
	}

	if !slices.Contains(got.AllowedTools, "linode_instance_create") {
		t.Errorf("got.AllowedTools does not contain %v", "linode_instance_create")
	}
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
	if !errors.Is(err, profiles.ErrActiveProfileDisabled) {
		t.Errorf("error = %v, want %v", err, profiles.ErrActiveProfileDisabled)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != tcMyProf {
		t.Errorf("got.Name = %v, want %v", got.Name, tcMyProf)
	}

	if got.Description != "single tool" {
		t.Errorf("got.Description = %v, want %v", got.Description, "single tool")
	}

	if !reflect.DeepEqual(got.AllowedTools, []string{toolVolumesList}) {
		t.Errorf("got.AllowedTools = %v, want %v", got.AllowedTools, []string{toolVolumesList})
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		gotEls := slices.Clone(got.AllowedTools)
		wantEls := slices.Clone([]string{toolVolumesList, toolVolumeCreate, toolVolumeClone, toolVolumeDelete, toolVolumeResize})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", got.AllowedTools, []string{toolVolumesList, toolVolumeCreate, toolVolumeClone, toolVolumeDelete, toolVolumeResize})
		}
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slices.Contains(got.AllowedTools, toolVolumeDelete) {
		t.Errorf("got.AllowedTools should not contain %v", toolVolumeDelete)
	}

	if !slices.Contains(got.AllowedTools, toolVolumeCreate) {
		t.Errorf("got.AllowedTools does not contain %v", toolVolumeCreate)
	}

	if !slices.Contains(got.AllowedTools, toolVolumeResize) {
		t.Errorf("got.AllowedTools does not contain %v", toolVolumeResize)
	}
}

func TestResolveActiveProfileUnknownName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: "nonexistent"}

	_, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())
	if !errors.Is(err, profiles.ErrActiveProfileUnknown) {
		t.Errorf("error = %v, want %v", err, profiles.ErrActiveProfileUnknown)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantNames := make([]string, 0, len(catalog))
	for _, descriptor := range catalog {
		wantNames = append(wantNames, descriptor.Name)
	}

	{
		gotEls := slices.Clone(got.AllowedTools)
		wantEls := slices.Clone(wantNames)

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", got.AllowedTools, wantNames)
		}
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.AllowedTools) != 0 {
		t.Errorf("got.AllowedTools = %v, want empty", got.AllowedTools)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != profileNameCustom {
		t.Errorf("got.Name = %v, want %v", got.Name, profileNameCustom)
	}

	if got.Disabled {
		t.Error("got.Disabled = true, want false")
	}

	if !reflect.DeepEqual(got.AllowedTools, []string{toolVolumesList}) {
		t.Errorf("got.AllowedTools = %v, want %v", got.AllowedTools, []string{toolVolumesList})
	}
}

func TestResolveActiveProfileUserDefinedPropagatesSettings(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ActiveProfile: "scoped",
		Profiles: map[string]config.UserProfileConfig{
			"scoped": {
				Description:         "scoped to dev",
				AllowedTools:        []string{toolVolumesList},
				AllowedEnvironments: []string{tcDev},
				RequiredTokenScopes: []string{tcVolumesReadOnly},
				AllowYolo:           true,
			},
		},
	}

	got, err := profiles.ResolveActiveProfile(cfg, loaderCatalog())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got.AllowedEnvironments, []string{tcDev}) {
		t.Errorf("got.AllowedEnvironments = %v, want %v", got.AllowedEnvironments, []string{tcDev})
	}

	if !reflect.DeepEqual(got.RequiredTokenScopes, []string{tcVolumesReadOnly}) {
		t.Errorf("got.RequiredTokenScopes = %v, want %v", got.RequiredTokenScopes, []string{tcVolumesReadOnly})
	}

	if !got.AllowYolo {
		t.Error("got.AllowYolo = false, want true")
	}
}

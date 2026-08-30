package profiles_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// TestResolveActiveProfileRefusesANilConfig pins the guard a caller that
// skipped config loading hits: the resolver reports the profile as unknown
// instead of dereferencing nil.
func TestResolveActiveProfileRefusesANilConfig(t *testing.T) {
	t.Parallel()

	_, err := profiles.ResolveActiveProfile(nil, loaderCatalog())
	if !errors.Is(err, profiles.ErrActiveProfileUnknown) {
		t.Errorf("error = %v, want %v", err, profiles.ErrActiveProfileUnknown)
	}
}

// TestLookupProfileMissesWithoutAConfigOrName covers the inputs the builder
// hands over before it has anything to clone from. Each row must miss cleanly
// so the caller falls back to its own default rather than seeing a partial
// Profile.
func TestLookupProfileMissesWithoutAConfigOrName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg  *config.Config
		name string
		key  string
	}{
		{name: "nil config", cfg: nil, key: profiles.BuiltinDefault},
		{name: "empty name", cfg: &config.Config{}, key: ""},
		{name: "name in neither catalog", cfg: &config.Config{}, key: "nonexistent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := profiles.LookupProfile(tt.key, tt.cfg, loaderCatalog())
			if ok {
				t.Errorf("LookupProfile(%q) reported a hit, want a miss", tt.key)
			}

			if !reflect.DeepEqual(got, profiles.Profile{}) {
				t.Errorf("LookupProfile(%q) = %+v, want the zero Profile on a miss", tt.key, got)
			}
		})
	}
}

// TestLookupProfileClonesADisabledBuiltinWithoutTheFlag pins the difference
// between the two resolvers: the active-profile path refuses full-access
// while it is disabled, but the builder must still be able to clone from it,
// and the clone must not inherit the disabled state.
func TestLookupProfileClonesADisabledBuiltinWithoutTheFlag(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ActiveProfile: profiles.BuiltinFullAccess}
	catalog := loaderCatalog()

	if _, err := profiles.ResolveActiveProfile(cfg, catalog); !errors.Is(err, profiles.ErrActiveProfileDisabled) {
		t.Fatalf("ResolveActiveProfile(full-access) error = %v, want %v", err, profiles.ErrActiveProfileDisabled)
	}

	got, ok := profiles.LookupProfile(profiles.BuiltinFullAccess, cfg, catalog)
	if !ok {
		t.Fatal("LookupProfile(full-access) missed, want the disabled built-in returned")
	}

	if got.Name != profiles.BuiltinFullAccess {
		t.Errorf("got.Name = %q, want %q", got.Name, profiles.BuiltinFullAccess)
	}

	if got.Disabled {
		t.Error("got.Disabled = true, want the clone to drop the built-in's disabled flag")
	}

	if len(got.AllowedTools) == 0 {
		t.Error("got.AllowedTools is empty, want the built-in's resolved tool list")
	}
}

// TestLookupProfileExpandsAUserDefinedProfile verifies the builder sees the
// same materialized shape the loader produces: wildcards expanded against
// the registry, denies subtracted, and Elevated derived from what is left.
func TestLookupProfileExpandsAUserDefinedProfile(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Profiles: map[string]config.UserProfileConfig{
			"volume-create-only": {
				Description:  "volume create without clone",
				AllowedTools: []string{"linode_volume_c*"},
				DeniedTools:  []string{toolVolumeClone},
			},
		},
	}

	got, ok := profiles.LookupProfile("volume-create-only", cfg, loaderCatalog())
	if !ok {
		t.Fatal("LookupProfile missed a name present in cfg.Profiles")
	}

	want := []string{toolVolumeCreate}
	if !reflect.DeepEqual(got.AllowedTools, want) {
		t.Errorf("got.AllowedTools = %v, want %v", got.AllowedTools, want)
	}

	if got.Description != "volume create without clone" {
		t.Errorf("got.Description = %q, want %q", got.Description, "volume create without clone")
	}

	if !got.Elevated {
		t.Error("got.Elevated = false, want true because the expansion keeps write tools")
	}
}

// TestLookupProfileUserDefinedShadowsABuiltinName pins the precedence the
// doc promises: a user-defined entry named like a built-in wins, matching
// what ResolveActiveProfile would activate under that name.
func TestLookupProfileUserDefinedShadowsABuiltinName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Profiles: map[string]config.UserProfileConfig{
			profiles.BuiltinDefault: {AllowedTools: []string{toolVolumesList}},
		},
	}

	got, ok := profiles.LookupProfile(profiles.BuiltinDefault, cfg, loaderCatalog())
	if !ok {
		t.Fatal("LookupProfile(default) missed, want the user-defined entry")
	}

	if !reflect.DeepEqual(got.AllowedTools, []string{toolVolumesList}) {
		t.Errorf("got.AllowedTools = %v, want the user-defined %v rather than the built-in default",
			got.AllowedTools, []string{toolVolumesList})
	}
}

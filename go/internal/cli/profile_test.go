package cli_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/cli"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	testProfileComputeAdmin = "compute-admin"
	testUserProfile         = "my-custom"
	testVolumesListTool     = "linode_volume_list"
)

// testCatalog returns a minimal config that the catalog/listing helpers
// can use without touching the user's real config file. The default
// environment is intentionally empty (the CLI list/show commands do not
// talk to the Linode API).
func testCatalog() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Name: "Test"},
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default"},
		},
	}
}

// TestAllProfilesContainsBuiltins verifies the catalog assembler
// returns every built-in profile. The list view depends on this set
// being correct.
func TestAllProfilesContainsBuiltins(t *testing.T) {
	t.Parallel()

	all := cli.AllProfiles(testCatalog())

	want := []string{
		profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
		profiles.BuiltinComputeAdmin,
		profiles.BuiltinNetworkAdmin,
		profiles.BuiltinKubernetesAdmin,
		profiles.BuiltinStorageAdmin,
		profiles.BuiltinFullAccess,
		profiles.BuiltinEmergency,
	}

	for _, name := range want {
		assert.Containsf(t, all, name,
			"catalog must include built-in profile %q", name)
	}
}

// TestAllProfilesIncludesUserDefined verifies user-defined profiles
// from cfg.Profiles get folded into the catalog alongside built-ins so
// `profile list` shows them.
func TestAllProfilesIncludesUserDefined(t *testing.T) {
	t.Parallel()

	cfg := testCatalog()
	cfg.Profiles = map[string]config.UserProfileConfig{
		testUserProfile: {
			Description:  "User-defined for the CLI list test",
			AllowedTools: []string{testVolumesListTool},
		},
	}

	all := cli.AllProfiles(cfg)

	prof, ok := all[testUserProfile]
	require.True(t, ok, "user-defined profile must appear in catalog")
	assert.Equal(t, "User-defined for the CLI list test", prof.Description)
	assert.Equal(t, []string{testVolumesListTool}, prof.AllowedTools)
}

// TestAllProfilesAppliesBuiltinOverrides verifies that disabling a
// built-in via ProfilesBuiltinOverrides is reflected in the catalog
// produced for the CLI. The `list` view shows DISABLED for these so
// users can spot a misconfigured override.
func TestAllProfilesAppliesBuiltinOverrides(t *testing.T) {
	t.Parallel()

	cfg := testCatalog()
	cfg.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{
		testProfileComputeAdmin: {Disabled: true},
	}

	all := cli.AllProfiles(cfg)
	prof := all[testProfileComputeAdmin]
	assert.True(t, prof.Disabled,
		"override Disabled=true must propagate into the listed profile")
}

// TestResolveActiveNameDefaults locks in the default fallback. An
// unset ActiveProfile must resolve to "default" so the active-marker
// in `profile list` stays accurate when users haven't picked one yet.
func TestResolveActiveNameDefaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, profiles.BuiltinDefault, cli.ResolveActiveName(testCatalog()))

	cfg := testCatalog()
	cfg.ActiveProfile = testProfileComputeAdmin
	assert.Equal(t, testProfileComputeAdmin, cli.ResolveActiveName(cfg))
}

// TestRunProfileCommandUnknownSubcommandReturnsUsageError verifies
// that an unknown subcommand exits with the usage-error code and
// writes the usage block to stderr.
func TestRunProfileCommandUnknownSubcommandReturnsUsageError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	rc := cli.RunProfileCommand([]string{"nonexistent"}, &bytes.Buffer{}, &stderr)

	assert.Equal(t, cli.ExitUsageError, rc,
		"unknown subcommand must exit with the usage-error code")
	assert.Contains(t, stderr.String(), "Usage:",
		"unknown subcommand must surface the full usage block")
}

// TestRunProfileCommandEmptyArgsReturnsUsageError verifies that
// `linodemcp profile` (no subcommand) shows usage and exits with the
// usage-error code.
func TestRunProfileCommandEmptyArgsReturnsUsageError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	rc := cli.RunProfileCommand(nil, &bytes.Buffer{}, &stderr)

	assert.Equal(t, cli.ExitUsageError, rc)
	assert.Contains(t, stderr.String(), "Usage:")
}

// TestRunProfileShowZeroArgsReturnsUsage exercises the arity check in
// the show subcommand directly so the test stays decoupled from the
// config-loading path. Show requires exactly one positional arg.
func TestRunProfileShowZeroArgsReturnsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	rc := cli.RunProfileShow(nil, &bytes.Buffer{}, &stderr)

	assert.Equal(t, cli.ExitUsageError, rc,
		"show with zero args must exit with the usage-error code")
	assert.Contains(t, stderr.String(), "Usage:",
		"zero-arg invocation must print usage to stderr")
}

// TestPrintProfileDetailMarksActive verifies the active marker on
// the detail header. The pretty-print is order-stable so substring
// checks suffice.
func TestPrintProfileDetailMarksActive(t *testing.T) {
	t.Parallel()

	prof := profiles.Profile{
		Name:        testProfileComputeAdmin,
		Description: "test profile",
	}

	var buf bytes.Buffer
	cli.PrintProfileDetail(&buf, &prof, testProfileComputeAdmin)

	out := buf.String()
	assert.Contains(t, out, "Profile: compute-admin (active)",
		"active profile must be marked in the header")
}

// TestPrintProfileDetailOmitsActiveMarkerForInactive locks the
// inverse case: a profile that is not the active one must NOT carry
// the "(active)" marker.
func TestPrintProfileDetailOmitsActiveMarkerForInactive(t *testing.T) {
	t.Parallel()

	prof := profiles.Profile{
		Name:        testProfileComputeAdmin,
		Description: "test",
	}

	var buf bytes.Buffer
	cli.PrintProfileDetail(&buf, &prof, profiles.BuiltinDefault)

	assert.NotContains(t, buf.String(), "(active)",
		"inactive profile must not be marked")
}

// TestPrintProfileDetailListsAllowedTools verifies that the
// AllowedTools list appears in the output with its count header.
func TestPrintProfileDetailListsAllowedTools(t *testing.T) {
	t.Parallel()

	prof := profiles.Profile{
		Name:         testProfileComputeAdmin,
		AllowedTools: []string{"linode_instance_list", "linode_instance_create"},
	}

	var buf bytes.Buffer
	cli.PrintProfileDetail(&buf, &prof, "")

	out := buf.String()
	assert.Contains(t, out, "Allowed tools (2):")
	assert.Contains(t, out, "linode_instance_list")
	assert.Contains(t, out, "linode_instance_create")
}

// TestPrintProfileDetailShowsRequiredScopes locks in that the
// scope list appears with its count header so users can audit what
// the profile expects from the token.
func TestPrintProfileDetailShowsRequiredScopes(t *testing.T) {
	t.Parallel()

	prof := profiles.Profile{
		Name:                testProfileComputeAdmin,
		RequiredTokenScopes: []string{"linodes:read_write", "volumes:read_write"},
	}

	var buf bytes.Buffer
	cli.PrintProfileDetail(&buf, &prof, "")

	out := buf.String()
	assert.Contains(t, out, "Required token scopes (2):")
	assert.Contains(t, out, "linodes:read_write")
	assert.Contains(t, out, "volumes:read_write")
}

// TestAllProfilesUserDefinedShadowsBuiltin verifies that a user
// profile with the same name as a built-in replaces the built-in in
// the listed catalog. This matches the resolver's precedence so the
// list view reflects what would actually run.
func TestAllProfilesUserDefinedShadowsBuiltin(t *testing.T) {
	t.Parallel()

	cfg := testCatalog()
	cfg.Profiles = map[string]config.UserProfileConfig{
		profiles.BuiltinDefault: {
			Description:  "shadowed default",
			AllowedTools: []string{testVolumesListTool},
		},
	}

	all := cli.AllProfiles(cfg)
	got := all[profiles.BuiltinDefault]

	assert.Equal(t, "shadowed default", got.Description,
		"user-defined profile with built-in name must replace the built-in")
	assert.True(t, slices.Contains(got.AllowedTools, testVolumesListTool),
		"shadowed entry must carry the user-defined allow list")
}

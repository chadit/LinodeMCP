package profiles_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

// syntheticCatalog returns a hand-built tool catalog that covers every
// category and capability the built-in profile rules touch. The fixture is
// intentionally independent of the live server registry so these tests stay
// stable as Phase 1 capability tags get filled in across the real tools.
func syntheticCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		// Core / meta
		{Name: "hello", Capability: profiles.CapMeta},
		{Name: "version", Capability: profiles.CapMeta},
		{Name: toolProfile, Capability: profiles.CapRead},
		{Name: toolAccount, Capability: profiles.CapRead},
		{Name: "linode_account_users", Capability: profiles.CapRead},
		{Name: "linode_account_user_update", Capability: profiles.CapAdmin},
		{Name: "linode_account_user_delete", Capability: profiles.CapDestroy},
		{Name: "linode_account_oauth_clients", Capability: profiles.CapRead},

		// Compute reads
		{Name: toolInstancesList, Capability: profiles.CapRead},
		{Name: "linode_instance_get", Capability: profiles.CapRead},
		{Name: "linode_region_list", Capability: profiles.CapRead},
		{Name: "linode_type_list", Capability: profiles.CapRead},
		{Name: "linode_image_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroups_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_get", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_images_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_members_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_member_token_get", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_create", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_images_add", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_image_update", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_update", Capability: profiles.CapWrite},

		{Name: "linode_image_sharegroup_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_image_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_tokens_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_token_get", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_token_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_token_images_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_token_update", Capability: profiles.CapAdmin},
		{Name: "linode_image_sharegroup_by_token_get", Capability: profiles.CapRead},
		{Name: "linode_stackscript_list", Capability: profiles.CapRead},

		// Databases reads
		{Name: "linode_database_engine_list", Capability: profiles.CapRead},
		{Name: "linode_database_type_list", Capability: profiles.CapRead},
		{Name: "linode_database_type_get", Capability: profiles.CapRead},
		{Name: "linode_database_engine_get", Capability: profiles.CapRead},
		{Name: "linode_database_mysql_config_get", Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_config_get", Capability: profiles.CapRead},
		{Name: "linode_database_instance_list", Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_instance_list", Capability: profiles.CapRead},
		{Name: "linode_database_instance_get", Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_instance_get", Capability: profiles.CapRead},
		{Name: "linode_database_instance_ssl_get", Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_instance_ssl_get", Capability: profiles.CapRead},
		{Name: "linode_database_instance_credentials_get", Capability: profiles.CapAdmin},
		{Name: "linode_database_instance_credentials_reset", Capability: profiles.CapAdmin},
		{Name: "linode_database_postgresql_instance_credentials_reset", Capability: profiles.CapAdmin},
		{Name: "linode_database_instance_update", Capability: profiles.CapWrite},
		{Name: "linode_database_instance_delete", Capability: profiles.CapDestroy},
		{Name: "linode_database_postgresql_instance_delete", Capability: profiles.CapDestroy},
		{Name: "linode_database_instance_patch", Capability: profiles.CapWrite},
		{Name: "linode_database_postgresql_instance_patch", Capability: profiles.CapWrite},
		{Name: "linode_database_instance_suspend", Capability: profiles.CapWrite},
		{Name: "linode_database_postgresql_instance_suspend", Capability: profiles.CapWrite},
		{Name: "linode_database_instance_resume", Capability: profiles.CapWrite},
		{Name: "linode_database_postgresql_instance_resume", Capability: profiles.CapWrite},
		// Compute writes / destroys
		{Name: "linode_instance_create", Capability: profiles.CapWrite},
		{Name: "linode_stackscript_create", Capability: profiles.CapWrite},
		{Name: toolInstanceDelete, Capability: profiles.CapDestroy},
		{Name: "linode_instance_boot", Capability: profiles.CapWrite},
		{Name: "linode_instance_reboot", Capability: profiles.CapWrite},
		{Name: "linode_instance_shutdown", Capability: profiles.CapWrite},
		{Name: "linode_instance_resize", Capability: profiles.CapWrite},
		{Name: "linode_instance_clone", Capability: profiles.CapWrite},
		{Name: "linode_instance_migrate", Capability: profiles.CapWrite},
		{Name: "linode_instance_rebuild", Capability: profiles.CapDestroy},
		{Name: "linode_instance_rescue", Capability: profiles.CapWrite},
		{Name: "linode_instance_password_reset", Capability: profiles.CapDestroy},

		// Compute deep (backups, disks, IPs)
		{Name: "linode_instance_backup_list", Capability: profiles.CapRead},
		{Name: "linode_instance_backup_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_disk_list", Capability: profiles.CapRead},
		{Name: "linode_instance_disk_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_disk_delete", Capability: profiles.CapDestroy},
		{Name: "linode_instance_ip_list", Capability: profiles.CapRead},
		{Name: "linode_instance_ip_allocate", Capability: profiles.CapWrite},
		{Name: "linode_instance_ip_update_rdns", Capability: profiles.CapWrite},

		// Block storage
		{Name: toolVolumesList, Capability: profiles.CapRead},
		{Name: toolVolumeCreate, Capability: profiles.CapWrite},
		{Name: toolVolumeDelete, Capability: profiles.CapDestroy},
		{Name: toolVolumeResize, Capability: profiles.CapWrite},

		// Object storage
		{Name: "linode_object_storage_bucket_list", Capability: profiles.CapRead},
		{Name: "linode_object_storage_bucket_create", Capability: profiles.CapWrite},
		{Name: "linode_object_storage_bucket_delete", Capability: profiles.CapDestroy},

		// Networking
		{Name: "linode_firewall_list", Capability: profiles.CapRead},
		{Name: "linode_firewall_create", Capability: profiles.CapWrite},
		{Name: "linode_firewall_delete", Capability: profiles.CapDestroy},
		{Name: "linode_nodebalancer_list", Capability: profiles.CapRead},
		{Name: "linode_nodebalancer_create", Capability: profiles.CapWrite},

		// DNS
		{Name: "linode_domain_list", Capability: profiles.CapRead},
		{Name: "linode_domain_create", Capability: profiles.CapWrite},
		{Name: "linode_domain_clone", Capability: profiles.CapWrite},
		{Name: "linode_domain_delete", Capability: profiles.CapDestroy},
		{Name: "linode_domain_record_create", Capability: profiles.CapWrite},

		// LKE
		{Name: "linode_lke_cluster_list", Capability: profiles.CapRead},
		{Name: "linode_lke_cluster_create", Capability: profiles.CapWrite},
		{Name: "linode_lke_cluster_delete", Capability: profiles.CapDestroy},

		// VPCs
		{Name: "linode_vpc_list", Capability: profiles.CapRead},
		{Name: "linode_vpc_create", Capability: profiles.CapWrite},
		{Name: "linode_vpc_delete", Capability: profiles.CapDestroy},

		// Security (SSH keys)
		{Name: "linode_sshkey_list", Capability: profiles.CapRead},
		{Name: "linode_sshkey_create", Capability: profiles.CapWrite},
		{Name: "linode_sshkey_update", Capability: profiles.CapWrite},
		{Name: "linode_sshkey_delete", Capability: profiles.CapDestroy},

		// Account admin tools must stay out of built-in profiles.
		{Name: "linode_account_update", Capability: profiles.CapAdmin},
		{Name: "linode_account_settings_update", Capability: profiles.CapAdmin},
		{Name: "linode_account_settings_managed_enable", Capability: profiles.CapAdmin},
		{Name: "linode_account_beta_enroll", Capability: profiles.CapAdmin},
		{Name: "linode_account_cancel", Capability: profiles.CapAdmin},
	}
}

// readAndMetaNames returns the set of tool names in the catalog whose
// capability is CapRead or CapMeta. Used by tests that assert the default
// profile contents exactly match the read/meta slice of the catalog.
func readAndMetaNames(catalog []profiles.ToolDescriptor) []string {
	names := make([]string, 0, len(catalog))

	for _, descriptor := range catalog {
		if descriptor.Capability == profiles.CapRead || descriptor.Capability == profiles.CapMeta {
			names = append(names, descriptor.Name)
		}
	}

	slices.Sort(names)

	return names
}

func TestBuiltinProfilesAreNonEmpty(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	builtins := profiles.BuiltinProfiles(catalog)

	expected := []string{
		profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
		profiles.BuiltinComputeAdmin,
		profiles.BuiltinNetworkAdmin,
		profiles.BuiltinKubernetesAdmin,
		profiles.BuiltinStorageAdmin,
		profiles.BuiltinFullAccess,
		profiles.BuiltinEmergency,
	}

	for _, name := range expected {
		profile, found := builtins[name]
		require.Truef(t, found, "built-in profile %q should be present", name)
		assert.NotEmptyf(
			t,
			profile.AllowedTools,
			"profile %q should resolve at least one tool from the catalog",
			name,
		)
	}
}

func TestDefaultProfileContainsOnlyReadAndMeta(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	builtins := profiles.BuiltinProfiles(catalog)

	want := readAndMetaNames(catalog)
	got := builtins[profiles.BuiltinDefault].AllowedTools

	assert.Equal(t, want, got, "default profile must contain exactly the read/meta tools from the catalog")

	gotReadonly := builtins[profiles.BuiltinReadonlyFull].AllowedTools
	assert.Equal(t, want, gotReadonly, "readonly-full profile must mirror default's tool set")
}

func TestEmergencyAllowsYolo(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())

	assert.True(t, builtins[profiles.BuiltinEmergency].AllowYolo, "emergency must opt into yolo")
	assert.False(t, builtins[profiles.BuiltinDefault].AllowYolo, "default must never allow yolo")
	assert.False(t, builtins[profiles.BuiltinFullAccess].AllowYolo, "full-access must not allow yolo by default")
}

func TestFullAccessAndEmergencyDisabled(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())

	assert.True(t, builtins[profiles.BuiltinFullAccess].Disabled, "full-access must be disabled by default")
	assert.True(t, builtins[profiles.BuiltinEmergency].Disabled, "emergency must be disabled by default")
	assert.False(t, builtins[profiles.BuiltinDefault].Disabled, "default must remain enabled")
	assert.False(t, builtins[profiles.BuiltinComputeAdmin].Disabled, "compute-admin must remain enabled")
}

func TestComputeAdminIncludesInstanceWrites(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())
	allowed := builtins[profiles.BuiltinComputeAdmin].AllowedTools

	assert.Contains(t, allowed, "linode_instance_create", "compute-admin must include instance writes")
	assert.Contains(t, allowed, "linode_instance_delete", "compute-admin must include instance destroys")
	assert.Contains(t, allowed, toolVolumeCreate, "compute-admin must include volume writes")
	assert.Contains(t, allowed, "linode_sshkey_create", "compute-admin must include ssh key writes")
	assert.Contains(t, allowed, "linode_instance_backup_create", "compute-admin must include backup writes")
}

func TestNetworkAdminExcludesComputeWrites(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())
	allowed := builtins[profiles.BuiltinNetworkAdmin].AllowedTools

	assert.NotContains(t, allowed, "linode_instance_create", "network-admin must not include compute writes")
	assert.NotContains(t, allowed, toolVolumeCreate, "network-admin must not include block-storage writes")
	assert.Contains(t, allowed, "linode_firewall_create", "network-admin must include firewall writes")
	assert.Contains(t, allowed, "linode_domain_create", "network-admin must include DNS writes")
	assert.Contains(t, allowed, "linode_vpc_create", "network-admin must include VPC writes")
}

func TestCapAdminExcludedFromEveryBuiltin(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())

	for name, profile := range builtins {
		assert.NotContainsf(
			t,
			profile.AllowedTools,
			"linode_account_update",
			"profile %q must never include CapAdmin tools",
			name,
		)
		assert.NotContainsf(
			t,
			profile.AllowedTools,
			"linode_account_settings_update",
			"profile %q must never include account settings admin tool",
			name,
		)
		assert.NotContainsf(
			t,
			profile.AllowedTools,
			"linode_account_settings_managed_enable",
			"profile %q must never include managed enable admin tool",
			name,
		)
		assert.NotContainsf(
			t,
			profile.AllowedTools,
			"linode_account_beta_enroll",
			"profile %q must never include beta enrollment admin tool",
			name,
		)
		assert.NotContainsf(
			t,
			profile.AllowedTools,
			"linode_account_cancel",
			"profile %q must never include account cancellation admin tool",
			name,
		)
		assert.NotContainsf(
			t,
			profile.AllowedTools,
			"linode_account_event_seen",
			"profile %q must never include event seen admin tool",
			name,
		)
	}
}

// TestRequiredTokenScopesDerivedFromTools is the Phase 6.3 contract:
// each built-in profile's RequiredTokenScopes must equal the
// deduplicated, sorted union of RequiredScopes() over its AllowedTools.
// This pins the derivation against drift between the scope catalog and
// the built-in blueprint and catches regressions where someone restores
// hardcoded scope lists by mistake.
func TestRequiredTokenScopesDerivedFromTools(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	built := profiles.BuiltinProfiles(catalog)

	capByName := make(map[string]profiles.Capability, len(catalog))
	for _, d := range catalog {
		capByName[d.Name] = d.Capability
	}

	for name, prof := range built {
		expected := make(map[string]struct{}, len(prof.AllowedTools))

		for _, toolName := range prof.AllowedTools {
			capability, ok := capByName[toolName]
			if !ok {
				continue
			}

			for _, scope := range profiles.RequiredScopes(toolName, capability) {
				expected[string(scope)] = struct{}{}
			}
		}

		assert.Lenf(
			t,
			prof.RequiredTokenScopes,
			len(expected),
			"profile %s should have %d unique scopes, got %d",
			name, len(expected), len(prof.RequiredTokenScopes),
		)

		for _, scope := range prof.RequiredTokenScopes {
			_, found := expected[scope]
			assert.Truef(
				t, found,
				"profile %s has scope %q that no allowed tool requires",
				name, scope,
			)
		}
	}
}

// TestRequiredTokenScopesReadOnlyProfilesHaveNoWriteScopes verifies that
// read-only built-ins (default, readonly-full) carry only :read_only
// scopes. A regression that accidentally lets a write tool slip into a
// read-only profile would surface here as a :read_write scope showing up.
func TestRequiredTokenScopesReadOnlyProfilesHaveNoWriteScopes(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	built := profiles.BuiltinProfiles(catalog)

	for _, name := range []string{
		profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
	} {
		prof, ok := built[name]
		require.Truef(t, ok, "built-in %s must exist", name)

		for _, scope := range prof.RequiredTokenScopes {
			assert.NotContainsf(
				t, scope, ":read_write",
				"profile %s is read-only but lists a write scope %q",
				name, scope,
			)
		}
	}
}

// TestRequiredTokenScopesFullAccessIncludesLinodesWrite verifies that a
// full-access profile aggregates write scopes from every category. Pins
// the Phase 6.3 derivation against silent drops by checking a few
// known-required scopes are present.
func TestRequiredTokenScopesFullAccessIncludesLinodesWrite(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	built := profiles.BuiltinProfiles(catalog)

	full, ok := built[profiles.BuiltinFullAccess]
	require.True(t, ok)

	// Each entry below is a scope the synthetic catalog must produce for
	// full-access. images:read_only is present because instance_create
	// pulls it in via the cross-category extras table.
	want := []string{
		string(profiles.ScopeLinodesReadWrite),
		string(profiles.ScopeVolumesReadWrite),
		string(profiles.ScopeDomainsReadWrite),
		string(profiles.ScopeFirewallReadWrite),
		string(profiles.ScopeNodeBalancersReadWrite),
		string(profiles.ScopeLKEReadWrite),
		string(profiles.ScopeObjectStorageReadWrite),
		string(profiles.ScopeVPCReadWrite),
		string(profiles.ScopeStackScriptsReadWrite),
		string(profiles.ScopeImagesReadOnly),
	}

	for _, scope := range want {
		assert.Containsf(
			t, full.RequiredTokenScopes, scope,
			"full-access profile should include %s in its required scopes", scope,
		)
	}
}

// TestRequiredTokenScopesSorted locks in the sort order so cross-language
// parity comparison via BuiltinCatalogJSON stays stable.
func TestRequiredTokenScopesSorted(t *testing.T) {
	t.Parallel()

	built := profiles.BuiltinProfiles(syntheticCatalog())

	for name, prof := range built {
		assert.Truef(
			t,
			slices.IsSorted(prof.RequiredTokenScopes),
			"profile %s must have RequiredTokenScopes sorted ascending for parity",
			name,
		)
	}
}

func TestJSONRoundtrip(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()

	data, err := profiles.BuiltinCatalogJSON(catalog)
	require.NoError(t, err, "BuiltinCatalogJSON should succeed on a healthy catalog")
	require.NotEmpty(t, data, "JSON output must not be empty")

	var decoded []map[string]any

	require.NoError(t, json.Unmarshal(data, &decoded), "JSON should round-trip cleanly")
	require.Len(t, decoded, 8, "catalog must contain all eight built-in profiles")

	names := make([]string, 0, len(decoded))
	for _, entry := range decoded {
		name, ok := entry["name"].(string)
		require.True(t, ok, "every entry must carry a string name")

		names = append(names, name)
	}

	assert.True(t, slices.IsSorted(names), "profile entries must be sorted by name for stable parity comparison")

	// Re-marshal and require byte-identical output to confirm determinism.
	second, err := profiles.BuiltinCatalogJSON(catalog)
	require.NoError(t, err)
	assert.Equal(t, data, second, "BuiltinCatalogJSON must be deterministic across calls")
}

func TestCategoriesIncludesAccountInvoicesInCore(t *testing.T) {
	t.Parallel()

	assert.Contains(t, profiles.Categories("linode_account_invoices"), "core")
	assert.Contains(t, profiles.Categories("linode_account_payments"), "core")
	assert.Contains(t, profiles.Categories("linode_account_payment_create"), "core")
	assert.Contains(t, profiles.Categories("linode_account_promo_credit"), "core")
	assert.Contains(t, profiles.Categories("linode_account_invoice_items"), "core")
}

func TestCategoriesIncludesAccountPaymentMethodsInCore(t *testing.T) {
	t.Parallel()

	assert.Contains(t, profiles.Categories("linode_account_payment_methods"), "core")
	assert.Contains(t, profiles.Categories("linode_account_payment_method_get"), "core")
	assert.Contains(t, profiles.Categories("linode_account_payment_method_create"), "core")
	assert.Contains(t, profiles.Categories("linode_account_payment_method_delete"), "core")
	assert.Contains(t, profiles.Categories("linode_account_payment_method_make_default"), "core")
}

func TestCategoriesIncludesAccountOAuthClientsInCore(t *testing.T) {
	t.Parallel()

	assert.Contains(t, profiles.Categories("linode_account_oauth_clients"), "core")
}

func TestCategoriesIncludesAccountUsersInCore(t *testing.T) {
	t.Parallel()

	assert.Contains(t, profiles.Categories("linode_account_users"), "core")
	assert.Contains(t, profiles.Categories("linode_account_user_get"), "core")
	assert.Contains(t, profiles.Categories("linode_account_user_grants"), "core")
	assert.Contains(t, profiles.Categories("linode_account_user_grants_update"), "core")
	assert.Contains(t, profiles.Categories("linode_account_user_update"), "core")
	assert.Contains(t, profiles.Categories("linode_account_user_delete"), "core")
	assert.Contains(t, profiles.Categories("linode_account_user_create"), "core")
}

func TestCategoriesDatabasesTools(t *testing.T) {
	t.Parallel()

	assert.Contains(t, profiles.Categories("linode_database_engine_list"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_type_list"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_type_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_engine_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_mysql_config_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_config_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_list"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_list"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_ssl_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_ssl_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_credentials_get"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_credentials_reset"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_credentials_reset"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_update"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_delete"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_delete"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_patch"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_patch"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_suspend"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_suspend"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_instance_resume"), "databases")
	assert.Contains(t, profiles.Categories("linode_database_postgresql_instance_resume"), "databases")
}

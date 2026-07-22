package profiles_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
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
		{Name: "linode_account_user_list", Capability: profiles.CapRead},
		{Name: "linode_account_user_update", Capability: profiles.CapAdmin},
		{Name: "linode_account_user_delete", Capability: profiles.CapDestroy},
		{Name: "linode_account_oauth_client_list", Capability: profiles.CapRead},
		{Name: "linode_profile_app_list", Capability: profiles.CapRead},
		{Name: "linode_profile_security_question_list", Capability: profiles.CapRead},
		{Name: "linode_profile_tfa_disable", Capability: profiles.CapAdmin},
		{Name: "linode_profile_tfa_enable_confirm", Capability: profiles.CapAdmin},
		{Name: "linode_profile_token_list", Capability: profiles.CapRead},
		{Name: "linode_profile_token_delete", Capability: profiles.CapDestroy},
		{Name: "linode_profile_token_update", Capability: profiles.CapAdmin},
		{Name: "linode_profile_device_list", Capability: profiles.CapRead},
		{Name: "linode_profile_preferences_update", Capability: profiles.CapWrite},
		{Name: "linode_longview_plan_get", Capability: profiles.CapRead},
		{Name: "linode_longview_subscription_list", Capability: profiles.CapRead},
		{Name: "linode_longview_client_list", Capability: profiles.CapRead},
		{Name: "linode_longview_client_update", Capability: profiles.CapAdmin},
		{Name: "linode_longview_client_delete", Capability: profiles.CapDestroy},

		// Compute reads
		{Name: toolInstancesList, Capability: profiles.CapRead},
		{Name: "linode_instance_get", Capability: profiles.CapRead},
		{Name: "linode_placement_group_assign", Capability: profiles.CapWrite},
		{Name: "linode_placement_group_get", Capability: profiles.CapRead},
		{Name: "linode_placement_group_delete", Capability: profiles.CapDestroy},
		{Name: "linode_region_list", Capability: profiles.CapRead},
		{Name: "linode_region_get", Capability: profiles.CapRead},
		{Name: "linode_region_availability_list", Capability: profiles.CapRead},
		{Name: "linode_region_availability_get", Capability: profiles.CapRead},
		{Name: "linode_placement_group_list", Capability: profiles.CapRead},
		{Name: "linode_placement_group_update", Capability: profiles.CapWrite},
		{Name: "linode_kernel_list", Capability: profiles.CapRead},
		{Name: "linode_kernel_get", Capability: profiles.CapRead},
		{Name: "linode_type_list", Capability: profiles.CapRead},
		{Name: "linode_image_list", Capability: profiles.CapRead},
		{Name: "linode_placement_group_create", Capability: profiles.CapWrite},
		{Name: "linode_placement_group_unassign", Capability: profiles.CapWrite},
		{Name: "linode_image_update", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_get", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_by_image_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_image_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_member_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_member_token_get", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_member_token_update", Capability: profiles.CapWrite},

		{Name: "linode_image_sharegroup_create", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_image_add", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_image_update", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_member_add", Capability: profiles.CapWrite},
		{Name: "linode_image_sharegroup_update", Capability: profiles.CapWrite},

		{Name: "linode_image_sharegroup_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_image_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_token_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_token_get", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_token_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_member_token_delete", Capability: profiles.CapDestroy},
		{Name: "linode_image_sharegroup_token_image_list", Capability: profiles.CapRead},
		{Name: "linode_image_sharegroup_token_update", Capability: profiles.CapAdmin},
		{Name: "linode_image_sharegroup_by_token_get", Capability: profiles.CapRead},
		{Name: "linode_network_transfer_price_list", Capability: profiles.CapRead},
		{Name: "linode_stackscript_get", Capability: profiles.CapRead},
		{Name: "linode_stackscript_list", Capability: profiles.CapRead},

		// Databases reads
		{Name: toolDatabaseEngineList, Capability: profiles.CapRead},
		{Name: toolDatabaseTypeList, Capability: profiles.CapRead},
		{Name: toolDatabaseTypeGet, Capability: profiles.CapRead},
		{Name: toolDatabaseEngineGet, Capability: profiles.CapRead},
		{Name: toolDatabaseMySQLConfigGet, Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_config_get", Capability: profiles.CapRead},
		{Name: toolDatabaseMySQLInstanceList, Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_instance_list", Capability: profiles.CapRead},
		{Name: "linode_database_mysql_instance_get", Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_instance_get", Capability: profiles.CapRead},
		{Name: "linode_database_mysql_instance_ssl_get", Capability: profiles.CapRead},
		{Name: "linode_database_postgresql_instance_ssl_get", Capability: profiles.CapRead},
		{Name: toolDatabaseMySQLCredentialsGet, Capability: profiles.CapAdmin},
		{Name: "linode_database_mysql_instance_credentials_reset", Capability: profiles.CapAdmin},
		{Name: "linode_database_postgresql_instance_credentials_reset", Capability: profiles.CapAdmin},
		{Name: "linode_database_mysql_instance_update", Capability: profiles.CapWrite},
		{Name: toolDatabaseMySQLInstanceDelete, Capability: profiles.CapDestroy},
		{Name: "linode_database_postgresql_instance_delete", Capability: profiles.CapDestroy},
		{Name: "linode_database_mysql_instance_patch", Capability: profiles.CapWrite},
		{Name: "linode_database_postgresql_instance_patch", Capability: profiles.CapWrite},
		{Name: "linode_database_mysql_instance_suspend", Capability: profiles.CapWrite},
		{Name: "linode_database_postgresql_instance_suspend", Capability: profiles.CapWrite},
		{Name: "linode_database_mysql_instance_resume", Capability: profiles.CapWrite},
		{Name: "linode_database_postgresql_instance_resume", Capability: profiles.CapWrite},
		// Compute writes / destroys
		{Name: toolInstanceCreate, Capability: profiles.CapWrite},
		{Name: "linode_stackscript_create", Capability: profiles.CapWrite},
		{Name: "linode_stackscript_delete", Capability: profiles.CapDestroy},
		{Name: "linode_stackscript_update", Capability: profiles.CapWrite},
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

		// Compute deep (backups, configs, disks, IPs)
		{Name: "linode_instance_backup_list", Capability: profiles.CapRead},
		{Name: "linode_instance_stats_get", Capability: profiles.CapRead},
		{Name: "linode_instance_backup_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_firewall_apply", Capability: profiles.CapWrite},
		{Name: "linode_instance_interface_upgrade", Capability: profiles.CapWrite},
		{Name: "linode_instance_interface_firewall_list", Capability: profiles.CapRead},
		{Name: "linode_instance_interface_settings_get", Capability: profiles.CapRead},
		{Name: "linode_instance_interface_settings_update", Capability: profiles.CapWrite},
		{Name: "linode_instance_interface_history_list", Capability: profiles.CapRead},
		{Name: "linode_instance_interface_update", Capability: profiles.CapWrite},
		{Name: toolLinodeInstanceConfigList, Capability: profiles.CapRead},
		{Name: "linode_instance_config_interface_list", Capability: profiles.CapRead},
		{Name: "linode_instance_config_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_config_interface_get", Capability: profiles.CapRead},
		{Name: "linode_instance_config_interface_delete", Capability: profiles.CapDestroy},
		{Name: "linode_instance_config_update", Capability: profiles.CapWrite},
		{Name: "linode_instance_config_interface_reorder", Capability: profiles.CapWrite},
		{Name: "linode_instance_config_delete", Capability: profiles.CapDestroy},
		{Name: "linode_instance_firewall_list", Capability: profiles.CapRead},
		{Name: "linode_instance_disk_list", Capability: profiles.CapRead},
		{Name: "linode_instance_disk_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_disk_delete", Capability: profiles.CapDestroy},
		{Name: "linode_instance_ip_list", Capability: profiles.CapRead},
		{Name: "linode_instance_ip_allocate", Capability: profiles.CapWrite},
		{Name: "linode_instance_ip_update", Capability: profiles.CapWrite},

		// Block storage
		{Name: toolVolumesList, Capability: profiles.CapRead},
		{Name: toolVolumeTypeList, Capability: profiles.CapRead},
		{Name: toolVolumeCreate, Capability: profiles.CapWrite},
		{Name: toolVolumeClone, Capability: profiles.CapWrite},
		{Name: toolVolumeDelete, Capability: profiles.CapDestroy},
		{Name: toolVolumeResize, Capability: profiles.CapWrite},

		// Object storage
		{Name: "linode_object_storage_bucket_list", Capability: profiles.CapRead},
		{Name: "linode_object_storage_bucket_by_region_list", Capability: profiles.CapRead},
		{Name: "linode_object_storage_bucket_create", Capability: profiles.CapWrite},
		{Name: "linode_object_storage_bucket_delete", Capability: profiles.CapDestroy},
		{Name: "linode_object_storage_cancel", Capability: profiles.CapAdmin},

		// Networking
		{Name: "linode_firewall_list", Capability: profiles.CapRead},
		{Name: "linode_vlan_list", Capability: profiles.CapRead},
		{Name: "linode_vlan_delete", Capability: profiles.CapDestroy},
		{Name: "linode_firewall_rules_get", Capability: profiles.CapRead},
		{Name: "linode_firewall_rules_update", Capability: profiles.CapWrite},
		{Name: "linode_firewall_rule_version_list", Capability: profiles.CapRead},
		{Name: "linode_firewall_create", Capability: profiles.CapWrite},
		{Name: "linode_firewall_delete", Capability: profiles.CapDestroy},
		{Name: "linode_nodebalancer_type_list", Capability: profiles.CapRead},
		{Name: "linode_nodebalancer_list", Capability: profiles.CapRead},
		{Name: "linode_nodebalancer_config_get", Capability: profiles.CapRead},
		{Name: "linode_nodebalancer_config_rebuild", Capability: profiles.CapWrite},
		{Name: "linode_nodebalancer_create", Capability: profiles.CapWrite},
		{Name: "linode_nodebalancer_config_node_update", Capability: profiles.CapWrite},
		{Name: "linode_networking_ip_get", Capability: profiles.CapRead},
		{Name: "linode_networking_ip_update", Capability: profiles.CapWrite},
		{Name: "linode_networking_ip_allocate", Capability: profiles.CapWrite},
		{Name: "linode_networking_ip_assign", Capability: profiles.CapWrite},
		{Name: "linode_networking_ipv4_assign", Capability: profiles.CapWrite},
		{Name: "linode_networking_ipv4_share", Capability: profiles.CapWrite},
		{Name: "linode_ipv6_pool_list", Capability: profiles.CapRead},
		{Name: "linode_ipv6_range_list", Capability: profiles.CapRead},
		{Name: "linode_ipv6_range_get", Capability: profiles.CapRead},
		{Name: "linode_ipv6_range_create", Capability: profiles.CapWrite},
		{Name: "linode_ipv6_range_delete", Capability: profiles.CapDestroy},

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
		{Name: "linode_profile_security_question_answer", Capability: profiles.CapAdmin},
		{Name: "linode_firewall_settings_update", Capability: profiles.CapAdmin},
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
		if !found {
			t.Fatal("found = false, want true")
		}

		if len(profile.AllowedTools) == 0 {
			t.Error("profile.AllowedTools is empty")
		}
	}
}

func TestDefaultProfileContainsOnlyReadAndMeta(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	builtins := profiles.BuiltinProfiles(catalog)

	want := readAndMetaNames(catalog)
	got := builtins[profiles.BuiltinDefault].AllowedTools

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %v, want %v", got, want)
	}

	gotReadonly := builtins[profiles.BuiltinReadonlyFull].AllowedTools
	if !reflect.DeepEqual(gotReadonly, want) {
		t.Errorf("gotReadonly = %v, want %v", gotReadonly, want)
	}
}

func TestEmergencyAllowsYolo(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())

	if !builtins[profiles.BuiltinEmergency].AllowYolo {
		t.Error("builtins[profiles.BuiltinEmergency].AllowYolo = false, want true")
	}

	if builtins[profiles.BuiltinDefault].AllowYolo {
		t.Error("builtins[profiles.BuiltinDefault].AllowYolo = true, want false")
	}

	if builtins[profiles.BuiltinFullAccess].AllowYolo {
		t.Error("builtins[profiles.BuiltinFullAccess].AllowYolo = true, want false")
	}
}

func TestFullAccessAndEmergencyDisabled(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())

	if !builtins[profiles.BuiltinFullAccess].Disabled {
		t.Error("builtins[profiles.BuiltinFullAccess].Disabled = false, want true")
	}

	if !builtins[profiles.BuiltinEmergency].Disabled {
		t.Error("builtins[profiles.BuiltinEmergency].Disabled = false, want true")
	}

	if builtins[profiles.BuiltinDefault].Disabled {
		t.Error("builtins[profiles.BuiltinDefault].Disabled = true, want false")
	}

	if builtins[profiles.BuiltinComputeAdmin].Disabled {
		t.Error("builtins[profiles.BuiltinComputeAdmin].Disabled = true, want false")
	}
}

func TestComputeAdminIncludesInstanceWrites(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())
	allowed := builtins[profiles.BuiltinComputeAdmin].AllowedTools

	if !slices.Contains(allowed, toolInstanceCreate) {
		t.Errorf("allowed does not contain %v", toolInstanceCreate)
	}

	if !slices.Contains(allowed, "linode_instance_delete") {
		t.Errorf("allowed does not contain %v", "linode_instance_delete")
	}

	if !slices.Contains(allowed, toolVolumeCreate) {
		t.Errorf("allowed does not contain %v", toolVolumeCreate)
	}

	if !slices.Contains(allowed, "linode_sshkey_create") {
		t.Errorf("allowed does not contain %v", "linode_sshkey_create")
	}

	if !slices.Contains(allowed, "linode_instance_backup_create") {
		t.Errorf("allowed does not contain %v", "linode_instance_backup_create")
	}
}

func TestNetworkAdminExcludesComputeWrites(t *testing.T) {
	t.Parallel()

	builtins := profiles.BuiltinProfiles(syntheticCatalog())
	allowed := builtins[profiles.BuiltinNetworkAdmin].AllowedTools

	if slices.Contains(allowed, toolInstanceCreate) {
		t.Errorf("allowed should not contain %v", toolInstanceCreate)
	}

	if slices.Contains(allowed, toolVolumeCreate) {
		t.Errorf("allowed should not contain %v", toolVolumeCreate)
	}

	if !slices.Contains(allowed, "linode_firewall_create") {
		t.Errorf("allowed does not contain %v", "linode_firewall_create")
	}

	if !slices.Contains(allowed, "linode_networking_ip_get") {
		t.Errorf("allowed does not contain %v", "linode_networking_ip_get")
	}

	if !slices.Contains(allowed, "linode_networking_ip_update") {
		t.Errorf("allowed does not contain %v", "linode_networking_ip_update")
	}

	if !slices.Contains(allowed, "linode_networking_ip_allocate") {
		t.Errorf("allowed does not contain %v", "linode_networking_ip_allocate")
	}

	if !slices.Contains(allowed, "linode_networking_ip_assign") {
		t.Errorf("allowed does not contain %v", "linode_networking_ip_assign")
	}

	if !slices.Contains(allowed, "linode_networking_ipv4_assign") {
		t.Errorf("allowed does not contain %v", "linode_networking_ipv4_assign")
	}

	if !slices.Contains(allowed, "linode_networking_ipv4_share") {
		t.Errorf("allowed does not contain %v", "linode_networking_ipv4_share")
	}

	if !slices.Contains(allowed, "linode_domain_create") {
		t.Errorf("allowed does not contain %v", "linode_domain_create")
	}

	if !slices.Contains(allowed, "linode_vpc_create") {
		t.Errorf("allowed does not contain %v", "linode_vpc_create")
	}
}

// TestCapAdminOnlyInWildcardBuiltins pins the Admin-tier wiring: Admin tools
// (account/child_account-scope operations) appear only in the full-access and
// emergency built-ins (the wildcard profiles) and in no other built-in. Every
// Admin tool in the catalog is checked against every built-in.
func TestCapAdminOnlyInWildcardBuiltins(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	builtins := profiles.BuiltinProfiles(catalog)

	var adminTools []string

	for _, descriptor := range catalog {
		if descriptor.Capability == profiles.CapAdmin {
			adminTools = append(adminTools, descriptor.Name)
		}
	}

	if len(adminTools) == 0 {
		t.Fatal("synthetic catalog has no CapAdmin tools; test would be vacuous")
	}

	grantsAdmin := map[string]bool{
		profiles.BuiltinFullAccess: true,
		profiles.BuiltinEmergency:  true,
	}

	for name, profile := range builtins {
		for _, adminTool := range adminTools {
			got := slices.Contains(profile.AllowedTools, adminTool)
			if want := grantsAdmin[name]; got != want {
				t.Errorf("profile %q: admin tool %q present=%v, want %v", name, adminTool, got, want)
			}
		}
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

	for _, prof := range built {
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

		if len(prof.RequiredTokenScopes) != len(expected) {
			t.Errorf("len(prof.RequiredTokenScopes) = %d, want %d", len(prof.RequiredTokenScopes), len(expected))
		}

		for _, scope := range prof.RequiredTokenScopes {
			_, found := expected[scope]
			if !found {
				t.Error("found = false, want true")
			}
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
		if !ok {
			t.Fatal("ok = false, want true")
		}

		for _, scope := range prof.RequiredTokenScopes {
			if strings.Contains(scope, ":read_write") {
				t.Errorf("profile %s is read-only but lists a write scope %q", name, scope)
			}
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
	if !ok {
		t.Fatal("ok = false, want true")
	}

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
		if !slices.Contains(full.RequiredTokenScopes, scope) {
			t.Errorf("full.RequiredTokenScopes does not contain %v", scope)
		}
	}
}

// TestRequiredTokenScopesSorted locks in the sort order so cross-language
// parity comparison via BuiltinCatalogJSON stays stable.
func TestRequiredTokenScopesSorted(t *testing.T) {
	t.Parallel()

	built := profiles.BuiltinProfiles(syntheticCatalog())

	for _, prof := range built {
		if !slices.IsSorted(prof.RequiredTokenScopes) {
			t.Error("slices.IsSorted(prof.RequiredTokenScopes) = false, want true")
		}
	}
}

func TestJSONRoundtrip(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()

	data, err := profiles.BuiltinCatalogJSON(catalog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("data is empty")
	}

	var decoded []map[string]any

	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decoded) != 8 {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), 8)
	}

	names := make([]string, 0, len(decoded))
	for _, entry := range decoded {
		name, ok := entry["name"].(string)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		names = append(names, name)
	}

	if !slices.IsSorted(names) {
		t.Error("slices.IsSorted(names) = false, want true")
	}

	// Re-marshal and require byte-identical output to confirm determinism.
	second, err := profiles.BuiltinCatalogJSON(catalog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(second, data) {
		t.Errorf("second = %v, want %v", second, data)
	}
}

func TestCategoriesIncludesAccountInvoicesInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_account_invoice_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_payment_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_payment_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_promo_credit_add"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_invoice_item_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesAccountPaymentMethodsInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_account_payment_method_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_payment_method_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_payment_method_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_payment_method_delete"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_payment_method_make_default"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesProfilePreferencesInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_profile_preferences_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_security_question_answer"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesProfileTokenCreateInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_profile_token_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesAccountOAuthClientsInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_account_oauth_client_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesProfileDeviceGetInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_profile_device_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesProfileAppsInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_profile_login_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_tfa_enable"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_phone_number_send"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_phone_number_delete"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_phone_number_verify"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_tfa_disable"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_tfa_enable_confirm"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_app_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_app_delete"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_device_revoke"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_app_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_security_question_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_device_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_preferences_update"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesMaintenancePoliciesInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_maintenance_policy_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesTagCreateInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_tag_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestTagCreateExcludedFromNarrowBuiltinProfiles(t *testing.T) {
	t.Parallel()

	registry := []profiles.ToolDescriptor{{Name: "linode_tag_create", Capability: profiles.CapWrite}}
	for _, profileName := range []string{profiles.BuiltinDefault, profiles.BuiltinReadonlyFull, profiles.BuiltinComputeAdmin, profiles.BuiltinNetworkAdmin, profiles.BuiltinKubernetesAdmin, profiles.BuiltinStorageAdmin} {
		profile, err := profiles.ResolveActiveProfile(&config.Config{ActiveProfile: profileName}, registry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if slices.Contains(profile.AllowedTools, "linode_tag_create") {
			t.Errorf("profile.AllowedTools should not contain %v", "linode_tag_create")
		}
	}
}

func TestCategoriesIncludesAccountUsersInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_account_user_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_user_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_profile_token_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_user_grants_get"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_user_grants_update"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_user_update"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_user_delete"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_account_user_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_support_ticket_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_support_ticket_attachment_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_support_ticket_reply_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_support_ticket_close"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_managed_contact_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}

	if !slices.Contains(profiles.Categories("linode_managed_service_create"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

func TestCategoriesIncludesLongviewClientsInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_longview_client_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}

func TestCategoriesIncludesLongviewSubscriptionsInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_longview_subscription_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}

func TestCategoriesIncludesMonitorServicesInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_monitor_service_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_service_get"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_service_dashboard_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_service_metric_query"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_service_metric_definition_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}

func TestCategoriesIncludesMonitorAlertDefinitionsInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_monitor_alert_definition_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}

func TestCategoriesIncludesMonitorAlertChannelsInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_monitor_alert_channel_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}

func TestCategoriesDatabasesTools(t *testing.T) {
	t.Parallel()

	for _, tool := range []string{
		toolDatabaseEngineList,
		toolDatabaseTypeList,
		toolDatabaseTypeGet,
		toolDatabaseEngineGet,
		toolDatabaseMySQLConfigGet,
		"linode_database_postgresql_config_get",
		toolDatabaseMySQLInstanceList,
		"linode_database_postgresql_instance_list",
		"linode_database_mysql_instance_get",
		"linode_database_postgresql_instance_get",
		"linode_database_mysql_instance_ssl_get",
		"linode_database_postgresql_instance_ssl_get",
		toolDatabaseMySQLCredentialsGet,
		"linode_database_mysql_instance_credentials_reset",
		"linode_database_postgresql_instance_credentials_reset",
		"linode_database_mysql_instance_update",
		toolDatabaseMySQLInstanceDelete,
		"linode_database_postgresql_instance_delete",
		"linode_database_mysql_instance_patch",
		"linode_database_postgresql_instance_patch",
		"linode_database_mysql_instance_suspend",
		"linode_database_postgresql_instance_suspend",
		"linode_database_mysql_instance_resume",
		"linode_database_postgresql_instance_resume",
	} {
		if !slices.Contains(profiles.Categories(tool), "databases") {
			t.Errorf("Categories(%s) does not contain databases", tool)
		}
	}
}

func TestCategoriesIncludesPlacementGroupsInCompute(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_placement_group_assign"), "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}

	if !slices.Contains(profiles.Categories("linode_placement_group_get"), "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}

	if !slices.Contains(profiles.Categories("linode_placement_group_delete"), "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}

	if !slices.Contains(profiles.Categories("linode_placement_group_list"), "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}

	if !slices.Contains(profiles.Categories("linode_placement_group_update"), "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}

	if !slices.Contains(profiles.Categories("linode_placement_group_unassign"), "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}
}

func TestCategoriesIncludesTagsInCore(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_tag_list"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

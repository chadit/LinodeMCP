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
		{Name: "linode_profile", Capability: profiles.CapRead},
		{Name: "linode_account", Capability: profiles.CapRead},

		// Compute reads
		{Name: "linode_instances_list", Capability: profiles.CapRead},
		{Name: "linode_instance_get", Capability: profiles.CapRead},
		{Name: "linode_regions_list", Capability: profiles.CapRead},
		{Name: "linode_types_list", Capability: profiles.CapRead},
		{Name: "linode_images_list", Capability: profiles.CapRead},
		{Name: "linode_stackscripts_list", Capability: profiles.CapRead},
		// Compute writes / destroys
		{Name: "linode_instance_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_delete", Capability: profiles.CapDestroy},
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
		{Name: "linode_instance_backups_list", Capability: profiles.CapRead},
		{Name: "linode_instance_backup_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_disks_list", Capability: profiles.CapRead},
		{Name: "linode_instance_disk_create", Capability: profiles.CapWrite},
		{Name: "linode_instance_disk_delete", Capability: profiles.CapDestroy},
		{Name: "linode_instance_ips_list", Capability: profiles.CapRead},
		{Name: "linode_instance_ip_allocate", Capability: profiles.CapWrite},

		// Block storage
		{Name: toolVolumesList, Capability: profiles.CapRead},
		{Name: toolVolumeCreate, Capability: profiles.CapWrite},
		{Name: toolVolumeDelete, Capability: profiles.CapDestroy},
		{Name: toolVolumeResize, Capability: profiles.CapWrite},

		// Object storage
		{Name: "linode_object_storage_buckets_list", Capability: profiles.CapRead},
		{Name: "linode_object_storage_bucket_create", Capability: profiles.CapWrite},
		{Name: "linode_object_storage_bucket_delete", Capability: profiles.CapDestroy},

		// Networking
		{Name: "linode_firewalls_list", Capability: profiles.CapRead},
		{Name: "linode_firewall_create", Capability: profiles.CapWrite},
		{Name: "linode_firewall_delete", Capability: profiles.CapDestroy},
		{Name: "linode_nodebalancers_list", Capability: profiles.CapRead},
		{Name: "linode_nodebalancer_create", Capability: profiles.CapWrite},

		// DNS
		{Name: "linode_domains_list", Capability: profiles.CapRead},
		{Name: "linode_domain_create", Capability: profiles.CapWrite},
		{Name: "linode_domain_delete", Capability: profiles.CapDestroy},
		{Name: "linode_domain_record_create", Capability: profiles.CapWrite},

		// LKE
		{Name: "linode_lke_clusters_list", Capability: profiles.CapRead},
		{Name: "linode_lke_cluster_create", Capability: profiles.CapWrite},
		{Name: "linode_lke_cluster_delete", Capability: profiles.CapDestroy},

		// VPCs
		{Name: "linode_vpcs_list", Capability: profiles.CapRead},
		{Name: "linode_vpc_create", Capability: profiles.CapWrite},
		{Name: "linode_vpc_delete", Capability: profiles.CapDestroy},

		// Security (SSH keys)
		{Name: "linode_sshkeys_list", Capability: profiles.CapRead},
		{Name: "linode_sshkey_create", Capability: profiles.CapWrite},
		{Name: "linode_sshkey_delete", Capability: profiles.CapDestroy},

		// A tool with CapAdmin (not yet used by any built-in)
		{Name: "linode_account_update", Capability: profiles.CapAdmin},
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

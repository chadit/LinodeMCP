package profiles_test

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// syntheticCatalog returns a hand-built tool catalog that covers every
// category and capability the built-in profile rules touch. The fixture is
// intentionally independent of the live server registry so these tests stay
// stable as Phase 1 capability tags get filled in across the real tools.
// Assembled from three chunks so no one literal sinks the file's
// maintainability score.
func syntheticCatalog() []profiles.ToolDescriptor {
	catalog := syntheticCoreAndCompute()
	catalog = append(catalog, syntheticDataServices()...)
	catalog = append(catalog, syntheticInfrastructure()...)

	return catalog
}

func syntheticCoreAndCompute() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		// Core / meta
		{Name: "hello", Capability: profiles.CapMeta, Categories: []string{categoryCore}},
		{Name: "version", Capability: profiles.CapMeta, Categories: []string{categoryCore}},
		{Name: toolProfile, Capability: profiles.CapRead, Categories: []string{categoryCore}},
		{Name: toolAccount, Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryCore}},
		{Name: "linode_account_user_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryAccount}},
		{Name: "linode_account_user_update", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_account_user_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_account_oauth_client_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_app_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_security_question_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_tfa_disable", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_tfa_enable_confirm", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_token_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_token_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_token_update", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_device_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_preferences_update", Capability: profiles.CapWrite, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_longview_plan_get", Capability: profiles.CapRead, Scopes: []string{scopeLongviewReadOnly}, Categories: []string{categoryLongview}},
		{Name: "linode_longview_subscription_list", Capability: profiles.CapRead, Categories: []string{categoryLongview}},
		{Name: "linode_longview_client_list", Capability: profiles.CapRead, Scopes: []string{scopeLongviewReadOnly}, Categories: []string{categoryLongview}},
		{Name: "linode_longview_client_update", Capability: profiles.CapAdmin, Scopes: []string{scopeLongviewReadWrite}, Categories: []string{categoryLongview}},
		{Name: "linode_longview_client_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLongviewReadWrite}, Categories: []string{categoryLongview}},

		// Compute reads
		{Name: toolInstancesList, Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_get", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_assign", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_get", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_region_list", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_region_get", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_region_availability_list", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_region_availability_get", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_update", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_kernel_list", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_kernel_get", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_type_list", Capability: profiles.CapRead, Categories: []string{categoryCompute}},
		{Name: "linode_image_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_create", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_placement_group_unassign", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_update", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_get", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_by_image_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_image_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_member_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_member_token_get", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_member_token_update", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},

		{Name: "linode_image_sharegroup_create", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_image_add", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_image_update", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_member_add", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_update", Capability: profiles.CapWrite, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},

		{Name: "linode_image_sharegroup_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_image_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_token_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_token_get", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_token_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_member_token_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_token_image_list", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_token_update", Capability: profiles.CapAdmin, Scopes: []string{scopeImagesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_image_sharegroup_by_token_get", Capability: profiles.CapRead, Scopes: []string{scopeImagesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_network_transfer_price_list", Capability: profiles.CapRead, Categories: []string{categoryNetworking}},
		{Name: "linode_stackscript_get", Capability: profiles.CapRead, Scopes: []string{scopeStackscriptsReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_stackscript_list", Capability: profiles.CapRead, Scopes: []string{scopeStackscriptsReadOnly}, Categories: []string{categoryCompute}},
	}
}

func syntheticDataServices() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		// Databases reads
		{Name: toolDatabaseEngineList, Capability: profiles.CapRead, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseTypeList, Capability: profiles.CapRead, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseTypeGet, Capability: profiles.CapRead, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseEngineGet, Capability: profiles.CapRead, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseMySQLConfigGet, Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_config_get", Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseMySQLInstanceList, Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_list", Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_get", Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_get", Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_ssl_get", Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_ssl_get", Capability: profiles.CapRead, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseMySQLCredentialsGet, Capability: profiles.CapAdmin, Scopes: []string{scopeDatabasesReadOnly}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_credentials_reset", Capability: profiles.CapAdmin, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_credentials_reset", Capability: profiles.CapAdmin, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_update", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: toolDatabaseMySQLInstanceDelete, Capability: profiles.CapDestroy, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_patch", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_patch", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_suspend", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_suspend", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_mysql_instance_resume", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		{Name: "linode_database_postgresql_instance_resume", Capability: profiles.CapWrite, Scopes: []string{scopeDatabasesReadWrite}, Categories: []string{categoryDatabases}},
		// Compute writes / destroys
		{Name: toolInstanceCreate, Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_stackscript_create", Capability: profiles.CapWrite, Scopes: []string{scopeStackscriptsReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_stackscript_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeStackscriptsReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_stackscript_update", Capability: profiles.CapWrite, Scopes: []string{scopeStackscriptsReadWrite}, Categories: []string{categoryCompute}},
		{Name: toolInstanceDelete, Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_boot", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_reboot", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_shutdown", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_resize", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_clone", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_migrate", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_rebuild", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_rescue", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_password_reset", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},

		// Compute deep (backups, configs, disks, IPs)
		{Name: "linode_instance_backup_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_stats_get", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_backup_create", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_firewall_apply", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_interface_upgrade", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_interface_firewall_list", Capability: profiles.CapRead, Scopes: []string{scopeNodebalancersReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_interface_settings_get", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_interface_settings_update", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_interface_history_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_interface_update", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: toolLinodeInstanceConfigList, Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_interface_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_create", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_interface_get", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_interface_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_update", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_interface_reorder", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_config_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_firewall_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryCompute}},
		{Name: "linode_instance_disk_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_disk_create", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_disk_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_ip_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_ip_allocate", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_ip_update", Capability: profiles.CapWrite, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryComputeDeep, categoryCompute}},
	}
}

func syntheticInfrastructure() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		// Block storage
		{Name: toolVolumesList, Capability: profiles.CapRead, Scopes: []string{scopeVolumesReadOnly}, Categories: []string{categoryBlockStorage}},
		{Name: toolVolumeTypeList, Capability: profiles.CapRead, Categories: []string{categoryBlockStorage}},
		{Name: toolVolumeCreate, Capability: profiles.CapWrite, Scopes: []string{scopeVolumesReadWrite}, Categories: []string{categoryBlockStorage}},
		{Name: toolVolumeClone, Capability: profiles.CapWrite, Scopes: []string{scopeVolumesReadWrite}, Categories: []string{categoryBlockStorage}},
		{Name: toolVolumeDelete, Capability: profiles.CapDestroy, Scopes: []string{scopeVolumesReadWrite}, Categories: []string{categoryBlockStorage}},
		{Name: toolVolumeResize, Capability: profiles.CapWrite, Scopes: []string{scopeVolumesReadWrite}, Categories: []string{categoryBlockStorage}},

		// Object storage
		{Name: "linode_object_storage_bucket_list", Capability: profiles.CapRead, Scopes: []string{scopeObjectStorageReadOnly}, Categories: []string{categoryObjectStorage}},
		{Name: "linode_object_storage_bucket_by_region_list", Capability: profiles.CapRead, Scopes: []string{scopeObjectStorageReadOnly}, Categories: []string{categoryObjectStorage}},
		{Name: "linode_object_storage_bucket_create", Capability: profiles.CapWrite, Scopes: []string{scopeObjectStorageReadWrite}, Categories: []string{categoryObjectStorage}},
		{Name: "linode_object_storage_bucket_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeObjectStorageReadWrite}, Categories: []string{categoryObjectStorage}},
		{Name: "linode_object_storage_cancel", Capability: profiles.CapAdmin, Scopes: []string{scopeObjectStorageReadWrite}, Categories: []string{categoryObjectStorage}},

		// Networking
		{Name: "linode_firewall_list", Capability: profiles.CapRead, Scopes: []string{scopeFirewallReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_vlan_list", Capability: profiles.CapRead, Scopes: []string{scopeLinodesReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_vlan_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLinodesReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_firewall_rules_get", Capability: profiles.CapRead, Scopes: []string{scopeFirewallReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_firewall_rules_update", Capability: profiles.CapWrite, Scopes: []string{scopeFirewallReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_firewall_rule_version_list", Capability: profiles.CapRead, Scopes: []string{scopeFirewallReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_firewall_create", Capability: profiles.CapWrite, Scopes: []string{scopeFirewallReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_firewall_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeFirewallReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_nodebalancer_type_list", Capability: profiles.CapRead, Categories: []string{categoryNetworking}},
		{Name: "linode_nodebalancer_list", Capability: profiles.CapRead, Scopes: []string{scopeNodebalancersReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_nodebalancer_config_get", Capability: profiles.CapRead, Scopes: []string{scopeNodebalancersReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_nodebalancer_config_rebuild", Capability: profiles.CapWrite, Scopes: []string{scopeNodebalancersReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_nodebalancer_create", Capability: profiles.CapWrite, Scopes: []string{scopeNodebalancersReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_nodebalancer_config_node_update", Capability: profiles.CapWrite, Scopes: []string{scopeNodebalancersReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_ip_get", Capability: profiles.CapRead, Scopes: []string{scopeIPsReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_ip_update", Capability: profiles.CapWrite, Scopes: []string{scopeIPsReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_ip_allocate", Capability: profiles.CapWrite, Scopes: []string{scopeIPsReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_ip_assign", Capability: profiles.CapWrite, Scopes: []string{scopeIPsReadWrite, scopeLinodesReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_ipv4_assign", Capability: profiles.CapWrite, Scopes: []string{scopeIPsReadWrite, scopeLinodesReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_ipv4_share", Capability: profiles.CapWrite, Scopes: []string{scopeIPsReadWrite, scopeLinodesReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_ipv6_pool_list", Capability: profiles.CapRead, Scopes: []string{scopeIPsReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_ipv6_range_list", Capability: profiles.CapRead, Scopes: []string{scopeIPsReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_ipv6_range_get", Capability: profiles.CapRead, Scopes: []string{scopeIPsReadOnly}, Categories: []string{categoryNetworking}},
		{Name: "linode_ipv6_range_create", Capability: profiles.CapWrite, Scopes: []string{scopeIPsReadWrite, scopeLinodesReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_ipv6_range_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeIPsReadWrite}, Categories: []string{categoryNetworking}},

		// DNS
		{Name: "linode_domain_list", Capability: profiles.CapRead, Scopes: []string{scopeDomainsReadOnly}, Categories: []string{categoryDNS}},
		{Name: "linode_domain_create", Capability: profiles.CapWrite, Scopes: []string{scopeDomainsReadWrite}, Categories: []string{categoryDNS}},
		{Name: "linode_domain_clone", Capability: profiles.CapWrite, Scopes: []string{scopeDomainsReadWrite}, Categories: []string{categoryDNS}},
		{Name: "linode_domain_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeDomainsReadWrite}, Categories: []string{categoryDNS}},
		{Name: "linode_domain_record_create", Capability: profiles.CapWrite, Scopes: []string{scopeDomainsReadWrite}, Categories: []string{categoryDNS}},

		// LKE
		{Name: "linode_lke_cluster_list", Capability: profiles.CapRead, Scopes: []string{scopeLKEReadOnly}, Categories: []string{categoryLKE}},
		{Name: "linode_lke_cluster_create", Capability: profiles.CapWrite, Scopes: []string{scopeLKEReadWrite}, Categories: []string{categoryLKE}},
		{Name: "linode_lke_cluster_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeLKEReadWrite}, Categories: []string{categoryLKE}},

		// VPCs
		{Name: "linode_vpc_list", Capability: profiles.CapRead, Categories: []string{categoryVPCs}},
		{Name: "linode_vpc_create", Capability: profiles.CapWrite, Scopes: []string{scopeVPCReadWrite}, Categories: []string{categoryVPCs}},
		{Name: "linode_vpc_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeVPCReadWrite}, Categories: []string{categoryVPCs}},

		// Security (SSH keys)
		{Name: "linode_sshkey_list", Capability: profiles.CapRead, Scopes: []string{scopeAccountReadOnly}, Categories: []string{categorySecurity}},
		{Name: "linode_sshkey_create", Capability: profiles.CapWrite, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categorySecurity}},
		{Name: "linode_sshkey_update", Capability: profiles.CapWrite, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categorySecurity}},
		{Name: "linode_sshkey_delete", Capability: profiles.CapDestroy, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categorySecurity}},

		// Account admin tools must stay out of built-in profiles.
		{Name: "linode_account_update", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_account_settings_update", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_account_settings_managed_enable", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_profile_security_question_answer", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_firewall_settings_update", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryNetworking}},
		{Name: "linode_account_beta_enroll", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
		{Name: "linode_account_cancel", Capability: profiles.CapAdmin, Scopes: []string{scopeAccountReadWrite}, Categories: []string{categoryAccount}},
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
		profiles.BuiltinIamAdmin,
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
// deduplicated, sorted union of the catalog's declared per-tool scopes
// over its AllowedTools. This pins the derivation against drift between
// the catalog and the built-in blueprint and catches regressions where
// someone restores hardcoded scope lists by mistake.
func TestRequiredTokenScopesDerivedFromTools(t *testing.T) {
	t.Parallel()

	catalog := syntheticCatalog()
	built := profiles.BuiltinProfiles(catalog)

	scopesByName := make(map[string][]string, len(catalog))
	for _, d := range catalog {
		scopesByName[d.Name] = d.Scopes
	}

	for _, prof := range built {
		expected := make(map[string]struct{}, len(prof.AllowedTools))

		for _, toolName := range prof.AllowedTools {
			for _, scope := range scopesByName[toolName] {
				expected[scope] = struct{}{}
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
		"linodes:read_write",
		"volumes:read_write",
		"domains:read_write",
		"firewall:read_write",
		"nodebalancers:read_write",
		"lke:read_write",
		"object_storage:read_write",
		"vpc:read_write",
		"stackscripts:read_write",
		"images:read_only",
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

	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if len(decoded) != builtinProfileCount {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), builtinProfileCount)
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

// Everything the API gates on account:* belongs to the account category, the
// one an account-scoped admin profile would elevate. Calling these core left
// them in a bucket no profile can lift, while Python filed the same tools under
// account.
func TestCategoriesFileAccountGatedToolsUnderAccount(t *testing.T) {
	t.Parallel()

	for _, toolName := range []string{
		"linode_account_beta_get",
		"linode_account_beta_list",
		"linode_account_event_get",
		"linode_account_invoice_item_list",
		"linode_account_invoice_list",
		"linode_account_notification_list",
		"linode_account_oauth_client_list",
		"linode_account_payment_create",
		"linode_account_payment_list",
		"linode_account_payment_method_create",
		"linode_account_payment_method_delete",
		"linode_account_payment_method_get",
		"linode_account_payment_method_list",
		"linode_account_payment_method_make_default",
		"linode_account_promo_credit_add",
		"linode_account_user_create",
		"linode_account_user_delete",
		"linode_account_user_get",
		"linode_account_user_grants_get",
		"linode_account_user_grants_update",
		"linode_account_user_list",
		"linode_account_user_update",
		"linode_beta_list",
		"linode_lock_create",
		"linode_lock_list",
		"linode_maintenance_policy_list",
		"linode_managed_contact_create",
		"linode_managed_credential_revoke",
		"linode_managed_service_create",
		"linode_profile_app_delete",
		"linode_profile_app_get",
		"linode_profile_device_get",
		"linode_profile_device_revoke",
		"linode_profile_grants_get",
		"linode_profile_login_get",
		"linode_profile_phone_number_delete",
		"linode_profile_phone_number_send",
		"linode_profile_phone_number_verify",
		"linode_profile_preferences_get",
		toolSecurityQuestionList,
		"linode_profile_tfa_enable",
		"linode_profile_token_create",
		"linode_profile_token_get",
		"linode_profile_update",
		"linode_support_ticket_attachment_create",
		"linode_support_ticket_close",
		"linode_support_ticket_create",
		"linode_support_ticket_get",
		"linode_support_ticket_reply_create",
		"linode_tag_create",
		"linode_tag_delete",
		"linode_tag_list",
		"linode_tag_object_list",
	} {
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			cats := gentools.CategoriesFor(toolName)
			if !slices.Contains(cats, categoryAccount) {
				t.Errorf("CategoriesFor(%q) = %v, want it to contain account", toolName, cats)
			}

			if slices.Contains(cats, categoryCore) {
				t.Errorf("CategoriesFor(%q) = %v, want no core entry", toolName, cats)
			}
		})
	}
}

// Core holds the tools a session starts from and nothing else, so it stays
// these four names however far the account surface grows.
func TestCategoriesHoldCoreToTheSessionTools(t *testing.T) {
	t.Parallel()

	for _, toolName := range []string{toolHello, toolVersion, toolProfile, toolAccount} {
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			if cats := gentools.CategoriesFor(toolName); !reflect.DeepEqual(cats, []string{categoryCore}) {
				t.Errorf("CategoriesFor(%q) = %v, want [core]", toolName, cats)
			}
		})
	}
}

func TestTagCreateExcludedFromNarrowBuiltinProfiles(t *testing.T) {
	t.Parallel()

	registry := []profiles.ToolDescriptor{{Name: "linode_tag_create", Capability: profiles.CapWrite, Categories: []string{categoryAccount}}}
	for _, profileName := range []string{profiles.BuiltinDefault, profiles.BuiltinReadonlyFull, profiles.BuiltinComputeAdmin, profiles.BuiltinNetworkAdmin, profiles.BuiltinKubernetesAdmin, profiles.BuiltinStorageAdmin, profiles.BuiltinIamAdmin} {
		profile, err := profiles.ResolveActiveProfile(&config.Config{ActiveProfile: profileName}, registry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if slices.Contains(profile.AllowedTools, "linode_tag_create") {
			t.Errorf("profile.AllowedTools should not contain %v", "linode_tag_create")
		}
	}
}

func TestCategoriesIncludesMonitorServicesInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_service_list"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_service_get"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_service_dashboard_list"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_service_metric_query"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_service_metric_definition_list"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}
}

func TestCategoriesIncludesMonitorAlertDefinitionsInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_alert_definition_list"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}
}

func TestCategoriesIncludesMonitorAlertChannelsInMonitor(t *testing.T) {
	t.Parallel()

	if !slices.Contains(gentools.CategoriesFor("linode_monitor_alert_channel_list"), categoryMonitor) {
		t.Errorf("collection does not contain %v", categoryMonitor)
	}
}

// The iam category exists so a mutating IAM tool resolves the same way in both
// languages. Go alone would serve a category-less mutator through isElevated's
// "*" short-circuit while Python's Write/Destroy branch, which needs a named
// category, would serve it nowhere.
func TestCategoriesIncludesIamToolsInIam(t *testing.T) {
	t.Parallel()

	if !slices.Contains(gentools.CategoriesFor(toolIamIdpConfigDelete), categoryIAM) {
		t.Errorf("collection does not contain %v", categoryIAM)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_iam_delegation_child_account_list"), categoryIAM) {
		t.Errorf("collection does not contain %v", categoryIAM)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_iam_role_permission_list"), categoryIAM) {
		t.Errorf("collection does not contain %v", categoryIAM)
	}
}

// The removal resolves in the two wildcard profiles and in iam-admin, which
// elevates the iam category, and nowhere else. Python's resolver has to reach
// the same answer.
//
// The category is load-bearing here in a way it was not before iam-admin
// existed: the wildcard short-circuits whatever a tool's categories are, so
// dropping the category would leave this destroy reachable only by wildcard
// while every gate stayed green.
func TestIamRemovalResolvesInWildcardAndIamAdmin(t *testing.T) {
	t.Parallel()

	catalog := []profiles.ToolDescriptor{
		{Name: toolIamIdpConfigDelete, Capability: profiles.CapDestroy, Categories: []string{categoryIAM}},
	}

	serving := map[string]bool{
		profiles.BuiltinFullAccess: true,
		profiles.BuiltinEmergency:  true,
		profiles.BuiltinIamAdmin:   true,
	}

	for name, profile := range profiles.BuiltinProfiles(catalog) {
		allowed := slices.Contains(profile.AllowedTools, toolIamIdpConfigDelete)
		if want := serving[name]; allowed != want {
			t.Errorf("profile %s serves the removal = %v, want %v", name, allowed, want)
		}
	}
}

// iamAdminCatalog is what the iam-admin cases resolve against: the nine IAM
// mutators and the two IAM removals beside the entity list, the legacy account
// grants pair, and one compute write. A profile that reaches past its own
// category fails by name rather than by a count.
func iamAdminCatalog() []profiles.ToolDescriptor {
	iam := []string{
		toolIamRolePermissionUpdate,
		"linode_iam_delegation_child_account_user_update",
		"linode_iam_delegation_default_role_permission_update",
		"linode_iam_delegation_profile_child_account_token_create",
		"linode_iam_idp_config_create",
		"linode_iam_idp_config_update",
		"linode_iam_idp_config_certificate_create",
		"linode_iam_idp_config_excluded_user_update",
		"linode_iam_idp_config_included_user_update",
	}

	catalog := make([]profiles.ToolDescriptor, 0, len(iam)+5)
	for _, name := range iam {
		catalog = append(catalog, profiles.ToolDescriptor{
			Name: name, Capability: profiles.CapWrite, Categories: []string{categoryIAM},
		})
	}

	return append(catalog,
		profiles.ToolDescriptor{Name: toolIamIdpConfigDelete, Capability: profiles.CapDestroy, Categories: []string{categoryIAM}},
		profiles.ToolDescriptor{Name: "linode_iam_idp_config_certificate_delete", Capability: profiles.CapDestroy, Categories: []string{categoryIAM}},
		profiles.ToolDescriptor{Name: toolEntityList, Capability: profiles.CapRead, Categories: []string{categoryIAM}},
		profiles.ToolDescriptor{Name: toolAccountUserGrantsGet, Capability: profiles.CapRead, Categories: []string{categoryAccount}},
		profiles.ToolDescriptor{Name: toolAccountUserGrantsUpdate, Capability: profiles.CapAdmin, Categories: []string{categoryAccount}},
		profiles.ToolDescriptor{Name: toolInstanceCreate, Capability: profiles.CapWrite, Categories: []string{categoryCompute}},
	)
}

// iam-admin exists to run Linode IAM: it serves every IAM mutator and removal
// plus the entity list the role assignment needs entity ids from.
func TestIamAdminServesTheWholeIamSurface(t *testing.T) {
	t.Parallel()

	catalog := iamAdminCatalog()
	allowed := profiles.BuiltinProfiles(catalog)[profiles.BuiltinIamAdmin].AllowedTools

	for _, descriptor := range catalog {
		if !slices.Contains(descriptor.Categories, categoryIAM) {
			continue
		}

		if !slices.Contains(allowed, descriptor.Name) {
			t.Errorf("iam-admin does not serve %s", descriptor.Name)
		}
	}

	if !slices.Contains(allowed, toolEntityList) {
		t.Errorf("iam-admin does not serve %s", toolEntityList)
	}
}

// The legacy account grants mutator stays out of iam-admin. Linode documents
// mixing grant-based and role-based access control on one account as a security
// risk, and the grants routes are deprecated, so the profile that manages IAM
// must not also hand over the older system. Nothing here relies on the tier
// alone: the tool is Admin AND filed under account, so a retag to Write would
// still leave it out of an iam-only profile.
//
// The paired read is CapRead, which every built-in serves down to the read-only
// default. That is pinned too, so retagging it into a mutating tier fails here
// rather than quietly widening what iam-admin can do.
func TestIamAdminExcludesTheLegacyGrantsMutator(t *testing.T) {
	t.Parallel()

	built := profiles.BuiltinProfiles(iamAdminCatalog())
	allowed := built[profiles.BuiltinIamAdmin].AllowedTools

	if slices.Contains(allowed, toolAccountUserGrantsUpdate) {
		t.Errorf("iam-admin serves %s, want the legacy grants mutator excluded", toolAccountUserGrantsUpdate)
	}

	if !slices.Contains(allowed, toolAccountUserGrantsGet) {
		t.Errorf("iam-admin does not serve the read %s", toolAccountUserGrantsGet)
	}

	if !slices.Contains(built[profiles.BuiltinDefault].AllowedTools, toolAccountUserGrantsGet) {
		t.Errorf("default does not serve the read %s", toolAccountUserGrantsGet)
	}
}

// A write outside the iam category is unreachable from iam-admin, and the IAM
// writes are unreachable from the sibling category admins.
func TestIamAdminIsScopedToTheIamCategory(t *testing.T) {
	t.Parallel()

	built := profiles.BuiltinProfiles(iamAdminCatalog())

	if slices.Contains(built[profiles.BuiltinIamAdmin].AllowedTools, toolInstanceCreate) {
		t.Errorf("iam-admin serves %s, want compute writes excluded", toolInstanceCreate)
	}

	for _, name := range []string{
		profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
		profiles.BuiltinComputeAdmin,
		profiles.BuiltinNetworkAdmin,
		profiles.BuiltinKubernetesAdmin,
		profiles.BuiltinStorageAdmin,
	} {
		if slices.Contains(built[name].AllowedTools, toolIamRolePermissionUpdate) {
			t.Errorf("profile %s serves %s, want it reachable only from iam-admin and the wildcards", name, toolIamRolePermissionUpdate)
		}
	}
}

// The entity list is a Read in the iam category, so every built-in serves it,
// the read-only default included.
func TestEntityListReadServedByEveryBuiltin(t *testing.T) {
	t.Parallel()

	catalog := []profiles.ToolDescriptor{
		{Name: toolEntityList, Capability: profiles.CapRead, Categories: []string{categoryIAM}},
	}

	for name, profile := range profiles.BuiltinProfiles(catalog) {
		if !slices.Contains(profile.AllowedTools, toolEntityList) {
			t.Errorf("profile %s does not serve %s", name, toolEntityList)
		}
	}
}

// GET /entities is documented under Linode IAM, so the generated table files
// linode_entity_list with its linode_iam_ siblings.
func TestCategoriesFilesEntityListUnderIam(t *testing.T) {
	t.Parallel()

	if got := gentools.CategoriesFor(toolEntityList); !reflect.DeepEqual(got, []string{categoryIAM}) {
		t.Errorf("CategoriesFor(%s) = %v, want [%s]", toolEntityList, got, categoryIAM)
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
		if !slices.Contains(gentools.CategoriesFor(tool), categoryDatabases) {
			t.Errorf("Categories(%s) does not contain databases", tool)
		}
	}
}

func TestCategoriesIncludesPlacementGroupsInCompute(t *testing.T) {
	t.Parallel()

	if !slices.Contains(gentools.CategoriesFor("linode_placement_group_assign"), categoryCompute) {
		t.Errorf("collection does not contain %v", categoryCompute)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_placement_group_get"), categoryCompute) {
		t.Errorf("collection does not contain %v", categoryCompute)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_placement_group_delete"), categoryCompute) {
		t.Errorf("collection does not contain %v", categoryCompute)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_placement_group_list"), categoryCompute) {
		t.Errorf("collection does not contain %v", categoryCompute)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_placement_group_update"), categoryCompute) {
		t.Errorf("collection does not contain %v", categoryCompute)
	}

	if !slices.Contains(gentools.CategoriesFor("linode_placement_group_unassign"), categoryCompute) {
		t.Errorf("collection does not contain %v", categoryCompute)
	}
}

// Reserved IPs are networking surface, so network-admin serves their writes.
// Go filed them under no category while Python filed them under networking,
// which made network-admin two different profiles depending on the client.
func TestNetworkAdminServesReservedIPWrites(t *testing.T) {
	t.Parallel()

	catalog := []profiles.ToolDescriptor{
		{Name: "linode_networking_reserved_ip_create", Capability: profiles.CapWrite, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_reserved_ip_update", Capability: profiles.CapWrite, Categories: []string{categoryNetworking}},
		{Name: "linode_networking_reserved_ip_delete", Capability: profiles.CapDestroy, Categories: []string{categoryNetworking}},
	}

	allowed := profiles.BuiltinProfiles(catalog)[profiles.BuiltinNetworkAdmin].AllowedTools
	for _, descriptor := range catalog {
		if !slices.Contains(allowed, descriptor.Name) {
			t.Errorf("network-admin does not allow %v", descriptor.Name)
		}
	}
}

// Enabling and canceling backups is the surface storage-admin exists for.
// The singular linode_instance_backup_ prefix missed the plural collection
// route both switches sit on, so only Python's table reached them.
func TestStorageAdminServesInstanceBackupSwitches(t *testing.T) {
	t.Parallel()

	catalog := []profiles.ToolDescriptor{
		{Name: "linode_instance_backups_enable", Capability: profiles.CapWrite, Categories: []string{categoryComputeDeep, categoryCompute}},
		{Name: "linode_instance_backups_cancel", Capability: profiles.CapDestroy, Categories: []string{categoryComputeDeep, categoryCompute}},
	}

	allowed := profiles.BuiltinProfiles(catalog)[profiles.BuiltinStorageAdmin].AllowedTools
	for _, descriptor := range catalog {
		if !slices.Contains(allowed, descriptor.Name) {
			t.Errorf("storage-admin does not allow %v", descriptor.Name)
		}
	}
}

// A mutator in no category reaches only the profiles that elevate every
// category, which is why `make profile-resolution` fails on one: no
// category-scoped admin profile can be given it, however plainly it belongs
// to that admin's surface.
func TestCategoryLessMutatorResolvesOnlyInWildcardProfiles(t *testing.T) {
	t.Parallel()

	homeless := "linode_unmapped_thing_create"
	catalog := []profiles.ToolDescriptor{{Name: homeless, Capability: profiles.CapWrite}}
	built := profiles.BuiltinProfiles(catalog)

	for _, name := range []string{profiles.BuiltinFullAccess, profiles.BuiltinEmergency} {
		if !slices.Contains(built[name].AllowedTools, homeless) {
			t.Errorf("%v does not allow %v", name, homeless)
		}
	}

	for _, name := range []string{
		profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
		profiles.BuiltinComputeAdmin,
		profiles.BuiltinNetworkAdmin,
		profiles.BuiltinKubernetesAdmin,
		profiles.BuiltinStorageAdmin,
		profiles.BuiltinIamAdmin,
	} {
		if slices.Contains(built[name].AllowedTools, homeless) {
			t.Errorf("%v allows %v", name, homeless)
		}
	}
}

// TestBuiltinProfileNamesMatchTheResolvedSet covers what a save reads before it
// refuses a name: the names come off the same blueprints BuiltinProfiles
// resolves, so a built-in added to one cannot be missing from the other.
func TestBuiltinProfileNamesMatchTheResolvedSet(t *testing.T) {
	t.Parallel()

	names := profiles.BuiltinProfileNames()
	resolved := slices.Sorted(maps.Keys(profiles.BuiltinProfiles(nil)))

	if !slices.Equal(names, resolved) {
		t.Errorf("BuiltinProfileNames() = %v, want %v", names, resolved)
	}

	if !slices.Contains(names, profiles.BuiltinDefault) {
		t.Errorf("BuiltinProfileNames() = %v, want it to carry %v", names, profiles.BuiltinDefault)
	}
}

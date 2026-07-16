"""Version information for LinodeMCP."""

import platform
from dataclasses import asdict, dataclass

VERSION = "0.1.0"
API_VERSION = "0.1.0"

REMOVED_FEATURE_TOOLS_LIST = "linode_object_storage_cluster_get"


@dataclass
class VersionInfo:
    """Build and version information."""

    version: str
    api_version: str
    build_date: str
    git_commit: str
    git_branch: str
    python_version: str
    platform: str
    features: dict[str, str]

    def to_dict(self) -> dict[str, str | dict[str, str]]:
        """Convert to dictionary."""
        return asdict(self)

    def __str__(self) -> str:
        """String representation."""
        return (
            f"LinodeMCP v{self.version} "
            f"(MCP: v{self.api_version}, {self.platform}, {self.git_commit})"
        )


FEATURE_TOOLS_LIST = (
    "hello,version,linode_profile_get,linode_profile_preferences_get,linode_profile_preferences_update,linode_profile_phone_number_delete,linode_profile_phone_number_send,linode_profile_phone_number_verify,linode_profile_device_revoke,linode_profile_tfa_disable,linode_profile_tfa_enable,linode_profile_tfa_enable_confirm,linode_account_get,linode_account_update,linode_account_user_create,linode_account_user_delete,linode_account_user_get,linode_account_user_grants_get,linode_account_user_grants_update,linode_account_user_update,linode_account_user_list,linode_account_cancel,linode_account_agreement_acknowledge,linode_account_beta_get,linode_account_child_account_get,linode_account_invoice_get,linode_account_oauth_client_create,linode_account_oauth_client_delete,linode_account_oauth_client_get,linode_account_payment_create,linode_account_payment_get,linode_account_promo_credit_add,linode_account_payment_method_create,linode_account_payment_method_delete,linode_account_payment_method_get,linode_account_service_transfer_accept,linode_account_service_transfer_create,linode_account_service_transfer_delete,linode_account_service_transfer_get,linode_account_service_transfer_list,linode_account_settings_get,linode_account_settings_managed_enable,linode_managed_credential_create,linode_managed_credential_revoke,linode_managed_credential_update,linode_managed_credential_username_password_update,linode_managed_contact_create,linode_managed_service_create,linode_managed_service_delete,linode_managed_service_disable,linode_managed_service_enable,linode_managed_service_get,linode_managed_service_update,linode_managed_contact_update,linode_managed_contact_delete,linode_managed_credential_get,linode_managed_linode_settings_list,linode_managed_linode_settings_get,linode_managed_sshkey_get,linode_managed_service_list,linode_account_settings_update,linode_account_transfer_get,linode_account_oauth_client_thumbnail_get,linode_account_oauth_client_thumbnail_update,linode_account_oauth_client_update,linode_account_beta_enroll,linode_beta_get,linode_beta_list,linode_database_mysql_instance_create,linode_database_engine_get,linode_database_instance_list,linode_database_type_get,linode_database_mysql_config_get,linode_database_postgresql_config_get,linode_database_postgresql_instance_credentials_reset,linode_database_postgresql_instance_create,linode_database_postgresql_instance_credentials_get,linode_database_postgresql_instance_delete,linode_database_postgresql_instance_get,linode_database_postgresql_instance_patch,linode_database_postgresql_instance_resume,linode_database_postgresql_instance_ssl_get,linode_database_postgresql_instance_suspend,linode_database_postgresql_instance_update,linode_database_postgresql_instance_list,linode_database_mysql_instance_credentials_reset,linode_database_mysql_instance_credentials_get,linode_database_mysql_instance_delete,linode_database_mysql_instance_get,linode_database_mysql_instance_patch,linode_database_mysql_instance_resume,linode_database_mysql_instance_ssl_get,linode_database_mysql_instance_suspend,linode_database_mysql_instance_update,linode_database_mysql_instance_list,"
    "linode_instance_list,linode_instance_nodebalancer_list,linode_instance_stats_month_get,linode_instance_transfer_get,linode_instance_transfer_month_get,linode_instance_interface_list,linode_instance_interface_add,linode_instance_interface_update,linode_instance_interface_delete,linode_instance_interface_settings_get,linode_instance_config_list,linode_instance_config_get,linode_instance_config_interface_add,linode_instance_config_interface_delete,linode_instance_config_interface_get,linode_instance_config_interface_update,linode_instance_interface_history_list,linode_instance_config_interface_list,linode_instance_config_interface_reorder,linode_instance_config_update,linode_instance_config_delete,"
    "linode_instance_get,linode_region_list,linode_kernel_list,linode_longview_subscription_list,"
    "linode_region_availability_list,linode_region_availability_get,"
    "linode_type_list,linode_type_get,linode_volume_list,linode_volume_get,linode_volume_type_list,linode_image_create,linode_image_delete,linode_image_get,linode_image_upload,linode_image_replicate,linode_image_sharegroup_create,linode_image_sharegroup_by_image_list,linode_image_list,linode_image_sharegroup_delete,linode_image_sharegroup_get,linode_image_sharegroup_image_delete,linode_image_sharegroup_image_update,linode_image_sharegroup_image_add,linode_image_sharegroup_image_list,linode_image_sharegroup_member_token_get,linode_image_sharegroup_member_token_delete,linode_image_sharegroup_member_token_update,linode_image_sharegroup_member_add,linode_image_sharegroup_member_list,linode_image_sharegroup_update,linode_image_sharegroup_list,linode_image_sharegroup_token_create,linode_image_sharegroup_token_delete,linode_image_sharegroup_token_get,linode_image_sharegroup_by_token_get,linode_image_sharegroup_token_image_list,linode_image_sharegroup_token_update,linode_image_sharegroup_token_list,"
    "linode_sshkey_list,linode_sshkey_get,linode_database_engine_list,linode_database_type_list,linode_domain_list,linode_domain_get,linode_domain_zone_file_get,"
    "linode_domain_record_list,linode_domain_record_get,linode_firewall_list,linode_firewall_get,"
    "linode_firewall_device_list,"
    "linode_nodebalancer_list,linode_nodebalancer_type_list,linode_nodebalancer_get,linode_nodebalancer_vpc_config_get,"
    "linode_nodebalancer_vpc_config_list,linode_nodebalancer_config_list,"
    "linode_stackscript_list,linode_stackscript_get,linode_stackscript_create,linode_stackscript_update,linode_stackscript_delete,linode_sshkey_create,linode_sshkey_update,linode_sshkey_delete,"
    "linode_instance_boot,linode_instance_reboot,linode_instance_shutdown,linode_instance_firewall_apply,linode_instance_interface_settings_update,"
    "linode_instance_create,linode_instance_update,linode_instance_delete,linode_instance_resize,"
    "linode_firewall_create,linode_firewall_update,linode_firewall_delete,"
    "linode_firewall_device_create,linode_firewall_device_delete,"
    "linode_domain_clone,linode_domain_create,linode_domain_update,linode_domain_delete,linode_domain_import,"
    "linode_domain_record_create,linode_domain_record_update,"
    "linode_domain_record_delete,linode_volume_create,linode_volume_attach,"
    "linode_volume_detach,linode_volume_resize,linode_volume_update,linode_volume_delete,"
    "linode_nodebalancer_config_delete,linode_nodebalancer_config_node_create,linode_nodebalancer_config_node_list,linode_nodebalancer_config_node_get,linode_nodebalancer_config_rebuild,linode_nodebalancer_config_update,linode_nodebalancer_config_node_update,linode_nodebalancer_create,linode_nodebalancer_update,"
    "linode_nodebalancer_firewall_update,linode_nodebalancer_delete,"
    "linode_object_storage_bucket_list,linode_object_storage_bucket_by_region_list,linode_object_storage_bucket_get,"
    "linode_object_storage_bucket_object_list,"
    "linode_object_storage_endpoint_list,"
    "linode_object_storage_type_list,linode_object_storage_key_list,"
    "linode_object_storage_key_get,linode_object_storage_transfer_get,"
    "linode_object_storage_quota_list,linode_object_storage_bucket_access_get,"
    "linode_object_storage_bucket_access_allow,"
    "linode_object_storage_cancel,"
    "linode_object_storage_bucket_create,linode_object_storage_bucket_delete,"
    "linode_object_storage_bucket_access_update,"
    "linode_object_storage_key_create,linode_object_storage_key_update,"
    "linode_object_storage_key_delete,linode_object_storage_presigned_url_create,"
    "linode_object_storage_object_acl_get,"
    "linode_object_storage_object_acl_update,"
    "linode_object_storage_ssl_get,linode_object_storage_ssl_delete,"
    "linode_lke_cluster_list,linode_lke_cluster_get,"
    "linode_lke_cluster_create,linode_lke_cluster_update,"
    "linode_lke_cluster_delete,linode_lke_cluster_recycle,"
    "linode_lke_cluster_regenerate,linode_lke_pool_list,"
    "linode_lke_pool_get,linode_lke_pool_create,linode_lke_pool_update,"
    "linode_lke_pool_delete,linode_lke_pool_recycle,"
    "linode_lke_node_get,linode_lke_node_delete,linode_lke_node_recycle,"
    "linode_lke_kubeconfig_get,linode_lke_kubeconfig_delete,"
    "linode_lke_dashboard_get,linode_lke_api_endpoint_list,linode_longview_client_delete,linode_longview_client_get,linode_longview_client_update,linode_longview_client_list,linode_longview_plan_get,linode_longview_plan_update,linode_longview_type_list,linode_monitor_dashboard_get,linode_monitor_alert_channel_list,linode_monitor_alert_definition_list,linode_monitor_dashboard_list,"
    "linode_lke_service_token_delete,linode_lke_acl_get,"
    "linode_lke_acl_update,linode_lke_acl_delete,"
    "linode_lke_version_list,linode_lke_version_get,"
    "linode_lke_type_list,linode_lke_tier_version_list,linode_lke_tier_version_get,"
    "linode_vlan_delete,linode_vlan_list,linode_networking_ip_share,linode_networking_reserved_ip_create,linode_networking_reserved_ip_delete,linode_networking_reserved_ip_get,linode_networking_reserved_ip_list,linode_networking_reserved_ip_type_list,linode_networking_reserved_ip_update,linode_vpc_list,linode_vpc_get,linode_vpc_create,"
    "linode_vpc_update,linode_vpc_delete,linode_vpc_ip_all_list,"
    "linode_vpc_ip_list,linode_vpc_subnet_list,linode_vpc_subnet_get,"
    "linode_vpc_subnet_create,linode_vpc_subnet_update,"
    "linode_vpc_subnet_delete,"
    "linode_instance_backup_list,linode_instance_backup_get,"
    "linode_instance_backup_create,linode_instance_backup_restore,"
    "linode_instance_backups_enable,linode_instance_backups_cancel,"
    "linode_instance_config_create,linode_instance_disk_list,linode_instance_disk_get,"
    "linode_instance_disk_create,linode_instance_disk_update,"
    "linode_instance_disk_delete,linode_instance_disk_clone,"
    "linode_instance_disk_resize,linode_instance_disk_password_reset,"
    "linode_instance_interface_get,linode_instance_ip_list,linode_instance_ip_get,"
    "linode_instance_ip_allocate,linode_instance_ip_update,linode_instance_ip_delete,"
    "linode_instance_clone,linode_instance_migrate,"
    "linode_instance_rebuild,linode_instance_rescue,"
    "linode_instance_password_reset"
)


def get_version_info(
    build_date: str = "unknown",
    git_commit: str = "dev",
    git_branch: str = "main",
) -> VersionInfo:
    """Get version information."""
    return VersionInfo(
        version=VERSION,
        api_version=API_VERSION,
        build_date=build_date,
        git_commit=git_commit,
        git_branch=git_branch,
        python_version=platform.python_version(),
        platform=f"{platform.system()}/{platform.machine()}",
        features={
            "tools": FEATURE_TOOLS_LIST,
            "logging": "basic",
            "protocol": "mcp",
        },
    )

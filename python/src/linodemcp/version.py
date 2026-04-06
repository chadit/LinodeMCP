"""Version information for LinodeMCP."""

import platform
from dataclasses import asdict, dataclass

VERSION = "0.1.0"
API_VERSION = "0.1.0"


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
    "hello,version,linode_profile,linode_account,"
    "linode_instances_list,linode_instance_get,linode_regions_list,"
    "linode_types_list,linode_volumes_list,linode_images_list,"
    "linode_sshkeys_list,linode_domains_list,linode_domain_get,"
    "linode_domain_records_list,linode_firewalls_list,"
    "linode_nodebalancers_list,linode_nodebalancer_get,"
    "linode_stackscripts_list,linode_sshkey_create,linode_sshkey_delete,"
    "linode_instance_boot,linode_instance_reboot,linode_instance_shutdown,"
    "linode_instance_create,linode_instance_delete,linode_instance_resize,"
    "linode_firewall_create,linode_firewall_update,linode_firewall_delete,"
    "linode_domain_create,linode_domain_update,linode_domain_delete,"
    "linode_domain_record_create,linode_domain_record_update,"
    "linode_domain_record_delete,linode_volume_create,linode_volume_attach,"
    "linode_volume_detach,linode_volume_resize,linode_volume_delete,"
    "linode_nodebalancer_create,linode_nodebalancer_update,"
    "linode_nodebalancer_delete,"
    "linode_object_storage_buckets_list,linode_object_storage_bucket_get,"
    "linode_object_storage_bucket_contents,linode_object_storage_clusters_list,"
    "linode_object_storage_type_list,linode_object_storage_keys_list,"
    "linode_object_storage_key_get,linode_object_storage_transfer,"
    "linode_object_storage_bucket_access_get,"
    "linode_object_storage_bucket_create,linode_object_storage_bucket_delete,"
    "linode_object_storage_bucket_access_update,"
    "linode_object_storage_key_create,linode_object_storage_key_update,"
    "linode_object_storage_key_delete,linode_object_storage_presigned_url,"
    "linode_object_storage_object_acl_get,"
    "linode_object_storage_object_acl_update,"
    "linode_object_storage_ssl_get,linode_object_storage_ssl_delete,"
    "linode_lke_clusters_list,linode_lke_cluster_get,"
    "linode_lke_cluster_create,linode_lke_cluster_update,"
    "linode_lke_cluster_delete,linode_lke_cluster_recycle,"
    "linode_lke_cluster_regenerate,linode_lke_pools_list,"
    "linode_lke_pool_get,linode_lke_pool_create,linode_lke_pool_update,"
    "linode_lke_pool_delete,linode_lke_pool_recycle,"
    "linode_lke_node_get,linode_lke_node_delete,linode_lke_node_recycle,"
    "linode_lke_kubeconfig_get,linode_lke_kubeconfig_delete,"
    "linode_lke_dashboard_get,linode_lke_api_endpoints_list,"
    "linode_lke_service_token_delete,linode_lke_acl_get,"
    "linode_lke_acl_update,linode_lke_acl_delete,"
    "linode_lke_versions_list,linode_lke_version_get,"
    "linode_lke_types_list,linode_lke_tier_versions_list,"
    "linode_vpcs_list,linode_vpc_get,linode_vpc_create,"
    "linode_vpc_update,linode_vpc_delete,linode_vpc_ips_list,"
    "linode_vpc_ip_list,linode_vpc_subnets_list,linode_vpc_subnet_get,"
    "linode_vpc_subnet_create,linode_vpc_subnet_update,"
    "linode_vpc_subnet_delete,"
    "linode_instance_backups_list,linode_instance_backup_get,"
    "linode_instance_backup_create,linode_instance_backup_restore,"
    "linode_instance_backups_enable,linode_instance_backups_cancel,"
    "linode_instance_disks_list,linode_instance_disk_get,"
    "linode_instance_disk_create,linode_instance_disk_update,"
    "linode_instance_disk_delete,linode_instance_disk_clone,"
    "linode_instance_disk_resize,"
    "linode_instance_ips_list,linode_instance_ip_get,"
    "linode_instance_ip_allocate,linode_instance_ip_delete,"
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

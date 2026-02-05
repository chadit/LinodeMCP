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
    "hello,version,linode_profile,linode_account,linode_instances_list,"
    "linode_instance_get,linode_regions_list,linode_types_list,linode_volumes_list,"
    "linode_images_list,linode_sshkeys_list,linode_domains_list,linode_domain_get,"
    "linode_domain_records_list,linode_firewalls_list,linode_nodebalancers_list,"
    "linode_nodebalancer_get,linode_stackscripts_list,linode_sshkey_create,"
    "linode_sshkey_delete,linode_instance_boot,linode_instance_reboot,"
    "linode_instance_shutdown,linode_instance_create,linode_instance_delete,"
    "linode_instance_resize,linode_firewall_create,linode_firewall_update,"
    "linode_firewall_delete,linode_domain_create,linode_domain_update,"
    "linode_domain_delete,linode_domain_record_create,linode_domain_record_update,"
    "linode_domain_record_delete,linode_volume_create,linode_volume_attach,"
    "linode_volume_detach,linode_volume_resize,linode_volume_delete,"
    "linode_nodebalancer_create,linode_nodebalancer_update,linode_nodebalancer_delete"
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

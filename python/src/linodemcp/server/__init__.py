"""MCP server implementation for LinodeMCP."""

import logging
from typing import Any

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server

from linodemcp.config import Config
from linodemcp.tools import (
    create_hello_tool,
    create_linode_account_tool,
    create_linode_domain_create_tool,
    create_linode_domain_delete_tool,
    create_linode_domain_get_tool,
    create_linode_domain_record_create_tool,
    create_linode_domain_record_delete_tool,
    create_linode_domain_record_update_tool,
    create_linode_domain_records_list_tool,
    create_linode_domain_update_tool,
    create_linode_domains_list_tool,
    create_linode_firewall_create_tool,
    create_linode_firewall_delete_tool,
    create_linode_firewall_update_tool,
    create_linode_firewalls_list_tool,
    create_linode_images_list_tool,
    create_linode_instance_boot_tool,
    create_linode_instance_create_tool,
    create_linode_instance_delete_tool,
    create_linode_instance_get_tool,
    create_linode_instance_reboot_tool,
    create_linode_instance_resize_tool,
    create_linode_instance_shutdown_tool,
    create_linode_instances_list_tool,
    create_linode_nodebalancer_create_tool,
    create_linode_nodebalancer_delete_tool,
    create_linode_nodebalancer_get_tool,
    create_linode_nodebalancer_update_tool,
    create_linode_nodebalancers_list_tool,
    create_linode_object_storage_bucket_access_get_tool,
    create_linode_object_storage_bucket_access_update_tool,
    create_linode_object_storage_bucket_contents_tool,
    create_linode_object_storage_bucket_create_tool,
    create_linode_object_storage_bucket_delete_tool,
    create_linode_object_storage_bucket_get_tool,
    create_linode_object_storage_buckets_list_tool,
    create_linode_object_storage_clusters_list_tool,
    create_linode_object_storage_key_create_tool,
    create_linode_object_storage_key_delete_tool,
    create_linode_object_storage_key_get_tool,
    create_linode_object_storage_key_update_tool,
    create_linode_object_storage_keys_list_tool,
    create_linode_object_storage_object_acl_get_tool,
    create_linode_object_storage_object_acl_update_tool,
    create_linode_object_storage_presigned_url_tool,
    create_linode_object_storage_ssl_delete_tool,
    create_linode_object_storage_ssl_get_tool,
    create_linode_object_storage_transfer_tool,
    create_linode_object_storage_types_list_tool,
    create_linode_profile_tool,
    create_linode_regions_list_tool,
    create_linode_sshkey_create_tool,
    create_linode_sshkey_delete_tool,
    create_linode_sshkeys_list_tool,
    create_linode_stackscripts_list_tool,
    create_linode_types_list_tool,
    create_linode_volume_attach_tool,
    create_linode_volume_create_tool,
    create_linode_volume_delete_tool,
    create_linode_volume_detach_tool,
    create_linode_volume_resize_tool,
    create_linode_volumes_list_tool,
    create_version_tool,
    handle_hello,
    handle_linode_account,
    handle_linode_domain_create,
    handle_linode_domain_delete,
    handle_linode_domain_get,
    handle_linode_domain_record_create,
    handle_linode_domain_record_delete,
    handle_linode_domain_record_update,
    handle_linode_domain_records_list,
    handle_linode_domain_update,
    handle_linode_domains_list,
    handle_linode_firewall_create,
    handle_linode_firewall_delete,
    handle_linode_firewall_update,
    handle_linode_firewalls_list,
    handle_linode_images_list,
    handle_linode_instance_boot,
    handle_linode_instance_create,
    handle_linode_instance_delete,
    handle_linode_instance_get,
    handle_linode_instance_reboot,
    handle_linode_instance_resize,
    handle_linode_instance_shutdown,
    handle_linode_instances_list,
    handle_linode_nodebalancer_create,
    handle_linode_nodebalancer_delete,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancer_update,
    handle_linode_nodebalancers_list,
    handle_linode_object_storage_bucket_access_get,
    handle_linode_object_storage_bucket_access_update,
    handle_linode_object_storage_bucket_contents,
    handle_linode_object_storage_bucket_create,
    handle_linode_object_storage_bucket_delete,
    handle_linode_object_storage_bucket_get,
    handle_linode_object_storage_buckets_list,
    handle_linode_object_storage_clusters_list,
    handle_linode_object_storage_key_create,
    handle_linode_object_storage_key_delete,
    handle_linode_object_storage_key_get,
    handle_linode_object_storage_key_update,
    handle_linode_object_storage_keys_list,
    handle_linode_object_storage_object_acl_get,
    handle_linode_object_storage_object_acl_update,
    handle_linode_object_storage_presigned_url,
    handle_linode_object_storage_ssl_delete,
    handle_linode_object_storage_ssl_get,
    handle_linode_object_storage_transfer,
    handle_linode_object_storage_types_list,
    handle_linode_profile,
    handle_linode_regions_list,
    handle_linode_sshkey_create,
    handle_linode_sshkey_delete,
    handle_linode_sshkeys_list,
    handle_linode_stackscripts_list,
    handle_linode_types_list,
    handle_linode_volume_attach,
    handle_linode_volume_create,
    handle_linode_volume_delete,
    handle_linode_volume_detach,
    handle_linode_volume_resize,
    handle_linode_volumes_list,
    handle_version,
)

__all__ = ["Server"]

logger = logging.getLogger(__name__)


class Server:
    """LinodeMCP server."""

    def __init__(self, config: Config) -> None:
        if not config:
            msg = "config cannot be None"
            raise ValueError(msg)

        self.config = config
        self.mcp = MCPServer(config.server.name)
        self._register_tools()

    def _register_tools(self) -> None:
        """Register all MCP tools."""
        self.mcp.list_tools()(  # type: ignore[no-untyped-call]
            lambda: [
                create_hello_tool(),
                create_version_tool(),
                create_linode_profile_tool(),
                create_linode_account_tool(),
                create_linode_instances_list_tool(),
                create_linode_instance_get_tool(),
                create_linode_regions_list_tool(),
                create_linode_types_list_tool(),
                create_linode_volumes_list_tool(),
                create_linode_images_list_tool(),
                # Stage 3: Extended read operations
                create_linode_sshkeys_list_tool(),
                create_linode_domains_list_tool(),
                create_linode_domain_get_tool(),
                create_linode_domain_records_list_tool(),
                create_linode_firewalls_list_tool(),
                create_linode_nodebalancers_list_tool(),
                create_linode_nodebalancer_get_tool(),
                create_linode_stackscripts_list_tool(),
                # Phase 1: Object Storage read operations
                create_linode_object_storage_buckets_list_tool(),
                create_linode_object_storage_bucket_get_tool(),
                create_linode_object_storage_bucket_contents_tool(),
                create_linode_object_storage_clusters_list_tool(),
                create_linode_object_storage_types_list_tool(),
                # Phase 2: Object Storage access key & transfer read operations
                create_linode_object_storage_keys_list_tool(),
                create_linode_object_storage_key_get_tool(),
                create_linode_object_storage_transfer_tool(),
                create_linode_object_storage_bucket_access_get_tool(),
                # Phase 3: Object Storage write operations
                create_linode_object_storage_bucket_create_tool(),
                create_linode_object_storage_bucket_delete_tool(),
                create_linode_object_storage_bucket_access_update_tool(),
                # Phase 4: Object Storage access key write operations
                create_linode_object_storage_key_create_tool(),
                create_linode_object_storage_key_update_tool(),
                create_linode_object_storage_key_delete_tool(),
                # Phase 5: Presigned URLs, Object ACL, and SSL
                create_linode_object_storage_presigned_url_tool(),
                create_linode_object_storage_object_acl_get_tool(),
                create_linode_object_storage_object_acl_update_tool(),
                create_linode_object_storage_ssl_get_tool(),
                create_linode_object_storage_ssl_delete_tool(),
                # Stage 4: Write operations
                create_linode_sshkey_create_tool(),
                create_linode_sshkey_delete_tool(),
                create_linode_instance_boot_tool(),
                create_linode_instance_reboot_tool(),
                create_linode_instance_shutdown_tool(),
                create_linode_instance_create_tool(),
                create_linode_instance_delete_tool(),
                create_linode_instance_resize_tool(),
                create_linode_firewall_create_tool(),
                create_linode_firewall_update_tool(),
                create_linode_firewall_delete_tool(),
                create_linode_domain_create_tool(),
                create_linode_domain_update_tool(),
                create_linode_domain_delete_tool(),
                create_linode_domain_record_create_tool(),
                create_linode_domain_record_update_tool(),
                create_linode_domain_record_delete_tool(),
                create_linode_volume_create_tool(),
                create_linode_volume_attach_tool(),
                create_linode_volume_detach_tool(),
                create_linode_volume_resize_tool(),
                create_linode_volume_delete_tool(),
                create_linode_nodebalancer_create_tool(),
                create_linode_nodebalancer_update_tool(),
                create_linode_nodebalancer_delete_tool(),
            ]
        )

        # Tool handlers requiring config
        config_handlers = {
            "linode_profile": handle_linode_profile,
            "linode_account": handle_linode_account,
            "linode_instances_list": handle_linode_instances_list,
            "linode_instance_get": handle_linode_instance_get,
            "linode_regions_list": handle_linode_regions_list,
            "linode_types_list": handle_linode_types_list,
            "linode_volumes_list": handle_linode_volumes_list,
            "linode_images_list": handle_linode_images_list,
            # Stage 3: Extended read operations
            "linode_sshkeys_list": handle_linode_sshkeys_list,
            "linode_domains_list": handle_linode_domains_list,
            "linode_domain_get": handle_linode_domain_get,
            "linode_domain_records_list": handle_linode_domain_records_list,
            "linode_firewalls_list": handle_linode_firewalls_list,
            "linode_nodebalancers_list": handle_linode_nodebalancers_list,
            "linode_nodebalancer_get": handle_linode_nodebalancer_get,
            "linode_stackscripts_list": handle_linode_stackscripts_list,
            # Phase 1: Object Storage read operations
            "linode_object_storage_buckets_list": (
                handle_linode_object_storage_buckets_list
            ),
            "linode_object_storage_bucket_get": (
                handle_linode_object_storage_bucket_get
            ),
            "linode_object_storage_bucket_contents": (
                handle_linode_object_storage_bucket_contents
            ),
            "linode_object_storage_clusters_list": (
                handle_linode_object_storage_clusters_list
            ),
            "linode_object_storage_types_list": (
                handle_linode_object_storage_types_list
            ),
            # Phase 2: Object Storage access key & transfer read operations
            "linode_object_storage_keys_list": (handle_linode_object_storage_keys_list),
            "linode_object_storage_key_get": (handle_linode_object_storage_key_get),
            "linode_object_storage_transfer": (handle_linode_object_storage_transfer),
            "linode_object_storage_bucket_access_get": (
                handle_linode_object_storage_bucket_access_get
            ),
            # Phase 3: Object Storage write operations
            "linode_object_storage_bucket_create": (
                handle_linode_object_storage_bucket_create
            ),
            "linode_object_storage_bucket_delete": (
                handle_linode_object_storage_bucket_delete
            ),
            "linode_object_storage_bucket_access_update": (
                handle_linode_object_storage_bucket_access_update
            ),
            # Phase 4: Object Storage access key write operations
            "linode_object_storage_key_create": (
                handle_linode_object_storage_key_create
            ),
            "linode_object_storage_key_update": (
                handle_linode_object_storage_key_update
            ),
            "linode_object_storage_key_delete": (
                handle_linode_object_storage_key_delete
            ),
            # Phase 5: Presigned URLs, Object ACL, and SSL
            "linode_object_storage_presigned_url": (
                handle_linode_object_storage_presigned_url
            ),
            "linode_object_storage_object_acl_get": (
                handle_linode_object_storage_object_acl_get
            ),
            "linode_object_storage_object_acl_update": (
                handle_linode_object_storage_object_acl_update
            ),
            "linode_object_storage_ssl_get": (handle_linode_object_storage_ssl_get),
            "linode_object_storage_ssl_delete": (
                handle_linode_object_storage_ssl_delete
            ),
            # Stage 4: Write operations
            "linode_sshkey_create": handle_linode_sshkey_create,
            "linode_sshkey_delete": handle_linode_sshkey_delete,
            "linode_instance_boot": handle_linode_instance_boot,
            "linode_instance_reboot": handle_linode_instance_reboot,
            "linode_instance_shutdown": handle_linode_instance_shutdown,
            "linode_instance_create": handle_linode_instance_create,
            "linode_instance_delete": handle_linode_instance_delete,
            "linode_instance_resize": handle_linode_instance_resize,
            "linode_firewall_create": handle_linode_firewall_create,
            "linode_firewall_update": handle_linode_firewall_update,
            "linode_firewall_delete": handle_linode_firewall_delete,
            "linode_domain_create": handle_linode_domain_create,
            "linode_domain_update": handle_linode_domain_update,
            "linode_domain_delete": handle_linode_domain_delete,
            "linode_domain_record_create": handle_linode_domain_record_create,
            "linode_domain_record_update": handle_linode_domain_record_update,
            "linode_domain_record_delete": handle_linode_domain_record_delete,
            "linode_volume_create": handle_linode_volume_create,
            "linode_volume_attach": handle_linode_volume_attach,
            "linode_volume_detach": handle_linode_volume_detach,
            "linode_volume_resize": handle_linode_volume_resize,
            "linode_volume_delete": handle_linode_volume_delete,
            "linode_nodebalancer_create": handle_linode_nodebalancer_create,
            "linode_nodebalancer_update": handle_linode_nodebalancer_update,
            "linode_nodebalancer_delete": handle_linode_nodebalancer_delete,
        }

        @self.mcp.call_tool()  # type: ignore[untyped-decorator]
        async def call_tool_handler(name: str, arguments: dict[str, Any]) -> list[Any]:
            """Handle tool calls."""
            match name:
                case "hello":
                    return await handle_hello(arguments)
                case "version":
                    return await handle_version(arguments)
                case _ if name in config_handlers:
                    return await config_handlers[name](arguments, self.config)
                case _:
                    msg = f"Unknown tool: {name}"
                    raise ValueError(msg)

    async def start(self) -> None:
        """Start the MCP server using stdio transport."""
        logger.info("Starting LinodeMCP server")
        logger.info(
            "Registered tools: hello, version, linode_profile, linode_account, "
            "linode_instances_list, linode_instance_get, linode_regions_list, "
            "linode_types_list, linode_volumes_list, linode_images_list, "
            "linode_sshkeys_list, linode_domains_list, linode_domain_get, "
            "linode_domain_records_list, linode_firewalls_list, "
            "linode_nodebalancers_list, linode_nodebalancer_get, "
            "linode_stackscripts_list"
        )

        async with stdio_server() as (read_stream, write_stream):
            await self.mcp.run(
                read_stream,
                write_stream,
                self.mcp.create_initialization_options(),
            )

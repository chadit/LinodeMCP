"""MCP server implementation for LinodeMCP."""

import logging
from typing import Any

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server

from linodemcp.config import Config
from linodemcp.tools import (
    create_hello_tool,
    create_linode_account_tool,
    create_linode_domain_get_tool,
    create_linode_domain_records_list_tool,
    create_linode_domains_list_tool,
    create_linode_firewalls_list_tool,
    create_linode_images_list_tool,
    create_linode_instance_get_tool,
    create_linode_instances_list_tool,
    create_linode_nodebalancer_get_tool,
    create_linode_nodebalancers_list_tool,
    create_linode_profile_tool,
    create_linode_regions_list_tool,
    create_linode_sshkeys_list_tool,
    create_linode_stackscripts_list_tool,
    create_linode_types_list_tool,
    create_linode_volumes_list_tool,
    create_version_tool,
    handle_hello,
    handle_linode_account,
    handle_linode_domain_get,
    handle_linode_domain_records_list,
    handle_linode_domains_list,
    handle_linode_firewalls_list,
    handle_linode_images_list,
    handle_linode_instance_get,
    handle_linode_instances_list,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancers_list,
    handle_linode_profile,
    handle_linode_regions_list,
    handle_linode_sshkeys_list,
    handle_linode_stackscripts_list,
    handle_linode_types_list,
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
        }

        @self.mcp.call_tool()  # type: ignore[untyped-decorator]
        async def call_tool_handler(name: str, arguments: dict[str, Any]) -> list[Any]:
            """Handle tool calls."""
            if name == "hello":
                return await handle_hello(arguments)
            if name == "version":
                return await handle_version(arguments)
            if name in config_handlers:
                return await config_handlers[name](arguments, self.config)

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

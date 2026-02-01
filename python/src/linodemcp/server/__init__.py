"""MCP server implementation for LinodeMCP."""

import logging
from typing import Any

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server

from linodemcp.config import Config
from linodemcp.tools import (
    create_hello_tool,
    create_linode_instances_list_tool,
    create_linode_profile_tool,
    create_version_tool,
    handle_hello,
    handle_linode_instances_list,
    handle_linode_profile,
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
                create_linode_instances_list_tool(),
            ]
        )

        @self.mcp.call_tool()  # type: ignore[untyped-decorator]
        async def call_tool_handler(name: str, arguments: dict[str, Any]) -> list[Any]:
            """Handle tool calls."""
            if name == "hello":
                return await handle_hello(arguments)
            if name == "version":
                return await handle_version(arguments)
            if name == "linode_profile":
                return await handle_linode_profile(arguments, self.config)
            if name == "linode_instances_list":
                return await handle_linode_instances_list(arguments, self.config)

            msg = f"Unknown tool: {name}"
            raise ValueError(msg)

    async def start(self) -> None:
        """Start the MCP server using stdio transport."""
        logger.info("Starting LinodeMCP server")
        logger.info(
            "Registered tools: hello, version, linode_profile, linode_instances_list"
        )

        async with stdio_server() as (read_stream, write_stream):
            await self.mcp.run(
                read_stream,
                write_stream,
                self.mcp.create_initialization_options(),
            )

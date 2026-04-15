"""MCP server implementation for LinodeMCP."""

from __future__ import annotations

import logging
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any, cast

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server
from mcp.types import Tool

import linodemcp.tools as tools_module
from linodemcp.tools import (
    handle_hello,
    handle_version,
)

if TYPE_CHECKING:
    from linodemcp.config import Config

__all__ = ["Server"]

logger = logging.getLogger(__name__)

# The MCP library's list_tools() and call_tool() methods lack return type
# annotations. These type aliases let us cast them to their actual signatures
# (verified from the library source) instead of suppressing type errors.
ListToolsDecorator = Callable[
    [Callable[[], Awaitable[list[Tool]]]],
    Callable[[], Awaitable[list[Tool]]],
]
CallToolDecorator = Callable[
    [Callable[..., Awaitable[list[Any]]]],
    Callable[..., Awaitable[list[Any]]],
]


@dataclass(frozen=True)
class ToolEntry:
    """A registered tool with its create and handle functions."""

    name: str
    create_fn: Callable[[], Tool]
    handle_fn: Callable[..., Awaitable[list[Any]]]


def _build_tool_registry() -> list[ToolEntry]:
    """Discover all tools from the linodemcp.tools module.

    Scans ``linodemcp.tools.__all__`` for names matching
    ``create_*_tool`` / ``handle_*`` patterns, pairing them by
    stripping the prefix/suffix to derive the tool name.
    """
    all_names = getattr(tools_module, "__all__", [])

    create_fns: dict[str, Callable[[], Tool]] = {}
    handle_fns: dict[str, Callable[..., Awaitable[list[Any]]]] = {}

    for name in all_names:
        if name.startswith("create_") and name.endswith("_tool"):
            # create_linode_instances_list_tool -> linode_instances_list
            tool_name = name[len("create_") : -len("_tool")]
            fn = getattr(tools_module, name, None)
            if fn is not None:
                create_fns[tool_name] = fn
        elif name.startswith("handle_"):
            # handle_linode_instances_list -> linode_instances_list
            tool_name = name[len("handle_") :]
            fn = getattr(tools_module, name, None)
            if fn is not None:
                handle_fns[tool_name] = fn

    entries = []
    for tool_name in sorted(create_fns.keys()):
        create_fn = create_fns[tool_name]
        handle_fn = handle_fns.get(tool_name)
        if handle_fn is None:
            logger.warning("No handler found for tool: %s", tool_name)
            continue
        entries.append(
            ToolEntry(name=tool_name, create_fn=create_fn, handle_fn=handle_fn)
        )

    return entries


_TOOL_REGISTRY = _build_tool_registry()


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
        """Register all MCP tools using auto-discovery."""
        _list_tools_method = cast(
            "Callable[[], ListToolsDecorator]", self.mcp.list_tools
        )

        async def _list_tools() -> list[Tool]:
            return [entry.create_fn() for entry in _TOOL_REGISTRY]

        _list_tools_method()(_list_tools)

        # Build handler map for config-requiring tools
        config_handlers = {entry.name: entry.handle_fn for entry in _TOOL_REGISTRY}

        async def _call_tool(name: str, arguments: dict[str, Any]) -> list[Any]:
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

        cast("CallToolDecorator", self.mcp.call_tool())(_call_tool)

    async def start(self) -> None:
        """Start the MCP server using stdio transport."""
        logger.info("Starting LinodeMCP server with %d tools", len(_TOOL_REGISTRY))

        async with stdio_server() as (read_stream, write_stream):
            await self.mcp.run(
                read_stream,
                write_stream,
                self.mcp.create_initialization_options(),
            )

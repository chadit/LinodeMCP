"""MCP tools for Linode network transfer pricing."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_network_transfer_price_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_network_transfer_price_list tool."""
    return Tool(
        name="linode_network_transfer_price_list",
        description="Gets network transfer prices.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_network_transfer_price_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_network_transfer_price_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_network_transfer_prices()

    return await execute_tool(cfg, arguments, "retrieve network transfer prices", _call)

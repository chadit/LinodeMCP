"""Networking READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}


def create_linode_vlans_list_tool() -> Tool:
    """Create the linode_vlans_list tool."""
    return Tool(
        name="linode_vlans_list",
        description="Lists all VLANs on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_vlans_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vlans_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vlans = await client.list_vlans()
        return {"count": len(vlans), "vlans": vlans}

    return await execute_tool(cfg, arguments, "list VLANs", _call)

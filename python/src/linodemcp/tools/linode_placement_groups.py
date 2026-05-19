"""Placement group READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from mcp.types import TextContent

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}
_GROUP_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The placement group ID.",
}


def _parse_positive_int(value: Any, name: str) -> int | list[TextContent]:
    """Parse a positive integer argument, rejecting bools and path strings."""
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return error_response(f"{name} must be a positive integer")
    return value


def create_linode_placement_group_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_get tool."""
    return Tool(
        name="linode_placement_group_get",
        description="Gets a placement group",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "group_id": _GROUP_ID_PROP,
            },
            "required": ["group_id"],
        },
    ), Capability.Read


async def handle_linode_placement_group_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_get tool request."""
    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_placement_group(group_id)

    return await execute_tool(cfg, arguments, "get placement group", _call)

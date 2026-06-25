"""Linode instance get tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.linode import instance_to_response_dict
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_instance_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_get tool."""
    return Tool(
        name="linode_instance_get",
        description="Retrieves details of a single Linode instance by its ID",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "string",
                    "description": (
                        "The ID of the Linode instance to retrieve (required)"
                    ),
                },
            },
            "required": ["instance_id"],
        },
    ), Capability.Read


async def handle_linode_instance_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_get tool request.

    Args:
        arguments: InstanceIDArgs - instance_id, environment (optional)
        cfg: Configuration object
    """
    instance_id_str = arguments.get("instance_id", "")

    if not instance_id_str:
        return error_response("instance_id is required")

    try:
        instance_id = int(instance_id_str)
    except ValueError:
        return error_response("instance_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.get_instance(instance_id)
        return instance_to_response_dict(instance)

    return await execute_tool(cfg, arguments, "retrieve Linode instance", _call)

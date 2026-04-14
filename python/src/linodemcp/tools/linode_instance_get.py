"""Linode instance get tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_instance_get_tool() -> Tool:
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
    )


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
        return _error_response("instance_id is required")

    try:
        instance_id = int(instance_id_str)
    except ValueError:
        return _error_response("instance_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.get_instance(instance_id)
        return {
            "id": instance.id,
            "label": instance.label,
            "status": instance.status,
            "type": instance.type,
            "region": instance.region,
            "image": instance.image,
            "ipv4": instance.ipv4,
            "ipv6": instance.ipv6,
            "created": instance.created,
            "updated": instance.updated,
            "tags": instance.tags,
        }

    return await execute_tool(cfg, arguments, "retrieve Linode instance", _call)

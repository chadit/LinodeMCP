"""Linode instances list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_instances_list_tool() -> Tool:
    """Create the linode_instances_list tool."""
    return Tool(
        name="linode_instances_list",
        description="Lists Linode instances with optional filtering by status",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "status": {
                    "type": "string",
                    "description": (
                        "Filter instances by status (running, stopped, etc.)"
                    ),
                },
            },
        },
    )


async def handle_linode_instances_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instances_list tool request.

    Args:
        arguments: InstanceFilterArgs - environment, status (optional)
        cfg: Configuration object
    """
    status_filter = arguments.get("status", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instances = await client.list_instances()

        if status_filter:
            instances = [
                inst
                for inst in instances
                if inst.status.lower() == status_filter.lower()
            ]

        instances_data = [
            {
                "id": inst.id,
                "label": inst.label,
                "status": inst.status,
                "type": inst.type,
                "region": inst.region,
                "image": inst.image,
                "ipv4": inst.ipv4,
                "ipv6": inst.ipv6,
                "created": inst.created,
                "updated": inst.updated,
                "tags": inst.tags,
            }
            for inst in instances
        ]

        response: dict[str, Any] = {
            "count": len(instances),
            "instances": instances_data,
        }

        if status_filter:
            response["filter"] = f"status={status_filter}"

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode instances", _call)

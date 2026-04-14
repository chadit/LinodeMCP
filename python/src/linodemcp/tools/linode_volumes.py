from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_volumes_list_tool() -> Tool:
    """Create the linode_volumes_list tool."""
    return Tool(
        name="linode_volumes_list",
        description=(
            "Lists all block storage volumes for the authenticated user "
            "with optional filtering by region or label"
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": "Filter volumes by region (e.g., 'us-east')",
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter volumes where label contains this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_volumes_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volumes_list tool request."""
    region_filter: str = arguments.get("region", "")
    label_contains: str = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volumes = await client.list_volumes()

        if region_filter:
            volumes = [v for v in volumes if v.region.lower() == region_filter.lower()]

        if label_contains:
            volumes = [v for v in volumes if label_contains.lower() in v.label.lower()]

        volumes_data = [
            {
                "id": v.id,
                "label": v.label,
                "status": v.status,
                "size": v.size,
                "region": v.region,
                "linode_id": v.linode_id,
                "created": v.created,
                "updated": v.updated,
            }
            for v in volumes
        ]

        response: dict[str, Any] = {
            "count": len(volumes),
            "volumes": volumes_data,
        }

        filters: list[str] = []
        if region_filter:
            filters.append(f"region={region_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode volumes", _call)

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _volume_to_dict(volume: Any) -> dict[str, Any]:
    return {
        "id": volume.id,
        "label": volume.label,
        "status": volume.status,
        "size": volume.size,
        "region": volume.region,
        "linode_id": volume.linode_id,
        "linode_label": volume.linode_label,
        "filesystem_path": volume.filesystem_path,
        "tags": volume.tags,
        "created": volume.created,
        "updated": volume.updated,
        "hardware_type": volume.hardware_type,
    }


def create_linode_volume_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_get tool."""
    return Tool(
        name="linode_volume_get",
        description="Gets details for a single block storage volume by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to retrieve (required)",
                },
            },
            "required": ["volume_id"],
        },
    ), Capability.Read


async def handle_linode_volume_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_get tool request."""
    volume_id = arguments.get("volume_id", 0)
    if not volume_id:
        return [TextContent(type="text", text="Error: volume_id is required")]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.get_volume(int(volume_id))
        return {"volume": _volume_to_dict(volume)}

    return await execute_tool(cfg, arguments, "retrieve Linode volume", _call)


def create_linode_volumes_list_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


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

        volumes_data = [_volume_to_dict(v) for v in volumes]

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

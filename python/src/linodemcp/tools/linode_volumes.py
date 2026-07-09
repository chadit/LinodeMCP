from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import type_pb2, volume_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_volume_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_get tool."""
    return Tool(
        name="linode_volume_get",
        description="Gets details for a single block storage volume by ID.",
        inputSchema=schema("linode.mcp.v1.VolumeGetInput"),
    ), Capability.Read


async def handle_linode_volume_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_get tool request."""
    volume_id = arguments.get("volume_id", 0)
    if not volume_id:
        return [TextContent(type="text", text="Error: volume_id is required")]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw(f"/volumes/{int(volume_id)}")
        return serialize_api_response({"volume": raw}, volume_pb2.VolumeGetResponse())

    return await execute_tool(cfg, arguments, "retrieve Linode volume", _call)


def create_linode_volume_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_list tool."""
    return Tool(
        name="linode_volume_list",
        description=(
            "Lists all block storage volumes for the authenticated user "
            "with optional filtering by region or label"
        ),
        inputSchema=schema("linode.mcp.v1.VolumeListInput"),
    ), Capability.Read


def create_linode_volume_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_type_list tool."""
    return Tool(
        name="linode_volume_type_list",
        description="Lists available block storage volume types and prices.",
        inputSchema=schema("linode.mcp.v1.VolumeTypeListInput"),
    ), Capability.Read


async def handle_linode_volume_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume_types = await client.list_volume_types()
        return serialize_list_response(
            {"data": volume_types},
            "volume_types",
            type_pb2.VolumeTypeListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode volume types", _call)


async def handle_linode_volume_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_list tool request."""
    region_filter: str = arguments.get("region", "")
    label_contains: str = arguments.get("label_contains", "")

    def _matches(volume: dict[str, Any]) -> bool:
        region = str(volume.get("region", ""))
        if region_filter and region.lower() != region_filter.lower():
            return False
        label = str(volume.get("label", ""))
        return not (label_contains and label_contains.lower() not in label.lower())

    filters: list[str] = []
    if region_filter:
        filters.append(f"region={region_filter}")
    if label_contains:
        filters.append(f"label_contains={label_contains}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/volumes")
        return serialize_list_response(
            raw,
            "volumes",
            volume_pb2.VolumeListResponse(),
            filter_value=", ".join(filters) if filters else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve Linode volumes", _call)

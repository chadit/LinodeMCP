"""Linode regions list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from urllib.parse import quote

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import region_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def _is_region_id(value: str) -> bool:
    """Return True when value looks like a Linode region ID slug."""
    return bool(value) and all(c.isalnum() or c == "-" for c in value)


def create_linode_region_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_region_list tool."""
    return Tool(
        name="linode_region_list",
        description=(
            "Lists all available Linode regions (datacenters) "
            "with optional filtering by country or capabilities"
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
                "country": {
                    "type": "string",
                    "description": "Filter regions by country code (e.g., 'us', 'de')",
                },
                "capability": {
                    "type": "string",
                    "description": (
                        "Filter regions by capability "
                        "(e.g., 'Linodes', 'Block Storage')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_region_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_region_list tool request.

    Country is a case-insensitive exact match; capability is a case-insensitive
    exact match against the region's capabilities list, mirroring the Go list
    tool's fieldFilter/filterRegionsByCapability.
    """
    country_filter: str = arguments.get("country", "")
    capability_filter: str = arguments.get("capability", "")

    def _matches(region: dict[str, Any]) -> bool:
        country = str(region.get("country", ""))
        if country_filter and country.lower() != country_filter.lower():
            return False
        capabilities = region.get("capabilities", [])
        return not (
            capability_filter
            and not any(
                str(cap).lower() == capability_filter.lower() for cap in capabilities
            )
        )

    applied: list[str] = []
    if country_filter:
        applied.append(f"country={country_filter}")
    if capability_filter:
        applied.append(f"capability={capability_filter}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/regions")
        return serialize_list_response(
            raw,
            "regions",
            region_pb2.RegionListResponse(),
            filter_value=", ".join(applied) if applied else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve Linode regions", _call)


def create_linode_region_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_region_get tool."""
    return Tool(
        name="linode_region_get",
        description="Gets details for a specific Linode region",
        inputSchema=schema("linode.mcp.v1.RegionGetInput"),
    ), Capability.Read


async def handle_linode_region_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_region_get tool request."""
    region_id = str(arguments.get("region_id", "")).strip()
    if not region_id:
        return error_response("region_id is required")
    if not _is_region_id(region_id):
        return error_response(
            "region_id must contain only letters, numbers, and hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw(f"/regions/{quote(region_id, safe='')}")
        return serialize_api_response(raw, region_pb2.Region())

    return await execute_tool(cfg, arguments, f"retrieve region {region_id}", _call)


def create_linode_region_availability_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_region_availability_list tool."""
    return Tool(
        name="linode_region_availability_list",
        description="Lists compute instance type availability across Linode regions",
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


async def handle_linode_region_availability_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_region_availability_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        availability = await client.list_regions_availability()
        return serialize_list_response(
            {"data": availability},
            "region_availabilities",
            region_pb2.RegionAvailabilityListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve regions availability", _call)


def create_linode_region_availability_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_region_availability_get tool."""
    return Tool(
        name="linode_region_availability_get",
        description=(
            "Gets compute instance type availability for a specific Linode region"
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
                "region_id": {
                    "type": "string",
                    "description": "Region ID to check (for example, 'us-east')",
                },
            },
            "required": ["region_id"],
        },
    ), Capability.Read


async def handle_linode_region_availability_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_region_availability_get tool request."""
    region_id = str(arguments.get("region_id", "")).strip()
    if not region_id:
        return error_response("region_id is required")
    if not _is_region_id(region_id):
        return error_response(
            "region_id must contain only letters, numbers, and hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        availability = await client.get_region_availability(region_id)
        return {
            "region_id": region_id,
            "count": len(availability),
            "availability": availability,
        }

    return await execute_tool(
        cfg, arguments, f"retrieve availability for region {region_id}", _call
    )

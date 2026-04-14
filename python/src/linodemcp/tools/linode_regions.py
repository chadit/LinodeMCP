"""Linode regions list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_regions_list_tool() -> Tool:
    """Create the linode_regions_list tool."""
    return Tool(
        name="linode_regions_list",
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
    )


async def handle_linode_regions_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_regions_list tool request."""
    country_filter: str = arguments.get("country", "")
    capability_filter: str = arguments.get("capability", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        regions = await client.list_regions()

        if country_filter:
            regions = [
                r for r in regions if r.country.lower() == country_filter.lower()
            ]

        if capability_filter:
            regions = [
                r
                for r in regions
                if any(
                    cap.lower() == capability_filter.lower() for cap in r.capabilities
                )
            ]

        regions_data = [
            {
                "id": r.id,
                "label": r.label,
                "country": r.country,
                "capabilities": r.capabilities,
                "status": r.status,
            }
            for r in regions
        ]

        response: dict[str, Any] = {
            "count": len(regions),
            "regions": regions_data,
        }

        filters: list[str] = []
        if country_filter:
            filters.append(f"country={country_filter}")
        if capability_filter:
            filters.append(f"capability={capability_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode regions", _call)

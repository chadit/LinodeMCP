"""Linode images list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_images_list_tool() -> Tool:
    """Create the linode_images_list tool."""
    return Tool(
        name="linode_images_list",
        description=(
            "Lists all available Linode images (OS images and custom images) "
            "with optional filtering by type, public status, or deprecated status"
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
                "type": {
                    "type": "string",
                    "description": "Filter images by type (manual, automatic)",
                },
                "is_public": {
                    "type": "string",
                    "description": "Filter by public status (true, false)",
                },
                "deprecated": {
                    "type": "string",
                    "description": "Filter by deprecated status (true, false)",
                },
            },
        },
    )


async def handle_linode_images_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_list tool request."""
    type_filter: str = arguments.get("type", "")
    is_public_filter: str | bool = arguments.get("is_public", "")
    deprecated_filter: str = arguments.get("deprecated", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        images = await client.list_images()

        if type_filter:
            images = [i for i in images if i.type.lower() == type_filter.lower()]

        if is_public_filter:
            want_public = (
                is_public_filter.lower() == "true"
                if isinstance(is_public_filter, str)
                else is_public_filter
            )
            images = [i for i in images if i.is_public == want_public]

        if deprecated_filter:
            want_deprecated = deprecated_filter.lower() == "true"
            images = [i for i in images if i.deprecated == want_deprecated]

        images_data = [
            {
                "id": i.id,
                "label": i.label,
                "type": i.type,
                "is_public": i.is_public,
                "deprecated": i.deprecated,
                "size": i.size,
                "vendor": i.vendor,
                "created": i.created,
            }
            for i in images
        ]

        response: dict[str, Any] = {
            "count": len(images),
            "images": images_data,
        }

        filters: list[str] = []
        if type_filter:
            filters.append(f"type={type_filter}")
        if is_public_filter:
            filters.append(f"is_public={is_public_filter}")
        if deprecated_filter:
            filters.append(f"deprecated={deprecated_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode images", _call)

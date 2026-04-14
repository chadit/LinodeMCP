"""Linode types list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_types_list_tool() -> Tool:
    """Create the linode_types_list tool."""
    return Tool(
        name="linode_types_list",
        description=(
            "Lists all available Linode instance types (plans) with pricing. "
            "Can filter by class (standard, dedicated, gpu, highmem, premium)."
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
                "class": {
                    "type": "string",
                    "description": (
                        "Filter types by class (standard, dedicated, gpu, highmem)"
                    ),
                },
            },
        },
    )


async def handle_linode_types_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_types_list tool request."""
    class_filter: str = arguments.get("class", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_types()

        if class_filter:
            types = [t for t in types if t.class_.lower() == class_filter.lower()]

        types_data = [
            {
                "id": t.id,
                "label": t.label,
                "class": t.class_,
                "disk": t.disk,
                "memory": t.memory,
                "vcpus": t.vcpus,
                "price": {"hourly": t.price.hourly, "monthly": t.price.monthly},
            }
            for t in types
        ]

        response: dict[str, Any] = {
            "count": len(types),
            "types": types_data,
        }

        if class_filter:
            response["filter"] = f"class={class_filter}"

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode types", _call)

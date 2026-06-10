"""Linode types list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def _is_type_id(value: str) -> bool:
    """Return True when value looks like a Linode type ID slug."""
    return bool(value) and all(c.isalnum() or c == "-" for c in value)


def create_linode_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_type_list tool."""
    return Tool(
        name="linode_type_list",
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
    ), Capability.Read


async def handle_linode_type_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_type_list tool request."""
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


def create_linode_type_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_type_get tool."""
    return Tool(
        name="linode_type_get",
        description="Gets details for a specific Linode instance type (plan).",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "type_id": {
                    "type": "string",
                    "description": (
                        "Linode type ID to retrieve (for example, 'g6-nanode-1')"
                    ),
                },
            },
            "required": ["type_id"],
        },
    ), Capability.Read


async def handle_linode_type_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_type_get tool request."""
    raw_type_id = arguments.get("type_id")
    if not isinstance(raw_type_id, str) or not raw_type_id.strip():
        return error_response("type_id is required")
    type_id = raw_type_id.strip()
    if not _is_type_id(type_id):
        return error_response("type_id must contain only letters, numbers, and hyphens")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        type_ = await client.get_type(type_id)
        return {
            "id": type_.id,
            "label": type_.label,
            "class": type_.class_,
            "disk": type_.disk,
            "memory": type_.memory,
            "vcpus": type_.vcpus,
            "gpus": type_.gpus,
            "network_out": type_.network_out,
            "transfer": type_.transfer,
            "price": {"hourly": type_.price.hourly, "monthly": type_.price.monthly},
            "addons": {
                "backups": {
                    "price": {
                        "hourly": type_.addons.backups.price.hourly,
                        "monthly": type_.addons.backups.price.monthly,
                    }
                }
            },
            "successor": type_.successor,
        }

    return await execute_tool(cfg, arguments, f"retrieve Linode type {type_id}", _call)

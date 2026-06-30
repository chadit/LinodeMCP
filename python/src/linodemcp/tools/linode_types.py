"""Linode types list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import type_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.proto_response import serialize_list_response
from linodemcp.tools.toolschemas import schema

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
    """Handle linode_type_list tool request.

    Class is a case-insensitive exact match against the type's class field,
    mirroring the Go list tool's fieldFilter. The output is the proto-canonical
    InstanceType envelope: every type element carries the full field set, so the
    raw API page flows through serialize_list_response unmodified.
    """
    class_filter: str = arguments.get("class", "")

    def _matches(type_: dict[str, Any]) -> bool:
        if not class_filter:
            return True
        return str(type_.get("class", "")).lower() == class_filter.lower()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/linode/types")
        return serialize_list_response(
            raw,
            "types",
            type_pb2.InstanceTypeListResponse(),
            filter_value=f"class={class_filter}" if class_filter else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve Linode types", _call)


def create_linode_type_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_type_get tool."""
    return Tool(
        name="linode_type_get",
        description="Gets details for a specific Linode instance type (plan).",
        inputSchema=schema("linode.mcp.v1.InstanceTypeGetInput"),
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
        result: dict[str, Any] = {
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
        }
        if type_.successor is not None:
            result["successor"] = type_.successor
        return result

    return await execute_tool(cfg, arguments, f"retrieve Linode type {type_id}", _call)

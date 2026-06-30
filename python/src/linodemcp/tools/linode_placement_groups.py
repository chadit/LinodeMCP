"""Placement group READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import Tool

from linodemcp.genpb.linode.mcp.v1 import placement_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from mcp.types import TextContent

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _pg_member_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw placement group member to proto-canonical form."""
    return {
        "linode_id": raw.get("linode_id", 0),
        "is_compliant": raw.get("is_compliant", False),
    }


def _pg_migration_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw placement group migration to proto-canonical form."""
    return {"linode_id": raw.get("linode_id", 0)}


def _pg_migrations_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape raw placement group migrations to proto-canonical form."""
    return {
        "inbound": [_pg_migration_to_dict(m) for m in raw.get("inbound", [])],
        "outbound": [_pg_migration_to_dict(m) for m in raw.get("outbound", [])],
    }


def placement_group_to_response_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw placement group API dict to proto-canonical form.

    members is always emitted as a list; migrations is omitted when absent,
    matching the proto linode.mcp.v1.PlacementGroup serialization.
    """
    body: dict[str, Any] = {
        "id": raw.get("id", 0),
        "label": raw.get("label", ""),
        "region": raw.get("region", ""),
        "placement_group_type": raw.get("placement_group_type", ""),
        "placement_group_policy": raw.get("placement_group_policy", ""),
        "is_compliant": raw.get("is_compliant", False),
        "members": [_pg_member_to_dict(m) for m in raw.get("members", [])],
    }
    migrations = raw.get("migrations")
    if migrations is not None:
        body["migrations"] = _pg_migrations_to_dict(migrations)
    return body


_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}
_GROUP_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The placement group ID.",
}
_PAGE_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "Page of results to return.",
}
_PAGE_SIZE_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 25,
    "maximum": 500,
    "description": "Number of results per page.",
}


def _parse_positive_int(value: Any, name: str) -> int | list[TextContent]:
    """Parse a positive integer argument, rejecting bools and path strings."""
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return error_response(f"{name} must be a positive integer")
    return value


def _parse_optional_int(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    """Parse an optional integer argument with inclusive bounds."""
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_placement_group_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_list tool."""
    return Tool(
        name="linode_placement_group_list",
        description="Lists placement groups",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "page": _PAGE_PROP,
                "page_size": _PAGE_SIZE_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_placement_group_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_list tool request."""
    try:
        page = _parse_optional_int(arguments, "page", 1)
        page_size = _parse_optional_int(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_placement_groups(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "placement_groups",
            placement_pb2.PlacementGroupListResponse(),
        )

    return await execute_tool(cfg, arguments, "list placement groups", _call)


def create_linode_placement_group_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_get tool."""
    return Tool(
        name="linode_placement_group_get",
        description="Gets a placement group",
        inputSchema=schema("linode.mcp.v1.PlacementGroupGetInput"),
    ), Capability.Read


async def handle_linode_placement_group_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_get tool request."""
    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_placement_group(group_id),
            placement_pb2.PlacementGroup(),
        )

    return await execute_tool(cfg, arguments, "get placement group", _call)

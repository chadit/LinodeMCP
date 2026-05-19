"""Placement group WRITE tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from mcp.types import Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from mcp.types import TextContent

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}
_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}
_GROUP_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The placement group ID.",
}
_LINODES_PROP: dict[str, Any] = {
    "type": "array",
    "description": "Linode IDs to assign or unassign from the placement group.",
    "items": {"type": "integer", "minimum": 1},
    "minItems": 1,
}


def _parse_positive_int(value: Any, name: str) -> int | list[TextContent]:
    """Parse a positive integer argument, rejecting bools and path strings."""
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return error_response(f"{name} must be a positive integer")
    return value


def _parse_linode_ids(
    arguments: dict[str, Any],
) -> tuple[list[int] | None, list[TextContent] | None]:
    """Parse and validate the linodes body field."""
    linodes = arguments.get("linodes")
    if not isinstance(linodes, list) or not linodes:
        return None, error_response(
            "linodes must be a non-empty array of positive integers"
        )

    raw_linodes = cast("list[object]", linodes)
    parsed: list[int] = []
    for linode_id in raw_linodes:
        if (
            not isinstance(linode_id, int)
            or isinstance(linode_id, bool)
            or linode_id < 1
        ):
            return None, error_response(
                "linodes must be a non-empty array of positive integers"
            )
        parsed.append(linode_id)
    return parsed, None


def create_linode_placement_group_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_delete tool."""
    return Tool(
        name="linode_placement_group_delete",
        description="Deletes a placement group",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "group_id": _GROUP_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["group_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_placement_group_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_delete tool request."""
    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_placement_group(group_id)
        return {"message": f"Placement group {group_id} deleted successfully"}

    return await execute_tool(cfg, arguments, "delete placement group", _call)


def create_linode_placement_group_assign_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_assign tool."""
    return Tool(
        name="linode_placement_group_assign",
        description="Assigns Linodes to a placement group",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "group_id": _GROUP_ID_PROP,
                "linodes": _LINODES_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["group_id", "linodes", "confirm"],
        },
    ), Capability.Write


async def handle_linode_placement_group_assign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_assign tool request."""
    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    linodes, linodes_error = _parse_linode_ids(arguments)
    if linodes_error is not None:
        return linodes_error
    if linodes is None:
        return error_response("linodes must be a non-empty array of positive integers")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.assign_placement_group(group_id, linodes)

    return await execute_tool(
        cfg, arguments, "assign Linodes to placement group", _call
    )


def create_linode_placement_group_unassign_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_unassign tool."""
    return Tool(
        name="linode_placement_group_unassign",
        description="Unassigns Linodes from a placement group",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "group_id": _GROUP_ID_PROP,
                "linodes": _LINODES_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["group_id", "linodes", "confirm"],
        },
    ), Capability.Write


async def handle_linode_placement_group_unassign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_unassign tool request."""
    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    linodes, linodes_error = _parse_linode_ids(arguments)
    if linodes_error is not None:
        return linodes_error
    if linodes is None:
        return error_response("linodes must be a non-empty array of positive integers")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.unassign_placement_group(group_id, linodes)

    return await execute_tool(
        cfg, arguments, "unassign Linodes from placement group", _call
    )

"""Placement group WRITE tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

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
_LABEL_PROP: dict[str, Any] = {
    "type": "string",
    "minLength": 1,
    "pattern": r"^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$",
    "description": "New placement group label.",
}
_REGION_PROP: dict[str, Any] = {
    "type": "string",
    "minLength": 1,
    "description": "Region where the placement group is created.",
}
_PLACEMENT_GROUP_TYPE_PROP: dict[str, Any] = {
    "type": "string",
    "enum": ["anti_affinity:local"],
    "description": "Placement group type.",
}
_PLACEMENT_GROUP_POLICY_PROP: dict[str, Any] = {
    "type": "string",
    "enum": ["strict", "flexible"],
    "description": "Placement group policy.",
}
_LABEL_ERROR = (
    "label must start and end with an alphanumeric character and contain only "
    "alphanumeric characters, hyphens, underscores, or periods"
)
_LABEL_PATTERN = re.compile(_LABEL_PROP["pattern"])
_PLACEMENT_GROUP_TYPES = {"anti_affinity:local"}
_PLACEMENT_GROUP_POLICIES = {"strict", "flexible"}


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


def _parse_placement_group_create(
    arguments: dict[str, Any],
) -> tuple[str, str, str, str] | list[TextContent]:
    """Parse and validate the create fields; return them or an error."""
    label = arguments.get("label")
    if not isinstance(label, str) or not _LABEL_PATTERN.fullmatch(label):
        return error_response(_LABEL_ERROR)
    region = arguments.get("region")
    if not isinstance(region, str) or not region:
        return error_response("region must be a non-empty string")
    placement_group_type = arguments.get("placement_group_type")
    if (
        not isinstance(placement_group_type, str)
        or placement_group_type not in _PLACEMENT_GROUP_TYPES
    ):
        return error_response("placement_group_type must be anti_affinity:local")
    placement_group_policy = arguments.get("placement_group_policy")
    if (
        not isinstance(placement_group_policy, str)
        or placement_group_policy not in _PLACEMENT_GROUP_POLICIES
    ):
        return error_response("placement_group_policy must be strict or flexible")
    return label, region, placement_group_type, placement_group_policy


def _parse_group_and_linodes(
    arguments: dict[str, Any],
) -> tuple[int, list[int]] | list[TextContent]:
    """Parse group_id and the linodes list; return the pair or an error."""
    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id
    linodes, linodes_error = _parse_linode_ids(arguments)
    if linodes_error is not None:
        return linodes_error
    if linodes is None:
        return error_response("linodes must be a non-empty array of positive integers")
    return group_id, linodes


def create_linode_placement_group_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_create tool."""
    return Tool(
        name="linode_placement_group_create",
        description="Creates a placement group",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "label": _LABEL_PROP,
                "region": _REGION_PROP,
                "placement_group_type": _PLACEMENT_GROUP_TYPE_PROP,
                "placement_group_policy": _PLACEMENT_GROUP_POLICY_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "label",
                "region",
                "placement_group_type",
                "placement_group_policy",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_placement_group_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_create tool request."""
    if is_dry_run(arguments):
        parsed = _parse_placement_group_create(arguments)
        if isinstance(parsed, list):
            return parsed
        return build_dry_run_response(
            "linode_placement_group_create",
            arguments.get("environment", ""),
            "POST",
            "/placement/groups",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    parsed = _parse_placement_group_create(arguments)
    if isinstance(parsed, list):
        return parsed
    label, region, placement_group_type, placement_group_policy = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_placement_group(
            label, region, placement_group_type, placement_group_policy
        )

    return await execute_tool(cfg, arguments, "create placement group", _call)


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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["group_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_placement_group_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_delete tool request."""
    if is_dry_run(arguments):
        group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
        if isinstance(group_id, list):
            return group_id
        gid = group_id

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_delete",
            "DELETE",
            f"/placement/groups/{gid}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_placement_group(group_id)
        return {"message": f"Placement group {group_id} deleted successfully"}

    return await execute_tool(cfg, arguments, "delete placement group", _call)


def create_linode_placement_group_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_update tool."""
    return Tool(
        name="linode_placement_group_update",
        description="Updates a placement group",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "group_id": _GROUP_ID_PROP,
                "label": _LABEL_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["group_id", "label", "confirm"],
        },
    ), Capability.Write


async def handle_linode_placement_group_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_update tool request."""
    if is_dry_run(arguments):
        group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
        if isinstance(group_id, list):
            return group_id
        gid = group_id

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_update",
            "PUT",
            f"/placement/groups/{gid}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    label = arguments.get("label")
    if not isinstance(label, str) or not _LABEL_PATTERN.fullmatch(label):
        return error_response(_LABEL_ERROR)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_placement_group(group_id, label)

    return await execute_tool(cfg, arguments, "update placement group", _call)


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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["group_id", "linodes", "confirm"],
        },
    ), Capability.Write


async def handle_linode_placement_group_assign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_assign tool request."""
    if is_dry_run(arguments):
        parsed = _parse_group_and_linodes(arguments)
        if isinstance(parsed, list):
            return parsed
        gid, _ = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_assign",
            "POST",
            f"/placement/groups/{gid}/assign",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    parsed = _parse_group_and_linodes(arguments)
    if isinstance(parsed, list):
        return parsed
    group_id, linodes = parsed

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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["group_id", "linodes", "confirm"],
        },
    ), Capability.Write


async def handle_linode_placement_group_unassign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_unassign tool request."""
    if is_dry_run(arguments):
        parsed = _parse_group_and_linodes(arguments)
        if isinstance(parsed, list):
            return parsed
        gid, _ = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_unassign",
            "POST",
            f"/placement/groups/{gid}/unassign",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    parsed = _parse_group_and_linodes(arguments)
    if isinstance(parsed, list):
        return parsed
    group_id, linodes = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.unassign_placement_group(group_id, linodes)

    return await execute_tool(
        cfg, arguments, "unassign Linodes from placement group", _call
    )

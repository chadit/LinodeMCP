"""Placement group WRITE tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import Tool

from linodemcp.genpb.linode.mcp.v1 import placement_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.proto_enum import enum_value_names
from linodemcp.tools.proto_response import raw_str, serialize_api_response
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from mcp.types import TextContent

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_LABEL_PROP: dict[str, Any] = {
    "type": "string",
    "minLength": 1,
    "pattern": r"^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$",
    "description": "New placement group label.",
}
_LABEL_ERROR = (
    "label must start and end with an alphanumeric character and contain only "
    "alphanumeric characters, hyphens, underscores, or periods"
)
_LABEL_PATTERN = re.compile(_LABEL_PROP["pattern"])
_PLACEMENT_GROUP_TYPES = {"anti_affinity:local"}


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
    if not isinstance(label, str):
        return error_response(_LABEL_ERROR)
    # Trim before matching so a whitespace-padded label is accepted and sent
    # trimmed, matching Go's requiredTrimmedString on the create path.
    label = label.strip()
    if not _LABEL_PATTERN.fullmatch(label):
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
    policy_values = enum_value_names(placement_pb2.PlacementGroupPolicy.Value)
    if (
        not isinstance(placement_group_policy, str)
        or placement_group_policy not in policy_values
    ):
        return error_response(
            "placement_group_policy must be one of: " + ", ".join(policy_values)
        )
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
        inputSchema=schema("linode.mcp.v1.PlacementGroupCreateInput"),
    ), Capability.Write


async def handle_linode_placement_group_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_create tool request."""
    if is_dry_run(arguments):
        parsed = _parse_placement_group_create(arguments)
        if isinstance(parsed, list):
            return parsed
        label, region, placement_group_type, placement_group_policy = parsed
        return build_dry_run_response(
            "linode_placement_group_create",
            arguments.get("environment", ""),
            "POST",
            "/placement/groups",
            None,
            side_effects=[
                (
                    f"A new placement group {label!r} "
                    f"({placement_group_type}, {placement_group_policy} policy) "
                    f"will be created in region {region}."
                )
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a placement group. Set confirm=true to proceed."
        )

    parsed = _parse_placement_group_create(arguments)
    if isinstance(parsed, list):
        return parsed
    label, region, placement_group_type, placement_group_policy = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        placement_group = await client.create_placement_group(
            label, region, placement_group_type, placement_group_policy
        )
        return serialize_api_response(
            {
                "message": (
                    f"Placement group '{raw_str(placement_group, 'label')}' "
                    "created successfully"
                ),
                "placement_group": placement_group,
            },
            placement_pb2.PlacementGroupWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create placement group", _call)


def create_linode_placement_group_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_delete tool."""
    return Tool(
        name="linode_placement_group_delete",
        description="Deletes a placement group" + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.PlacementGroupDeleteInput"),
    ), Capability.Destroy


def _placement_group_delete_dependency_walk(group_state: Any) -> DryRunDetails:
    """Phase 2 Tier A walk for placement group delete. The group state (already
    fetched for current_state) carries the member Linodes; deleting the group
    detaches them (the instances are not deleted). No extra API call.
    """
    details: DryRunDetails = {}
    if not isinstance(group_state, dict):
        return details

    group = cast("dict[str, Any]", group_state)
    members = cast("list[dict[str, Any]]", group.get("members", []))
    dependencies: list[dict[str, Any]] = [
        {
            "kind": "instance",
            "id": member.get("linode_id"),
            "action": "detached",
            "note": "Linode is removed from the placement group; "
            "the instance is not deleted.",
        }
        for member in members
    ]
    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            (
                f"Deleting this placement group detaches {len(dependencies)} "
                "Linode(s); the instances are not deleted."
            )
        ]
    return details


async def _placement_group_delete_two_stage(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id
    gid = group_id

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_placement_group(gid)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_placement_group(gid)
        return serialize_api_response(
            {"message": f"Placement group {gid} deleted successfully"},
            placement_pb2.PlacementGroupDeleteResponse(),
        )

    async def _ts_walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _placement_group_delete_dependency_walk(state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_placement_group_delete",
        method="DELETE",
        path=f"/placement/groups/{gid}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("PlacementGroup"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_placement_group_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_delete tool request."""
    two_stage = await _placement_group_delete_two_stage(arguments, cfg)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
        if isinstance(group_id, list):
            return group_id
        gid = group_id

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _placement_group_delete_dependency_walk(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_delete",
            "DELETE",
            f"/placement/groups/{gid}",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm=true is required to delete the placement group")

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_placement_group(group_id)
        return serialize_api_response(
            {"message": f"Placement group {group_id} deleted successfully"},
            placement_pb2.PlacementGroupDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete placement group", _call)


def create_linode_placement_group_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_update tool."""
    return Tool(
        name="linode_placement_group_update",
        description="Updates a placement group",
        inputSchema=schema("linode.mcp.v1.PlacementGroupUpdateInput"),
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

        label = arguments.get("label", "")

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [f"The placement group's label is set to {label!r}."]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_update",
            "PUT",
            f"/placement/groups/{gid}",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a placement group. Set confirm=true to proceed."
        )

    group_id = _parse_positive_int(arguments.get("group_id"), "group_id")
    if isinstance(group_id, list):
        return group_id

    label = arguments.get("label")
    if not isinstance(label, str) or not _LABEL_PATTERN.fullmatch(label):
        return error_response(_LABEL_ERROR)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        placement_group = await client.update_placement_group(group_id, label)
        return serialize_api_response(placement_group, placement_pb2.PlacementGroup())

    return await execute_tool(cfg, arguments, "update placement group", _call)


def create_linode_placement_group_assign_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_assign tool."""
    return Tool(
        name="linode_placement_group_assign",
        description="Assigns Linodes to a placement group",
        inputSchema=schema("linode.mcp.v1.PlacementGroupAssignInput"),
    ), Capability.Write


def _placement_group_membership_side_effects(
    linodes: list[int], group_id: int, action: str
) -> DryRunDetails:
    """Tier B preview shared by assign/unassign. Names the Linodes whose
    membership changes; action is 'assigned to' or 'removed from'.
    """
    return {
        "side_effects": [
            f"Linode {linode_id} will be {action} placement group {group_id}."
            for linode_id in linodes
        ]
    }


async def handle_linode_placement_group_assign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_assign tool request."""
    if is_dry_run(arguments):
        parsed = _parse_group_and_linodes(arguments)
        if isinstance(parsed, list):
            return parsed
        gid, linodes = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _placement_group_membership_side_effects(linodes, gid, "assigned to")

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_assign",
            "POST",
            f"/placement/groups/{gid}/assign",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This assigns Linodes to a placement group. Set confirm=true to proceed."
        )

    parsed = _parse_group_and_linodes(arguments)
    if isinstance(parsed, list):
        return parsed
    group_id, linodes = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        placement_group = await client.assign_placement_group(group_id, linodes)
        return serialize_api_response(
            {
                "message": (
                    f"Assigned {len(linodes)} Linode(s) to placement group {group_id}"
                ),
                "placement_group": placement_group,
            },
            placement_pb2.PlacementGroupWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "assign Linodes to placement group", _call
    )


def create_linode_placement_group_unassign_tool() -> tuple[Tool, Capability]:
    """Create the linode_placement_group_unassign tool."""
    return Tool(
        name="linode_placement_group_unassign",
        description="Unassigns Linodes from a placement group",
        inputSchema=schema("linode.mcp.v1.PlacementGroupUnassignInput"),
    ), Capability.Write


async def handle_linode_placement_group_unassign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_placement_group_unassign tool request."""
    if is_dry_run(arguments):
        parsed = _parse_group_and_linodes(arguments)
        if isinstance(parsed, list):
            return parsed
        gid, linodes = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_placement_group(gid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _placement_group_membership_side_effects(
                linodes, gid, "removed from"
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_placement_group_unassign",
            "POST",
            f"/placement/groups/{gid}/unassign",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This unassigns Linodes from a placement group. Set confirm=true to "
            "proceed."
        )

    parsed = _parse_group_and_linodes(arguments)
    if isinstance(parsed, list):
        return parsed
    group_id, linodes = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        placement_group = await client.unassign_placement_group(group_id, linodes)
        return serialize_api_response(
            {
                "message": (
                    f"Linodes unassigned from placement group {group_id} successfully"
                ),
                "placement_group": placement_group,
            },
            placement_pb2.PlacementGroupWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "unassign Linodes from placement group", _call
    )

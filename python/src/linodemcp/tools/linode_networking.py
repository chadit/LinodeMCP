"""Networking tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import ip_pb2, vlan_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_vlan_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vlan_list tool."""
    return Tool(
        name="linode_vlan_list",
        description="Lists all VLANs on the account",
        inputSchema=schema("linode.mcp.v1.VLANListInput"),
    ), Capability.Read


async def handle_linode_vlan_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vlan_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vlans = await client.list_vlans(page=page, page_size=page_size)
        return serialize_list_response(
            {"data": vlans},
            "vlans",
            vlan_pb2.VLANListResponse(),
        )

    return await execute_tool(cfg, arguments, "list VLANs", _call)


# Lowercase region slug (mirrors Go's validRegionSlugRegex) used to reject a
# malformed region_id locally on vlan_delete instead of forwarding it.
_REGION_SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$")


def create_linode_vlan_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_vlan_delete tool."""
    return Tool(
        name="linode_vlan_delete",
        description="Deletes a VLAN. Pass dry_run=true to preview without deleting."
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.VLANDeleteInput"),
    ), Capability.Destroy


def _find_vlan(
    vlans: list[dict[str, Any]], region_id: str, label: str
) -> dict[str, Any] | None:
    """Find a VLAN by region+label. VLANs have no single-GET endpoint."""
    for vlan in vlans:
        if vlan.get("region") == region_id and vlan.get("label") == label:
            return vlan
    return None


async def _fetch_vlan_state(
    client: RetryableClient, region_id: str, label: str
) -> dict[str, Any]:
    """Resolve a VLAN by region+label. VLANs expose only a list endpoint, so
    this lists and filters; raises ValueError when no VLAN matches.
    """
    vlans: list[dict[str, Any]] = await client.list_vlans()
    match = _find_vlan(vlans, region_id, label)
    if match is None:
        msg = f"VLAN not found: {label} in region {region_id}"
        raise ValueError(msg)
    return match


async def _vlan_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, region_id: str, label: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await _fetch_vlan_state(client, region_id, label)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vlan(region_id, label)
        return serialize_api_response(
            {
                "message": (
                    f"VLAN {label} deleted successfully from region {region_id}"
                ),
                "region_id": region_id,
                "label": label,
            },
            vlan_pb2.VLANDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_vlan_delete",
        method="DELETE",
        path=f"/networking/vlans/{region_id}/{label}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("VLAN"),
    )


def _vlan_delete_ids(
    arguments: dict[str, Any],
) -> tuple[str, str] | list[TextContent]:
    """Validate region_id (lowercase slug, mirrors Go) + label, or return an error.

    Both branches need region+label, and the spec says dry-run errors on missing
    required args the same way the real call would.
    """
    region_id = arguments.get("region_id", "")
    label = arguments.get("label", "")
    if not isinstance(region_id, str) or not region_id:
        return error_response("region_id is required")
    if not _REGION_SLUG_RE.fullmatch(region_id):
        return error_response("region_id must be a lowercase region slug")
    if not isinstance(label, str) or not label:
        return error_response("label is required")
    return region_id, label


async def handle_linode_vlan_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vlan_delete tool request."""
    ids = _vlan_delete_ids(arguments)
    if isinstance(ids, list):
        return ids
    region_id, label = ids

    two_stage = await _vlan_delete_two_stage(arguments, cfg, region_id, label)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        # VLANs expose only a list endpoint, so the dry-run fetch lists
        # and filters to the matching region+label.
        async def _fetch(client: RetryableClient) -> Any:
            vlans = await client.list_vlans()
            match = _find_vlan(vlans, region_id, label)
            if match is None:
                msg = f"VLAN not found: {label} in region {region_id}"
                raise ValueError(msg)
            return match

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vlan_delete",
            "DELETE",
            f"/networking/vlans/{region_id}/{label}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("This deletes a VLAN. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vlan(region_id, label)
        return serialize_api_response(
            {
                "message": (
                    f"VLAN {label} deleted successfully from region {region_id}"
                ),
                "region_id": region_id,
                "label": label,
            },
            vlan_pb2.VLANDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete VLAN", _call)


def create_linode_networking_ipv4_share_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ipv4_share tool."""
    return Tool(
        name="linode_networking_ipv4_share",
        description="Shares IPv4 addresses with a Linode",
        inputSchema=schema("linode.mcp.v1.NetworkingIPv4ShareInput"),
    ), Capability.Write


def _parse_ipv4_share(
    arguments: dict[str, Any],
) -> tuple[list[str], int] | list[TextContent]:
    """Parse ips and linode_id; return the pair or an error response."""
    ips = arguments.get("ips")
    linode_id = arguments.get("linode_id")
    if not isinstance(ips, list):
        return error_response("ips must be a non-empty list of IPv4 addresses")
    typed_ips = cast("list[str]", ips)
    if len(typed_ips) == 0:
        return error_response("ips must be a non-empty list of IPv4 addresses")
    if linode_id is None:
        return error_response("linode_id is required")
    if not isinstance(linode_id, int):
        return error_response("linode_id must be an integer")
    return typed_ips, linode_id


async def handle_linode_networking_ipv4_share(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ipv4_share tool request."""
    if is_dry_run(arguments):
        parsed = _parse_ipv4_share(arguments)
        if isinstance(parsed, list):
            return parsed
        return build_dry_run_response(
            "linode_networking_ipv4_share",
            arguments.get("environment", ""),
            "POST",
            "/networking/ipv4/share",
            None,
            request_body={"ips": parsed[0], "linode_id": parsed[1]},
        )

    confirm = arguments.get("confirm", False)
    if confirm is not True:
        return error_response(
            "This changes shared IP assignments. Set confirm=true to proceed."
        )

    parsed = _parse_ipv4_share(arguments)
    if isinstance(parsed, list):
        return parsed
    typed_ips, linode_id = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.share_ipv4s(typed_ips, linode_id)
        return serialize_api_response(
            {
                "message": "Networking IP sharing updated",
                "linode_id": linode_id,
                "ips": typed_ips,
            },
            ip_pb2.NetworkingIPShareWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "share IPv4 addresses", _call)


def create_linode_networking_ip_share_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ip_share tool."""
    return Tool(
        name="linode_networking_ip_share",
        description="Shares IP addresses with a Linode",
        inputSchema=schema("linode.mcp.v1.NetworkingIPShareInput"),
    ), Capability.Write


def _parse_ips_share(
    arguments: dict[str, Any],
) -> tuple[list[str], int] | list[TextContent]:
    """Parse ips and linode_id; return the pair or an error response."""
    ips = arguments.get("ips")
    linode_id = arguments.get("linode_id")
    if not isinstance(ips, list):
        return error_response("ips must be a non-empty list of IP addresses")
    raw_ips = cast("list[object]", ips)
    if len(raw_ips) == 0:
        return error_response("ips must be a non-empty list of IP addresses")
    if not all(isinstance(ip, str) and ip for ip in raw_ips):
        return error_response("ips entries must be non-empty strings")
    typed_ips = [ip for ip in raw_ips if isinstance(ip, str)]
    if linode_id is None:
        return error_response("linode_id is required")
    if not isinstance(linode_id, int):
        return error_response("linode_id must be an integer")
    return typed_ips, linode_id


async def handle_linode_networking_ip_share(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ip_share tool request."""
    if is_dry_run(arguments):
        parsed = _parse_ips_share(arguments)
        if isinstance(parsed, list):
            return parsed
        return build_dry_run_response(
            "linode_networking_ip_share",
            arguments.get("environment", ""),
            "POST",
            "/networking/ips/share",
            None,
            request_body={"ips": parsed[0], "linode_id": parsed[1]},
        )

    confirm = arguments.get("confirm", False)
    if confirm is not True:
        return error_response(
            "This changes shared IP assignments. Set confirm=true to proceed."
        )

    parsed = _parse_ips_share(arguments)
    if isinstance(parsed, list):
        return parsed
    typed_ips, linode_id = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.share_ips(typed_ips, linode_id)
        return serialize_api_response(
            {
                "message": "Networking IP sharing updated",
                "linode_id": linode_id,
                "ips": typed_ips,
            },
            ip_pb2.NetworkingIPShareWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "share IP addresses", _call)


def create_linode_networking_ipv4_assign_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ipv4_assign tool."""
    return Tool(
        name="linode_networking_ipv4_assign",
        description="Assigns IPv4 addresses to Linodes in a region",
        inputSchema=schema("linode.mcp.v1.NetworkingIPv4AssignInput"),
    ), Capability.Write


def _parse_ipv4_assign(
    arguments: dict[str, Any],
) -> tuple[str, list[dict[str, Any]]] | list[TextContent]:
    """Parse region and assignments; return the pair or an error response."""
    region = arguments.get("region")
    assignments = arguments.get("assignments")

    if not isinstance(region, str) or not region.strip():
        return error_response("region is required")
    if not isinstance(assignments, list):
        return error_response(
            "assignments must be a non-empty list of assignment objects"
        )
    raw_assignments = cast("list[object]", assignments)
    if not raw_assignments:
        return error_response(
            "assignments must be a non-empty list of assignment objects"
        )

    typed_assignments: list[dict[str, Any]] = []
    assignment_error: str | None = None
    for assignment in raw_assignments:
        if not isinstance(assignment, dict):
            assignment_error = (
                "assignments must be a non-empty list of assignment objects"
            )
            break
        assignment_dict = cast("dict[str, Any]", assignment)
        address = assignment_dict.get("address")
        linode_id = assignment_dict.get("linode_id")
        if not isinstance(address, str) or not address.strip():
            assignment_error = "each assignment requires address"
            break
        if not isinstance(linode_id, int):
            assignment_error = "each assignment requires integer linode_id"
            break
        typed_assignments.append(assignment_dict)

    if assignment_error is not None:
        return error_response(assignment_error)

    return region, typed_assignments


async def handle_linode_networking_ipv4_assign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ipv4_assign tool request."""
    if is_dry_run(arguments):
        parsed = _parse_ipv4_assign(arguments)
        if isinstance(parsed, list):
            return parsed
        region, typed_assignments = parsed
        request_body = {"region": region, "assignments": typed_assignments}
        return build_dry_run_response(
            "linode_networking_ipv4_assign",
            arguments.get("environment", ""),
            "POST",
            "/networking/ipv4/assign",
            None,
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This assigns IPv4 addresses to Linodes. Set confirm=true to proceed."
        )

    parsed = _parse_ipv4_assign(arguments)
    if isinstance(parsed, list):
        return parsed
    region, typed_assignments = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.assign_ipv4s(region, typed_assignments)
        return serialize_api_response(
            {
                "message": "Networking IPv4 assignments updated",
                "region": region,
                "assignments": typed_assignments,
            },
            ip_pb2.NetworkingIPAssignWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "assign IPv4 addresses", _call)


def create_linode_networking_ip_assign_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ip_assign tool.

    The generic assign endpoint (/networking/ips/assign), distinct from
    linode_networking_ipv4_assign which hits /networking/ipv4/assign.
    """
    return Tool(
        name="linode_networking_ip_assign",
        description=(
            "Assigns IP addresses to Linodes in a region. WARNING: This "
            "changes IP ownership assignments."
        ),
        inputSchema=schema("linode.mcp.v1.NetworkingIPAssignInput"),
    ), Capability.Write


async def handle_linode_networking_ip_assign(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ip_assign tool request."""
    if is_dry_run(arguments):
        parsed = _parse_ipv4_assign(arguments)
        if isinstance(parsed, list):
            return parsed
        region, typed_assignments = parsed
        request_body = {"region": region, "assignments": typed_assignments}
        return build_dry_run_response(
            "linode_networking_ip_assign",
            arguments.get("environment", ""),
            "POST",
            "/networking/ips/assign",
            None,
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This assigns IP addresses to Linodes. Set confirm=true to proceed."
        )

    parsed = _parse_ipv4_assign(arguments)
    if isinstance(parsed, list):
        return parsed
    region, typed_assignments = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.assign_ips(region, typed_assignments)
        return serialize_api_response(
            {
                "message": "Networking IP assignments updated",
                "region": region,
                "assignments": typed_assignments,
            },
            ip_pb2.NetworkingIPAssignWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "assign IP addresses", _call)

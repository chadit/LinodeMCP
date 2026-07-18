"""VPC WRITE tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import ip_pb2, vpc_pb2
from linodemcp.linode import APIError, NetworkError
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    valid_ipv6_prefix,
)
from linodemcp.tools.proto_response import raw_int, raw_str, serialize_api_response
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_IPV6_RANGE_KEY = "range"

_IPV6_PREFIX_LENGTH_KEY = "prefix_length"
_LINODE_ID_KEY = "linode_id"
_ROUTE_TARGET_KEY = "route_target"


def _parse_vpc_subnet_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse and validate vpc_id and subnet_id from arguments.

    Returns a (vpc_id, subnet_id) tuple on success, or an error
    response list on failure.
    """
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    subnet_id_str = arguments.get("subnet_id", "")
    if not subnet_id_str:
        return error_response("subnet_id is required")
    try:
        subnet_id = int(subnet_id_str)
    except ValueError:
        return error_response("subnet_id must be a valid integer")

    return (vpc_id, subnet_id)


def _parse_ipv6_prefix_length(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse and validate the IPv6 range prefix length."""
    prefix_length_value = arguments.get(_IPV6_PREFIX_LENGTH_KEY)
    if prefix_length_value is None:
        return error_response("prefix_length is required")
    try:
        prefix_length = int(str(prefix_length_value))
    except ValueError:
        return error_response("prefix_length must be 56 or 64")
    if prefix_length not in (56, 64):
        return error_response("prefix_length must be 56 or 64")
    return prefix_length


def _parse_ipv6_range_target(
    arguments: dict[str, Any],
) -> tuple[int | None, str | None] | list[TextContent]:
    """Parse and validate the IPv6 range assignment target."""
    linode_id_value = arguments.get(_LINODE_ID_KEY)
    route_target_value = arguments.get(_ROUTE_TARGET_KEY)
    has_linode_id = linode_id_value not in (None, "")
    has_route_target = route_target_value not in (None, "")

    if not has_linode_id and not has_route_target:
        return error_response("linode_id or route_target is required")
    if has_linode_id and has_route_target:
        return error_response("linode_id and route_target are mutually exclusive")

    if has_linode_id:
        try:
            return int(str(linode_id_value)), None
        except ValueError:
            return error_response("linode_id must be a valid integer")

    if not isinstance(route_target_value, str) or not route_target_value.strip():
        return error_response("route_target must be a non-empty string")
    return None, route_target_value.strip()


def _parse_ipv6_range_create_args(
    arguments: dict[str, Any],
) -> tuple[int, int | None, str | None] | list[TextContent]:
    """Parse and validate create IPv6 range arguments."""
    prefix_length = _parse_ipv6_prefix_length(arguments)
    if isinstance(prefix_length, list):
        return prefix_length

    target = _parse_ipv6_range_target(arguments)
    if isinstance(target, list):
        return target

    linode_id, route_target = target
    return prefix_length, linode_id, route_target


def create_linode_vpc_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_create tool."""
    return Tool(
        name="linode_vpc_create",
        description="Creates a new VPC",
        inputSchema=schema("linode.mcp.v1.VpcCreateInput"),
    ), Capability.Write


def _vpc_create_error(label: str, region: str) -> list[TextContent] | None:
    """Validate VPC create args; return an error response or None."""
    if not label:
        return error_response("label is required")
    if not region:
        return error_response("region is required")
    return None


async def handle_linode_vpc_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_create tool request."""
    label = arguments.get("label", "")
    region = arguments.get("region", "")

    if is_dry_run(arguments):
        fields_error = _vpc_create_error(label, region)
        if fields_error is not None:
            return fields_error
        return build_dry_run_response(
            "linode_vpc_create",
            arguments.get("environment", ""),
            "POST",
            "/vpcs",
            None,
            side_effects=[f"A new VPC {label!r} will be created in region {region}."],
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This creates a billable VPC resource. Set confirm=true to proceed."
        )

    fields_error = _vpc_create_error(label, region)
    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vpc = await client.create_vpc(
            label=label,
            region=region,
            description=arguments.get("description"),
            subnets=arguments.get("subnets"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"VPC '{raw_str(vpc, 'label')}' (ID: {raw_int(vpc, 'id')}) "
                    f"created in {raw_str(vpc, 'region')}"
                ),
                "vpc": vpc,
            },
            vpc_pb2.VpcWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create VPC", _call)


def create_linode_vpc_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_update tool."""
    return Tool(
        name="linode_vpc_update",
        description="Updates an existing VPC",
        inputSchema=schema("linode.mcp.v1.VpcUpdateInput"),
    ), Capability.Write


def _vpc_update_side_effects(
    state: Any, new_label: Any, new_description: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for VPC update. Reports the label change against the
    fetched state (a dict) and notes a description change.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = ""
        if isinstance(state, dict):
            from_label = cast("dict[str, Any]", state).get("label", "")
        if from_label and from_label != new_label:
            side_effects.append(f"Label changes from {from_label!r} to {new_label!r}.")
        else:
            side_effects.append(f"Label is set to {new_label!r}.")
    if new_description:
        side_effects.append("The VPC description is updated.")
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_vpc_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_update tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_vpc(vpc_id)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _vpc_update_side_effects(
                state, arguments.get("label"), arguments.get("description")
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vpc_update",
            "PUT",
            f"/vpcs/{vpc_id}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This modifies the VPC configuration. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vpc = await client.update_vpc(
            vpc_id=vpc_id,
            label=arguments.get("label"),
            description=arguments.get("description"),
        )
        return serialize_api_response(
            {
                "message": f"VPC {vpc_id} modified successfully",
                "vpc": vpc,
            },
            vpc_pb2.VpcWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update VPC", _call)


def create_linode_vpc_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_delete tool."""
    return Tool(
        name="linode_vpc_delete",
        description="Deletes a VPC. Pass dry_run=true to preview without deleting."
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.VpcDeleteInput"),
    ), Capability.Destroy


async def _vpc_delete_dependency_walk(
    client: RetryableClient, vpc_id: int
) -> DryRunDetails:
    """Phase 2 Tier A walk for VPC delete. Each subnet is destroyed with the
    VPC, and any Linode interfaces in a subnet are detached, so subnets are
    surfaced as cascade_deleted dependencies with their attached-interface
    count. Best-effort: a failed subnet list becomes a warning.
    """
    details: DryRunDetails = {}
    try:
        subnets = await client.list_vpc_subnets(vpc_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list VPC subnets: {exc}"]
        return details

    attached_interfaces = 0
    dependencies: list[dict[str, Any]] = []
    for subnet in subnets:
        linodes = cast("list[Any]", subnet.get("linodes", []))
        attached_interfaces += len(linodes)
        dependencies.append(
            {
                "kind": "vpc_subnet",
                "id": subnet.get("id"),
                "label": subnet.get("label", ""),
                "action": "cascade_deleted",
                "note": f"{len(linodes)} attached Linode interface(s)",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
    if attached_interfaces > 0:
        details["warnings"] = [
            f"{attached_interfaces} Linode interface(s) across "
            f"{len(subnets)} subnet(s) will be detached."
        ]
    return details


async def _vpc_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, vpc_id: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_vpc(vpc_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc(vpc_id)
        return serialize_api_response(
            {
                "message": f"VPC {vpc_id} removed successfully",
                "vpc_id": vpc_id,
            },
            vpc_pb2.VpcDeleteResponse(),
        )

    async def _ts_walk(client: RetryableClient, _state: Any) -> DryRunDetails:
        return await _vpc_delete_dependency_walk(client, vpc_id)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_vpc_delete",
        method="DELETE",
        path=f"/vpcs/{vpc_id}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("VPC"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_vpc_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_delete tool request."""
    vpc_id_str = arguments.get("vpc_id", "")

    # Both branches need a valid vpc_id, and the spec says dry-run
    # errors on missing required args the same way the real call would.
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    two_stage = await _vpc_delete_two_stage(arguments, cfg, vpc_id)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_vpc(vpc_id)

        async def _walk(client: RetryableClient, _state: Any) -> DryRunDetails:
            return await _vpc_delete_dependency_walk(client, vpc_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vpc_delete",
            "DELETE",
            f"/vpcs/{vpc_id}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This is irreversible. All subnets in the VPC will also be deleted. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc(vpc_id)
        return serialize_api_response(
            {
                "message": f"VPC {vpc_id} removed successfully",
                "vpc_id": vpc_id,
            },
            vpc_pb2.VpcDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete VPC", _call)


def create_linode_vpc_subnet_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_create tool."""
    return Tool(
        name="linode_vpc_subnet_create",
        description="Creates a new subnet in a VPC",
        inputSchema=schema("linode.mcp.v1.VpcSubnetCreateInput"),
    ), Capability.Write


def _vpc_subnet_create_error(
    vpc_id_str: Any, label: str, ipv4: str
) -> tuple[int, None] | tuple[None, list[TextContent]]:
    """Parse+validate subnet create args; return (vpc_id, None) or (None, err)."""
    if not vpc_id_str:
        return None, error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return None, error_response("vpc_id must be a valid integer")
    if not label:
        return None, error_response("label is required")
    if not ipv4:
        return None, error_response("ipv4 is required")
    return vpc_id, None


async def handle_linode_vpc_subnet_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_create tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    label = arguments.get("label", "")
    ipv4 = arguments.get("ipv4", "")

    vpc_id, fields_error = _vpc_subnet_create_error(vpc_id_str, label, ipv4)

    if is_dry_run(arguments):
        if fields_error is not None:
            return fields_error
        effect = f"A new subnet {label!r} will be created in VPC {vpc_id}"
        if ipv4:
            effect += f" with IPv4 range {ipv4}"
        return build_dry_run_response(
            "linode_vpc_subnet_create",
            arguments.get("environment", ""),
            "POST",
            f"/vpcs/{vpc_id}/subnets",
            None,
            side_effects=[f"{effect}."],
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This creates a new subnet in the VPC. Set confirm=true to proceed."
        )

    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        subnet = await client.create_vpc_subnet(
            vpc_id=cast("int", vpc_id),
            label=label,
            ipv4=ipv4,
        )
        return serialize_api_response(
            {
                "message": (
                    f"Subnet '{raw_str(subnet, 'label')}' "
                    f"(ID: {raw_int(subnet, 'id')}) created in VPC {vpc_id}"
                ),
                "subnet": subnet,
            },
            vpc_pb2.VpcSubnetWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create VPC subnet", _call)


def create_linode_vpc_subnet_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_update tool."""
    return Tool(
        name="linode_vpc_subnet_update",
        description="Updates a VPC subnet",
        inputSchema=schema("linode.mcp.v1.VpcSubnetUpdateInput"),
    ), Capability.Write


async def handle_linode_vpc_subnet_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_update tool request."""
    ids = _parse_vpc_subnet_ids(arguments)
    if isinstance(ids, list):
        return ids
    vpc_id, subnet_id = ids

    label = arguments.get("label", "")
    if not label:
        return error_response("label is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_vpc_subnet(vpc_id, subnet_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vpc_subnet_update",
            "PUT",
            f"/vpcs/{vpc_id}/subnets/{subnet_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This modifies the subnet configuration. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        subnet = await client.update_vpc_subnet(
            vpc_id=vpc_id,
            subnet_id=subnet_id,
            label=label,
        )
        return serialize_api_response(
            {
                "message": (
                    f"Subnet {subnet_id} in VPC {vpc_id} modified successfully"
                ),
                "subnet": subnet,
            },
            vpc_pb2.VpcSubnetWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update VPC subnet", _call)


def create_linode_vpc_subnet_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_delete tool."""
    return Tool(
        name="linode_vpc_subnet_delete",
        description=(
            "Deletes a VPC subnet. Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.VpcSubnetDeleteInput"),
    ), Capability.Destroy


async def _vpc_subnet_delete_dependency_walk(
    client: RetryableClient, vpc_id: int, subnet_state: Any
) -> DryRunDetails:
    """Phase 2 Tier A walk for VPC subnet delete. The subnet state (already
    fetched for current_state) carries the Linodes with interfaces in this
    subnet; each is surfaced as a detached dependency. The parent VPC is
    fetched once to label the warning, mirroring the Go walk; a failed VPC
    fetch just leaves the label empty.
    """
    details: DryRunDetails = {}
    if not isinstance(subnet_state, dict):
        return details

    subnet = cast("dict[str, Any]", subnet_state)
    linodes = cast("list[dict[str, Any]]", subnet.get("linodes", []))
    dependencies: list[dict[str, Any]] = [
        {
            "kind": "instance",
            "id": linode_ref.get("id"),
            "action": "detached",
            "note": (
                f"{len(cast('list[Any]', linode_ref.get('interfaces', [])))} "
                "interface(s) in this subnet are detached."
            ),
        }
        for linode_ref in linodes
    ]
    if not dependencies:
        return details

    vpc_label = ""
    try:
        vpc = await client.get_vpc(vpc_id)
    except (APIError, NetworkError, httpx.HTTPError):
        vpc = None
    if vpc is not None:
        vpc_label = str(vpc.get("label", ""))

    subnet_label = str(subnet.get("label", ""))
    details["dependencies"] = dependencies
    details["warnings"] = [
        f'{len(dependencies)} Linode(s) have interfaces in subnet "{subnet_label}" '
        f'(VPC "{vpc_label}") and will be detached.'
    ]
    return details


async def _vpc_subnet_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, vpc_id: int, subnet_id: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_vpc_subnet(vpc_id, subnet_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc_subnet(vpc_id, subnet_id)
        return serialize_api_response(
            {
                "message": f"Subnet {subnet_id} deleted from VPC {vpc_id} successfully",
                "vpc_id": vpc_id,
                "subnet_id": subnet_id,
            },
            vpc_pb2.VpcSubnetDeleteResponse(),
        )

    async def _ts_walk(client: RetryableClient, state: Any) -> DryRunDetails:
        return await _vpc_subnet_delete_dependency_walk(client, vpc_id, state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_vpc_subnet_delete",
        method="DELETE",
        path=f"/vpcs/{vpc_id}/subnets/{subnet_id}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("VPCSubnet"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_vpc_subnet_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_delete tool request."""
    # Both branches need valid IDs, and the spec says dry-run errors on
    # missing required args the same way the real call would.
    ids = _parse_vpc_subnet_ids(arguments)
    if isinstance(ids, list):
        return ids
    vpc_id, subnet_id = ids

    two_stage = await _vpc_subnet_delete_two_stage(arguments, cfg, vpc_id, subnet_id)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_vpc_subnet(vpc_id, subnet_id)

        async def _walk(client: RetryableClient, state: Any) -> DryRunDetails:
            return await _vpc_subnet_delete_dependency_walk(client, vpc_id, state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vpc_subnet_delete",
            "DELETE",
            f"/vpcs/{vpc_id}/subnets/{subnet_id}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This is irreversible. The subnet will be permanently deleted. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc_subnet(vpc_id, subnet_id)
        return serialize_api_response(
            {
                "message": f"Subnet {subnet_id} deleted from VPC {vpc_id} successfully",
                "vpc_id": vpc_id,
                "subnet_id": subnet_id,
            },
            vpc_pb2.VpcSubnetDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete VPC subnet", _call)


def create_linode_ipv6_range_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_create tool."""
    return Tool(
        name="linode_ipv6_range_create",
        description="Creates an IPv6 range",
        inputSchema=schema("linode.mcp.v1.IPv6RangeCreateInput"),
    ), Capability.Write


async def handle_linode_ipv6_range_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_create tool request."""
    if is_dry_run(arguments):
        parsed = _parse_ipv6_range_create_args(arguments)
        if isinstance(parsed, list):
            return parsed
        prefix_length, linode_id, route_target = parsed
        effect = (
            f"A new IPv6 range with prefix length /{prefix_length} will be allocated"
        )
        if route_target:
            effect += f" and routed to {route_target}"
        elif linode_id:
            effect += f" and routed to instance {linode_id}"
        return build_dry_run_response(
            "linode_ipv6_range_create",
            arguments.get("environment", ""),
            "POST",
            "/networking/ipv6/ranges",
            None,
            side_effects=[f"{effect}."],
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This creates an IPv6 range and changes networking configuration. Set "
            "confirm=true to proceed."
        )

    parsed_args = _parse_ipv6_range_create_args(arguments)
    if isinstance(parsed_args, list):
        return parsed_args
    prefix_length, linode_id, route_target = parsed_args

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.create_ipv6_range(
            prefix_length=prefix_length,
            linode_id=linode_id,
            route_target=route_target,
        )
        return serialize_api_response(
            {"message": "IPv6 range created", _IPV6_RANGE_KEY: raw},
            ip_pb2.IPv6RangeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create IPv6 range", _call)


def create_linode_ipv6_range_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_delete tool."""
    return Tool(
        name="linode_ipv6_range_delete",
        description=(
            "Deletes an IPv6 range. Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.IPv6RangeDeleteInput"),
    ), Capability.Destroy


async def _ipv6_range_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, ipv6_range: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_ipv6_range(ipv6_range)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ipv6_range(ipv6_range)
        return serialize_api_response(
            {
                "message": "IPv6 range deleted",
                _IPV6_RANGE_KEY: ipv6_range,
            },
            ip_pb2.IPv6RangeDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_ipv6_range_delete",
        method="DELETE",
        path=f"/networking/ipv6/ranges/{ipv6_range}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("IPv6Range"),
    )


async def handle_linode_ipv6_range_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_delete tool request."""
    range_value = arguments.get(_IPV6_RANGE_KEY, "")
    if not isinstance(range_value, str) or not range_value.strip():
        return error_response("range is required")
    ipv6_range = range_value.strip()
    if not valid_ipv6_prefix(ipv6_range):
        return error_response("range must be a valid IPv6 prefix")

    two_stage = await _ipv6_range_delete_two_stage(arguments, cfg, ipv6_range)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_ipv6_range(ipv6_range)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_ipv6_range_delete",
            "DELETE",
            f"/networking/ipv6/ranges/{ipv6_range}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This deletes an IPv6 range and changes networking configuration. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ipv6_range(ipv6_range)
        return serialize_api_response(
            {
                "message": "IPv6 range deleted",
                _IPV6_RANGE_KEY: ipv6_range,
            },
            ip_pb2.IPv6RangeDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete IPv6 range", _call)

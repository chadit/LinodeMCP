"""VPC WRITE tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_VPC_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the VPC (required)",
}

_SUBNET_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the subnet (required)",
}

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}

_IPV6_RANGE_KEY = "range"
_IPV6_RANGE_PROP: dict[str, Any] = {
    "type": "string",
    "description": (
        "The IPv6 range to delete, without prefix length (for example 2001:0db8::)"
    ),
}

_IPV6_PREFIX_LENGTH_KEY = "prefix_length"
_LINODE_ID_KEY = "linode_id"
_ROUTE_TARGET_KEY = "route_target"
_IPV6_PREFIX_LENGTH_PROP: dict[str, Any] = {
    "type": "integer",
    "enum": [56, 64],
    "description": "The prefix length of the IPv6 range. Must be 56 or 64.",
}
_LINODE_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "description": (
        "The ID of the Linode to assign this range to. Required when "
        "route_target is omitted."
    ),
}
_ROUTE_TARGET_PROP: dict[str, Any] = {
    "type": "string",
    "description": (
        "The IPv6 SLAAC address to assign this range to. Required when "
        "linode_id is omitted."
    ),
}


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
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the VPC (required)",
                },
                "region": {
                    "type": "string",
                    "description": "Region for the VPC (required)",
                },
                "description": {
                    "type": "string",
                    "description": "Description of the VPC",
                },
                "subnets": {
                    "type": "array",
                    "description": "Initial subnets: [{label, ipv4}]",
                    "items": {"type": "object"},
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm creation.",
                },
            },
            "required": ["label", "region", "confirm"],
        },
    ), Capability.Write


async def handle_linode_vpc_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    label = arguments.get("label", "")
    region = arguments.get("region", "")
    if not label:
        return error_response("label is required")
    if not region:
        return error_response("region is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_vpc(
            label=label,
            region=region,
            description=arguments.get("description"),
            subnets=arguments.get("subnets"),
        )

    return await execute_tool(cfg, arguments, "create VPC", _call)


def create_linode_vpc_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_update tool."""
    return Tool(
        name="linode_vpc_update",
        description="Updates an existing VPC",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "New label for the VPC",
                },
                "description": {
                    "type": "string",
                    "description": "New description for the VPC",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["vpc_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_vpc_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_vpc(
            vpc_id=vpc_id,
            label=arguments.get("label"),
            description=arguments.get("description"),
        )

    return await execute_tool(cfg, arguments, "update VPC", _call)


def create_linode_vpc_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_delete tool."""
    return Tool(
        name="linode_vpc_delete",
        description="Deletes a VPC. Pass dry_run=true to preview without deleting.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                        " Ignored when dry_run=true."
                    ),
                },
                **DRY_RUN_PROP,
            },
            "required": ["vpc_id", "confirm"],
        },
    ), Capability.Destroy


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

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_vpc(vpc_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vpc_delete",
            "DELETE",
            f"/vpcs/{vpc_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("This is destructive. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc(vpc_id)
        return {
            "message": f"VPC {vpc_id} deleted",
            "vpc_id": vpc_id,
        }

    return await execute_tool(cfg, arguments, "delete VPC", _call)


def create_linode_vpc_subnet_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_create tool."""
    return Tool(
        name="linode_vpc_subnet_create",
        description="Creates a new subnet in a VPC",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the subnet (required)",
                },
                "ipv4": {
                    "type": "string",
                    "description": (
                        "IPv4 range in CIDR format, e.g. 10.0.0.0/24 (required)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm creation.",
                },
            },
            "required": [
                "vpc_id",
                "label",
                "ipv4",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_vpc_subnet_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    label = arguments.get("label", "")
    ipv4 = arguments.get("ipv4", "")
    if not label:
        return error_response("label is required")
    if not ipv4:
        return error_response("ipv4 is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_vpc_subnet(
            vpc_id=vpc_id,
            label=label,
            ipv4=ipv4,
        )

    return await execute_tool(cfg, arguments, "create VPC subnet", _call)


def create_linode_vpc_subnet_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_update tool."""
    return Tool(
        name="linode_vpc_subnet_update",
        description="Updates a VPC subnet",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "subnet_id": _SUBNET_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "New label for the subnet (required)",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "vpc_id",
                "subnet_id",
                "label",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_vpc_subnet_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    ids = _parse_vpc_subnet_ids(arguments)
    if isinstance(ids, list):
        return ids
    vpc_id, subnet_id = ids

    label = arguments.get("label", "")
    if not label:
        return error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_vpc_subnet(
            vpc_id=vpc_id,
            subnet_id=subnet_id,
            label=label,
        )

    return await execute_tool(cfg, arguments, "update VPC subnet", _call)


def create_linode_vpc_subnet_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_delete tool."""
    return Tool(
        name="linode_vpc_subnet_delete",
        description=(
            "Deletes a VPC subnet. Pass dry_run=true to preview without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "subnet_id": _SUBNET_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                        " Ignored when dry_run=true."
                    ),
                },
                **DRY_RUN_PROP,
            },
            "required": [
                "vpc_id",
                "subnet_id",
                "confirm",
            ],
        },
    ), Capability.Destroy


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

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_vpc_subnet(vpc_id, subnet_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_vpc_subnet_delete",
            "DELETE",
            f"/vpcs/{vpc_id}/subnets/{subnet_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("This is destructive. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc_subnet(vpc_id, subnet_id)
        return {
            "message": f"Subnet {subnet_id} deleted from VPC {vpc_id}",
            "vpc_id": vpc_id,
            "subnet_id": subnet_id,
        }

    return await execute_tool(cfg, arguments, "delete VPC subnet", _call)


def create_linode_ipv6_range_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_create tool."""
    return Tool(
        name="linode_ipv6_range_create",
        description="Creates an IPv6 range",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                _IPV6_PREFIX_LENGTH_KEY: _IPV6_PREFIX_LENGTH_PROP,
                _LINODE_ID_KEY: _LINODE_ID_PROP,
                _ROUTE_TARGET_KEY: _ROUTE_TARGET_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm creation.",
                },
            },
            "required": [_IPV6_PREFIX_LENGTH_KEY, "confirm"],
        },
    ), Capability.Write


async def handle_linode_ipv6_range_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    parsed_args = _parse_ipv6_range_create_args(arguments)
    if isinstance(parsed_args, list):
        return parsed_args
    prefix_length, linode_id, route_target = parsed_args

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_ipv6_range(
            prefix_length=prefix_length,
            linode_id=linode_id,
            route_target=route_target,
        )

    return await execute_tool(cfg, arguments, "create IPv6 range", _call)


def create_linode_ipv6_range_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_delete tool."""
    return Tool(
        name="linode_ipv6_range_delete",
        description="Deletes an IPv6 range",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                _IPV6_RANGE_KEY: _IPV6_RANGE_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": [_IPV6_RANGE_KEY, "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_ipv6_range_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("This is destructive. Set confirm=true to proceed.")

    range_value = arguments.get(_IPV6_RANGE_KEY, "")
    if not isinstance(range_value, str) or not range_value.strip():
        return error_response("range is required")
    ipv6_range = range_value.strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ipv6_range(ipv6_range)
        return {
            "message": f"IPv6 range {ipv6_range} deleted",
            _IPV6_RANGE_KEY: ipv6_range,
        }

    return await execute_tool(cfg, arguments, "delete IPv6 range", _call)

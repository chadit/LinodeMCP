"""VPC READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

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

_IPV6_RANGE_KEY = "range"
_IPV6_RANGE_PROP: dict[str, Any] = {
    "type": "string",
    "description": (
        "The IPv6 range to access, without prefix length (for example 2001:0db8::)"
    ),
}


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_vpc_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_list tool."""
    return Tool(
        name="linode_vpc_list",
        description="Lists all VPCs on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_vpc_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vpcs = await client.list_vpcs()
        return {"count": len(vpcs), "vpcs": vpcs}

    return await execute_tool(cfg, arguments, "list VPCs", _call)


def create_linode_vpc_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_get tool."""
    return Tool(
        name="linode_vpc_get",
        description="Gets details of a specific VPC by ID",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
            },
            "required": ["vpc_id"],
        },
    ), Capability.Read


async def handle_linode_vpc_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_get tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_vpc(vpc_id)

    return await execute_tool(cfg, arguments, "get VPC", _call)


def create_linode_ipv6_range_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_get tool."""
    return Tool(
        name="linode_ipv6_range_get",
        description="Gets details of an IPv6 range",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                _IPV6_RANGE_KEY: _IPV6_RANGE_PROP,
            },
            "required": [_IPV6_RANGE_KEY],
        },
    ), Capability.Read


async def handle_linode_ipv6_range_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_get tool request."""
    range_value = arguments.get(_IPV6_RANGE_KEY, "")
    if not isinstance(range_value, str) or not range_value.strip():
        return error_response("range is required")
    ipv6_range = range_value.strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_ipv6_range(ipv6_range)

    return await execute_tool(cfg, arguments, "get IPv6 range", _call)


def create_linode_ipv6_range_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_list tool."""
    return Tool(
        name="linode_ipv6_range_list",
        description="Lists all IPv6 ranges on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_ipv6_range_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        response = await client.list_ipv6_ranges(page=page, page_size=page_size)
        ranges: list[dict[str, Any]] = response.get("data", [])
        return {"count": len(ranges), "ipv6_ranges": ranges}

    return await execute_tool(cfg, arguments, "list IPv6 ranges", _call)


def create_linode_ipv6_pool_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_pool_list tool."""
    return Tool(
        name="linode_ipv6_pool_list",
        description="Lists all IPv6 pools on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_ipv6_pool_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_pool_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        response = await client.list_ipv6_pools(page=page, page_size=page_size)
        pools: list[dict[str, Any]] = response.get("data", [])
        return {"count": len(pools), "ipv6_pools": pools}

    return await execute_tool(cfg, arguments, "list IPv6 pools", _call)


def create_linode_vpc_ip_all_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_ip_all_list tool."""
    return Tool(
        name="linode_vpc_ip_all_list",
        description="Lists all VPC IP addresses across all VPCs",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_vpc_ip_all_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_ip_all_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ips = await client.list_vpc_ips()
        return {"count": len(ips), "ips": ips}

    return await execute_tool(cfg, arguments, "list VPC IPs", _call)


def create_linode_vpc_ip_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_ip_list tool."""
    return Tool(
        name="linode_vpc_ip_list",
        description="Lists IP addresses for a specific VPC",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
            },
            "required": ["vpc_id"],
        },
    ), Capability.Read


async def handle_linode_vpc_ip_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_ip_list tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ips = await client.list_vpc_ip(vpc_id)
        return {"count": len(ips), "ips": ips}

    return await execute_tool(cfg, arguments, "list VPC IPs", _call)


def create_linode_vpc_subnet_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_list tool."""
    return Tool(
        name="linode_vpc_subnet_list",
        description="Lists subnets for a specific VPC",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
            },
            "required": ["vpc_id"],
        },
    ), Capability.Read


async def handle_linode_vpc_subnet_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_list tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        subnets = await client.list_vpc_subnets(vpc_id)
        return {"count": len(subnets), "subnets": subnets}

    return await execute_tool(cfg, arguments, "list VPC subnets", _call)


def create_linode_vpc_subnet_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_get tool."""
    return Tool(
        name="linode_vpc_subnet_get",
        description="Gets details of a specific VPC subnet",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "subnet_id": _SUBNET_ID_PROP,
            },
            "required": ["vpc_id", "subnet_id"],
        },
    ), Capability.Read


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


async def handle_linode_vpc_subnet_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_get tool request."""
    ids = _parse_vpc_subnet_ids(arguments)
    if isinstance(ids, list):
        return ids
    vpc_id, subnet_id = ids

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_vpc_subnet(vpc_id, subnet_id)

    return await execute_tool(cfg, arguments, "get VPC subnet", _call)

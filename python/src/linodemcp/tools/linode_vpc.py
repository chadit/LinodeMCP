"""VPC READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import ip_pb2, vpc_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    error_response,
    execute_tool,
    pagination_int_argument,
    valid_ipv6_prefix,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_SUBNET_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the subnet (required)",
}

_IPV6_RANGE_KEY = "range"


def create_linode_vpc_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_list tool."""
    return Tool(
        name="linode_vpc_list",
        description="Lists all VPCs. Can filter by label or region.",
        inputSchema=schema("linode.mcp.v1.VpcListInput"),
    ), Capability.Read


async def handle_linode_vpc_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_list tool request.

    Label is a case-insensitive substring match; region is a case-insensitive
    exact match, mirroring the Go list tool's containsFilter/fieldFilter.
    """
    label_filter = arguments.get("label", "")
    region_filter = arguments.get("region", "")

    def _matches(vpc: dict[str, Any]) -> bool:
        label = str(vpc.get("label", ""))
        if label_filter and label_filter.lower() not in label.lower():
            return False
        region = str(vpc.get("region", ""))
        return not (region_filter and region.lower() != region_filter.lower())

    applied: list[str] = []
    if label_filter:
        applied.append(f"label={label_filter}")
    if region_filter:
        applied.append(f"region={region_filter}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/vpcs")
        return serialize_list_response(
            raw,
            "vpcs",
            vpc_pb2.VpcListResponse(),
            filter_value=", ".join(applied) if applied else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "list VPCs", _call)


def _vpc_subnet_linode_interface_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw VPC subnet-linode interface to proto-canonical form."""
    return {
        "id": raw.get("id", 0),
        "active": raw.get("active", False),
        "config_id": raw.get("config_id", 0),
    }


def _vpc_subnet_linode_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw VPC subnet-linode to proto-canonical form."""
    return {
        "id": raw.get("id", 0),
        "interfaces": [
            _vpc_subnet_linode_interface_to_dict(iface)
            for iface in raw.get("interfaces", [])
        ],
    }


def vpc_subnet_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw VPC subnet to proto-canonical form."""
    return {
        "id": raw.get("id", 0),
        "label": raw.get("label", ""),
        "ipv4": raw.get("ipv4", ""),
        "linodes": [
            _vpc_subnet_linode_to_dict(linode) for linode in raw.get("linodes", [])
        ],
        "created": raw.get("created", ""),
        "updated": raw.get("updated", ""),
    }


def vpc_to_response_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw VPC API dict to proto-canonical linode.mcp.v1.Vpc form.

    Selects only the proto-modeled fields and always emits the repeated fields
    (subnets, linodes, interfaces) as lists, matching Go's protojson output.
    """
    return {
        "id": raw.get("id", 0),
        "label": raw.get("label", ""),
        "description": raw.get("description", ""),
        "region": raw.get("region", ""),
        "subnets": [vpc_subnet_to_dict(subnet) for subnet in raw.get("subnets", [])],
        "created": raw.get("created", ""),
        "updated": raw.get("updated", ""),
    }


def create_linode_vpc_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_get tool."""
    return Tool(
        name="linode_vpc_get",
        description="Gets details of a specific VPC by ID",
        inputSchema=schema("linode.mcp.v1.VpcGetInput"),
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
        return serialize_api_response(await client.get_vpc(vpc_id), vpc_pb2.Vpc())

    return await execute_tool(cfg, arguments, "get VPC", _call)


def create_linode_ipv6_range_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_get tool."""
    return Tool(
        name="linode_ipv6_range_get",
        description="Gets details of an IPv6 range",
        inputSchema=schema("linode.mcp.v1.IPv6RangeGetInput"),
    ), Capability.Read


async def handle_linode_ipv6_range_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_get tool request."""
    range_value = arguments.get(_IPV6_RANGE_KEY, "")
    if not isinstance(range_value, str) or not range_value.strip():
        return error_response("range is required")
    ipv6_range = range_value.strip()
    if not valid_ipv6_prefix(ipv6_range):
        return error_response("range must be a valid IPv6 prefix")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_ipv6_range(ipv6_range), ip_pb2.IPv6Range()
        )

    return await execute_tool(cfg, arguments, "get IPv6 range", _call)


def create_linode_ipv6_range_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_range_list tool."""
    return Tool(
        name="linode_ipv6_range_list",
        description="Lists all IPv6 ranges on the account",
        inputSchema=schema("linode.mcp.v1.IPv6RangeListInput"),
    ), Capability.Read


async def handle_linode_ipv6_range_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_range_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_ipv6_ranges(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "ipv6_ranges",
            ip_pb2.IPv6RangeListResponse(),
        )

    return await execute_tool(cfg, arguments, "list IPv6 ranges", _call)


def create_linode_ipv6_pool_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv6_pool_list tool."""
    return Tool(
        name="linode_ipv6_pool_list",
        description="Lists all IPv6 pools on the account",
        inputSchema=schema("linode.mcp.v1.IPv6PoolListInput"),
    ), Capability.Read


async def handle_linode_ipv6_pool_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv6_pool_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_ipv6_pools(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "ipv6_pools",
            ip_pb2.IPv6PoolListResponse(),
        )

    return await execute_tool(cfg, arguments, "list IPv6 pools", _call)


def create_linode_vpc_ip_all_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_ip_all_list tool."""
    return Tool(
        name="linode_vpc_ip_all_list",
        description="Lists all VPC IP addresses across all VPCs",
        inputSchema=schema("linode.mcp.v1.VPCIPAllListInput"),
    ), Capability.Read


async def handle_linode_vpc_ip_all_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_ip_all_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ips = await client.list_vpc_ips()
        return serialize_list_response(
            {"data": ips}, "ips", vpc_pb2.VPCIPListResponse()
        )

    return await execute_tool(cfg, arguments, "list VPC IPs", _call)


def create_linode_vpc_ip_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_ip_list tool."""
    return Tool(
        name="linode_vpc_ip_list",
        description="Lists IP addresses for a specific VPC",
        inputSchema=schema("linode.mcp.v1.VPCIPListInput"),
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
        return serialize_list_response(
            {"data": ips}, "ips", vpc_pb2.VPCIPListResponse()
        )

    return await execute_tool(cfg, arguments, "list VPC IPs", _call)


def create_linode_vpc_subnet_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_list tool."""
    return Tool(
        name="linode_vpc_subnet_list",
        description="Lists subnets for a specific VPC",
        inputSchema=schema("linode.mcp.v1.VpcSubnetListInput"),
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
        raw = await client.get_raw(f"/vpcs/{vpc_id}/subnets")
        return serialize_list_response(
            raw,
            "subnets",
            vpc_pb2.VpcSubnetListResponse(),
        )

    return await execute_tool(cfg, arguments, "list VPC subnets", _call)


def create_linode_vpc_subnet_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_vpc_subnet_get tool."""
    return Tool(
        name="linode_vpc_subnet_get",
        description="Gets details of a specific VPC subnet",
        inputSchema=schema("linode.mcp.v1.VpcSubnetGetInput"),
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
        return serialize_api_response(
            await client.get_vpc_subnet(vpc_id, subnet_id), vpc_pb2.VpcSubnet()
        )

    return await execute_tool(cfg, arguments, "get VPC subnet", _call)

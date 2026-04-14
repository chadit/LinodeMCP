"""VPC READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import _error_response, execute_tool

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


def create_linode_vpcs_list_tool() -> Tool:
    """Create the linode_vpcs_list tool."""
    return Tool(
        name="linode_vpcs_list",
        description="Lists all VPCs on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_vpcs_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpcs_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vpcs = await client.list_vpcs()
        return {"count": len(vpcs), "vpcs": vpcs}

    return await execute_tool(cfg, arguments, "list VPCs", _call)


def create_linode_vpc_get_tool() -> Tool:
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
    )


async def handle_linode_vpc_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_get tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_vpc(vpc_id)

    return await execute_tool(cfg, arguments, "get VPC", _call)


def create_linode_vpc_ips_list_tool() -> Tool:
    """Create the linode_vpc_ips_list tool."""
    return Tool(
        name="linode_vpc_ips_list",
        description="Lists all VPC IP addresses across all VPCs",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_vpc_ips_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_ips_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ips = await client.list_vpc_ips()
        return {"count": len(ips), "ips": ips}

    return await execute_tool(cfg, arguments, "list VPC IPs", _call)


def create_linode_vpc_ip_list_tool() -> Tool:
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
    )


async def handle_linode_vpc_ip_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_ip_list tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ips = await client.list_vpc_ip(vpc_id)
        return {"count": len(ips), "ips": ips}

    return await execute_tool(cfg, arguments, "list VPC IPs", _call)


def create_linode_vpc_subnets_list_tool() -> Tool:
    """Create the linode_vpc_subnets_list tool."""
    return Tool(
        name="linode_vpc_subnets_list",
        description="Lists subnets for a specific VPC",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
            },
            "required": ["vpc_id"],
        },
    )


async def handle_linode_vpc_subnets_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnets_list tool request."""
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        subnets = await client.list_vpc_subnets(vpc_id)
        return {"count": len(subnets), "subnets": subnets}

    return await execute_tool(cfg, arguments, "list VPC subnets", _call)


def create_linode_vpc_subnet_get_tool() -> Tool:
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
    )


def _parse_vpc_subnet_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse and validate vpc_id and subnet_id from arguments.

    Returns a (vpc_id, subnet_id) tuple on success, or an error
    response list on failure.
    """
    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    subnet_id_str = arguments.get("subnet_id", "")
    if not subnet_id_str:
        return _error_response("subnet_id is required")
    try:
        subnet_id = int(subnet_id_str)
    except ValueError:
        return _error_response("subnet_id must be a valid integer")

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

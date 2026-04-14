"""VPC WRITE tools for LinodeMCP."""

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

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
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


def create_linode_vpc_create_tool() -> Tool:
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
    )


async def handle_linode_vpc_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    label = arguments.get("label", "")
    region = arguments.get("region", "")
    if not label:
        return _error_response("label is required")
    if not region:
        return _error_response("region is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_vpc(
            label=label,
            region=region,
            description=arguments.get("description"),
            subnets=arguments.get("subnets"),
        )

    return await execute_tool(cfg, arguments, "create VPC", _call)


def create_linode_vpc_update_tool() -> Tool:
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
    )


async def handle_linode_vpc_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_vpc(
            vpc_id=vpc_id,
            label=arguments.get("label"),
            description=arguments.get("description"),
        )

    return await execute_tool(cfg, arguments, "update VPC", _call)


def create_linode_vpc_delete_tool() -> Tool:
    """Create the linode_vpc_delete tool."""
    return Tool(
        name="linode_vpc_delete",
        description="Deletes a VPC",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "vpc_id": _VPC_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                    "Must be true to confirm deletion. This is irreversible."
                ),
                },
            },
            "required": ["vpc_id", "confirm"],
        },
    )


async def handle_linode_vpc_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc(vpc_id)
        return {
            "message": f"VPC {vpc_id} deleted",
            "vpc_id": vpc_id,
        }

    return await execute_tool(cfg, arguments, "delete VPC", _call)


def create_linode_vpc_subnet_create_tool() -> Tool:
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
    )


async def handle_linode_vpc_subnet_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    vpc_id_str = arguments.get("vpc_id", "")
    if not vpc_id_str:
        return _error_response("vpc_id is required")
    try:
        vpc_id = int(vpc_id_str)
    except ValueError:
        return _error_response("vpc_id must be a valid integer")

    label = arguments.get("label", "")
    ipv4 = arguments.get("ipv4", "")
    if not label:
        return _error_response("label is required")
    if not ipv4:
        return _error_response("ipv4 is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_vpc_subnet(
            vpc_id=vpc_id,
            label=label,
            ipv4=ipv4,
        )

    return await execute_tool(cfg, arguments, "create VPC subnet", _call)


def create_linode_vpc_subnet_update_tool() -> Tool:
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
    )


async def handle_linode_vpc_subnet_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    ids = _parse_vpc_subnet_ids(arguments)
    if isinstance(ids, list):
        return ids
    vpc_id, subnet_id = ids

    label = arguments.get("label", "")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_vpc_subnet(
            vpc_id=vpc_id,
            subnet_id=subnet_id,
            label=label,
        )

    return await execute_tool(cfg, arguments, "update VPC subnet", _call)


def create_linode_vpc_subnet_delete_tool() -> Tool:
    """Create the linode_vpc_subnet_delete tool."""
    return Tool(
        name="linode_vpc_subnet_delete",
        description="Deletes a VPC subnet",
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
                ),
                },
            },
            "required": [
                "vpc_id",
                "subnet_id",
                "confirm",
            ],
        },
    )


async def handle_linode_vpc_subnet_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vpc_subnet_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    ids = _parse_vpc_subnet_ids(arguments)
    if isinstance(ids, list):
        return ids
    vpc_id, subnet_id = ids

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vpc_subnet(vpc_id, subnet_id)
        return {
            "message": f"Subnet {subnet_id} deleted from VPC {vpc_id}",
            "vpc_id": vpc_id,
            "subnet_id": subnet_id,
        }

    return await execute_tool(cfg, arguments, "delete VPC subnet", _call)

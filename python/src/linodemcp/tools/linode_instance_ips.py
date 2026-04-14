from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_INSTANCE_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the Linode instance (required)",
}

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}


def _parse_instance_id(
    arguments: dict[str, Any],
) -> int | list[TextContent]:
    """Parse and validate instance_id from arguments."""
    raw = arguments.get("instance_id", "")
    if not raw:
        return _error_response("instance_id is required")
    try:
        return int(raw)
    except (ValueError, TypeError):
        return _error_response("instance_id must be a valid integer")


def create_linode_instance_ips_list_tool() -> Tool:
    """Create the linode_instance_ips_list tool."""
    return Tool(
        name="linode_instance_ips_list",
        description=("Lists IP addresses for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_ips_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ips_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.list_instance_ips(iid)

    return await execute_tool(cfg, arguments, "list instance IPs", _call)


def create_linode_instance_ip_get_tool() -> Tool:
    """Create the linode_instance_ip_get tool."""
    return Tool(
        name="linode_instance_ip_get",
        description=("Gets details of a specific IP for an instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "address": {
                    "type": "string",
                    "description": ("The IP address to look up (required)"),
                },
            },
            "required": ["instance_id", "address"],
        },
    )


async def handle_linode_instance_ip_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_get tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    address = arguments.get("address", "")
    if not address:
        return _error_response("address is required")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.get_instance_ip(iid, address)

    return await execute_tool(cfg, arguments, "get instance IP", _call)


def create_linode_instance_ip_allocate_tool() -> Tool:
    """Create the linode_instance_ip_allocate tool."""
    return Tool(
        name="linode_instance_ip_allocate",
        description=("Allocates a new IP address for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "type": {
                    "type": "string",
                    "description": ("IP type: ipv4 or ipv6 (required)"),
                },
                "public": {
                    "type": "boolean",
                    "description": ("Whether the IP is public (default true)"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "type",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_ip_allocate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_allocate tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    ip_type = arguments.get("type", "")
    if not ip_type:
        return _error_response("type is required")

    public = arguments.get("public", True)

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.allocate_instance_ip(iid, ip_type=ip_type, public=public)

    return await execute_tool(cfg, arguments, "allocate instance IP", _call)


def create_linode_instance_ip_delete_tool() -> Tool:
    """Create the linode_instance_ip_delete tool."""
    return Tool(
        name="linode_instance_ip_delete",
        description=("Deletes an IP address from a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "address": {
                    "type": "string",
                    "description": ("The IP address to delete (required)"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": [
                "instance_id",
                "address",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_ip_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    address = arguments.get("address", "")
    if not address:
        return _error_response("address is required")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.delete_instance_ip(iid, address)
        return {
            "message": (f"IP {address} deleted from instance {iid}"),
            "instance_id": iid,
            "address": address,
        }

    return await execute_tool(cfg, arguments, "delete instance IP", _call)

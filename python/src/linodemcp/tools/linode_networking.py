"""Networking tools for LinodeMCP."""

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


def create_linode_vlans_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_vlans_list tool."""
    return Tool(
        name="linode_vlans_list",
        description="Lists all VLANs on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_vlans_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vlans_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        vlans = await client.list_vlans()
        return {"count": len(vlans), "vlans": vlans}

    return await execute_tool(cfg, arguments, "list VLANs", _call)


def create_linode_vlan_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_vlan_delete tool."""
    return Tool(
        name="linode_vlan_delete",
        description="Deletes a VLAN",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "region_id": {
                    "type": "string",
                    "description": "Region ID where the VLAN exists (required)",
                },
                "label": {
                    "type": "string",
                    "description": "VLAN label to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["region_id", "label", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_vlan_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_vlan_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("This is destructive. Set confirm=true to proceed.")

    region_id = arguments.get("region_id", "")
    label = arguments.get("label", "")

    if not region_id:
        return error_response("region_id is required")
    if not label:
        return error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_vlan(region_id, label)
        return {
            "message": f"VLAN {label} in region {region_id} deleted successfully",
            "region_id": region_id,
            "label": label,
        }

    return await execute_tool(cfg, arguments, "delete VLAN", _call)


def create_linode_ipv4_share_tool() -> tuple[Tool, Capability]:
    """Create the linode_ipv4_share tool."""
    return Tool(
        name="linode_ipv4_share",
        description="Shares IPv4 addresses with a Linode",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "ips": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of IPv4 addresses to share (required)",
                },
                "linode_id": {
                    "type": "integer",
                    "description": "Linode ID to share the IPs with (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm sharing IPs.",
                },
            },
            "required": ["ips", "linode_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_ipv4_share(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_ipv4_share tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This modifies network state. Set confirm=true to proceed."
        )

    ips = arguments.get("ips")
    linode_id = arguments.get("linode_id")

    if not ips or not isinstance(ips, list) or len(ips) == 0:
        return error_response("ips must be a non-empty list of IPv4 addresses")
    if linode_id is None:
        return error_response("linode_id is required")
    if not isinstance(linode_id, int):
        return error_response("linode_id must be an integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.share_ipv4s(ips, linode_id)
        return {
            "message": f"IPv4 addresses shared with Linode {linode_id}",
            "linode_id": linode_id,
            "ips": ips,
            "result": result,
        }

    return await execute_tool(cfg, arguments, "share IPv4 addresses", _call)

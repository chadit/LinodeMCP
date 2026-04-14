from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_firewall_create_tool() -> Tool:
    """Create the linode_firewall_create tool."""
    return Tool(
        name="linode_firewall_create",
        description=(
            "Creates a new Cloud Firewall. The firewall is created with no rules."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {
                    "type": "string",
                    "description": "A label for the firewall (required)",
                },
                "inbound_policy": {
                    "type": "string",
                    "description": (
                        "Default inbound policy: 'ACCEPT' or 'DROP' (default: 'ACCEPT')"
                    ),
                },
                "outbound_policy": {
                    "type": "string",
                    "description": (
                        "Default outbound policy: 'ACCEPT' or 'DROP' "
                        "(default: 'ACCEPT')"
                    ),
                },
            },
            "required": ["label"],
        },
    )


async def handle_linode_firewall_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_create tool request."""
    label = arguments.get("label", "")
    inbound_policy = arguments.get("inbound_policy", "ACCEPT")
    outbound_policy = arguments.get("outbound_policy", "ACCEPT")

    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.create_firewall(
            label=label,
            inbound_policy=inbound_policy,
            outbound_policy=outbound_policy,
        )
        return {
            "message": (
                f"Firewall '{firewall.label}' (ID: {firewall.id}) created successfully"
            ),
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "created": firewall.created,
            },
        }

    return await execute_tool(cfg, arguments, "create firewall", _call)


def create_linode_firewall_update_tool() -> Tool:
    """Create the linode_firewall_update tool."""
    return Tool(
        name="linode_firewall_update",
        description="Updates an existing Cloud Firewall.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New label for the firewall (optional)",
                },
                "status": {
                    "type": "string",
                    "description": "New status: 'enabled' or 'disabled' (optional)",
                },
                "inbound_policy": {
                    "type": "string",
                    "description": "New inbound policy: 'ACCEPT' or 'DROP' (optional)",
                },
                "outbound_policy": {
                    "type": "string",
                    "description": "New outbound policy: 'ACCEPT' or 'DROP' (optional)",
                },
            },
            "required": ["firewall_id"],
        },
    )


async def handle_linode_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_update tool request."""
    firewall_id = arguments.get("firewall_id", 0)

    if not firewall_id:
        return _error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.update_firewall(
            firewall_id=int(firewall_id),
            label=arguments.get("label"),
            status=arguments.get("status"),
            inbound_policy=arguments.get("inbound_policy"),
            outbound_policy=arguments.get("outbound_policy"),
        )
        return {
            "message": f"Firewall {firewall_id} updated successfully",
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "updated": firewall.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update firewall", _call)


def create_linode_firewall_delete_tool() -> Tool:
    """Create the linode_firewall_delete tool."""
    return Tool(
        name="linode_firewall_delete",
        description=(
            "Deletes a Cloud Firewall. WARNING: This removes all rules "
            "and unassigns all devices."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["firewall_id", "confirm"],
        },
    )


async def handle_linode_firewall_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_delete tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not firewall_id:
        return _error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall(int(firewall_id))
        return {
            "message": f"Firewall {firewall_id} deleted successfully",
            "firewall_id": firewall_id,
        }

    return await execute_tool(cfg, arguments, "delete firewall", _call)

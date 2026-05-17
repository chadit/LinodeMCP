from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_sshkey_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_create tool."""
    return Tool(
        name="linode_sshkey_create",
        description="Creates a new SSH key and adds it to your Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "A label for the SSH key (required)",
                },
                "ssh_key": {
                    "type": "string",
                    "description": "The public SSH key (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
            },
            "required": ["label", "ssh_key", "confirm"],
        },
    ), Capability.Write


async def handle_linode_sshkey_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_create tool request."""
    if not arguments.get("confirm"):
        return error_response(
            "This creates an SSH key on your profile. Set confirm=true to proceed."
        )

    label = arguments.get("label", "")
    ssh_key = arguments.get("ssh_key", "")

    if not label:
        return error_response("label is required")
    if not ssh_key:
        return error_response("ssh_key is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.create_ssh_key(label, ssh_key)
        return {
            "message": f"SSH key '{key.label}' (ID: {key.id}) created successfully",
            "ssh_key": {
                "id": key.id,
                "label": key.label,
                "created": key.created,
            },
        }

    return await execute_tool(cfg, arguments, "create SSH key", _call)


def create_linode_sshkey_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_delete tool."""
    return Tool(
        name="linode_sshkey_delete",
        description="Deletes an SSH key from your Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "ssh_key_id": {
                    "type": "integer",
                    "description": "The ID of the SSH key to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
            },
            "required": ["ssh_key_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_sshkey_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_delete tool request."""
    if not arguments.get("confirm"):
        return error_response("This deletes an SSH key. Set confirm=true to proceed.")

    ssh_key_id = arguments.get("ssh_key_id", 0)

    if not ssh_key_id:
        return error_response("ssh_key_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ssh_key(int(ssh_key_id))
        return {
            "message": f"SSH key {ssh_key_id} deleted successfully",
            "ssh_key_id": ssh_key_id,
        }

    return await execute_tool(cfg, arguments, "delete SSH key", _call)

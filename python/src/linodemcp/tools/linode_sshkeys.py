from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    SSH_KEY_TRUNCATE_LIMIT,
    execute_tool,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient, SSHKey


def ssh_key_to_response_dict(key: SSHKey) -> dict[str, Any]:
    """Shape an SSH key dataclass to proto-canonical form."""
    return {
        "id": key.id,
        "label": key.label,
        "ssh_key": key.ssh_key,
        "created": key.created,
    }


def create_linode_sshkey_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_get tool."""
    return Tool(
        name="linode_sshkey_get",
        description="Gets one SSH key associated with your Linode profile.",
        inputSchema=schema("linode.mcp.v1.SSHKeyGetInput"),
    ), Capability.Read


async def handle_linode_sshkey_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_get tool request."""
    ssh_key_id = arguments.get("ssh_key_id", 0)

    if not ssh_key_id:
        return [TextContent(type="text", text="Error: ssh_key_id is required")]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.get_ssh_key(int(ssh_key_id))
        return ssh_key_to_response_dict(key)

    return await execute_tool(cfg, arguments, "retrieve SSH key", _call)


def create_linode_sshkey_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_list tool."""
    return Tool(
        name="linode_sshkey_list",
        description=(
            "Lists all SSH keys associated with your Linode profile. "
            "Can filter by label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter SSH keys by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_sshkey_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_list tool request."""
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        keys = await client.list_ssh_keys()

        if label_contains:
            keys = [k for k in keys if label_contains.lower() in k.label.lower()]

        keys_data = [
            {
                "id": k.id,
                "label": k.label,
                "ssh_key": truncate_string(k.ssh_key, SSH_KEY_TRUNCATE_LIMIT),
                "created": k.created,
            }
            for k in keys
        ]

        response: dict[str, Any] = {
            "count": len(keys),
            "ssh_keys": keys_data,
        }

        if label_contains:
            response["filter"] = f"label_contains={label_contains}"

        return response

    return await execute_tool(cfg, arguments, "retrieve SSH keys", _call)


def truncate_string(value: str, limit: int) -> str:
    """Truncate a string with ellipsis if it exceeds the limit."""
    if len(value) > limit:
        return value[:limit] + "..."
    return value

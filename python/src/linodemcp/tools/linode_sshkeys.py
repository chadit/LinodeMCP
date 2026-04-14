from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import (
    SSH_KEY_TRUNCATE_LIMIT,
    execute_tool,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_sshkeys_list_tool() -> Tool:
    """Create the linode_sshkeys_list tool."""
    return Tool(
        name="linode_sshkeys_list",
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
    )


async def handle_linode_sshkeys_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkeys_list tool request."""
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        keys = await client.list_ssh_keys()

        if label_contains:
            keys = [k for k in keys if label_contains.lower() in k.label.lower()]

        keys_data = [
            {
                "id": k.id,
                "label": k.label,
                "ssh_key": _truncate_string(k.ssh_key, SSH_KEY_TRUNCATE_LIMIT),
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


def _truncate_string(value: str, limit: int) -> str:
    """Truncate a string with ellipsis if it exceeds the limit."""
    if len(value) > limit:
        return value[:limit] + "..."
    return value

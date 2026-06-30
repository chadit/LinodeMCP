from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import sshkey_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


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
        raw = await client.get_raw(f"/profile/sshkeys/{int(ssh_key_id)}")
        return serialize_api_response(raw, sshkey_pb2.SSHKey())

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

    def _matches(key: dict[str, Any]) -> bool:
        label = str(key.get("label", ""))
        return not label_contains or label_contains.lower() in label.lower()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/profile/sshkeys")
        return serialize_list_response(
            raw,
            "ssh_keys",
            sshkey_pb2.SSHKeyListResponse(),
            filter_value=(
                f"label_contains={label_contains}" if label_contains else None
            ),
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve SSH keys", _call)

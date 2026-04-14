"""Linode account tool - authenticated user account information."""

from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
from linodemcp.tools.helpers import execute_tool


def create_linode_account_tool() -> Tool:
    """Create the linode_account tool."""
    return Tool(
        name="linode_account",
        description=(
            "Retrieves the authenticated user's Linode account information "
            "including billing details and capabilities"
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
            },
        },
    )


async def handle_linode_account(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account tool request.

    Args:
        arguments: EnvironmentArgs - environment (optional)
        cfg: Configuration object
    """

    async def _call(client: RetryableClient) -> dict[str, Any]:
        account = await client.get_account()
        return {
            "first_name": account.first_name,
            "last_name": account.last_name,
            "email": account.email,
            "company": account.company,
            "balance": account.balance,
            "balance_uninvoiced": account.balance_uninvoiced,
            "capabilities": account.capabilities,
            "active_since": account.active_since,
        }

    return await execute_tool(cfg, arguments, "retrieve Linode account", _call)

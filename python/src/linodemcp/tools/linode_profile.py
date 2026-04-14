"""Linode profile tool - user account profile information."""

from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
from linodemcp.tools.helpers import execute_tool


def create_linode_profile_tool() -> Tool:
    """Create the linode_profile tool."""
    return Tool(
        name="linode_profile",
        description="Retrieves Linode user account profile information",
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


async def handle_linode_profile(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile tool request.

    Args:
        arguments: EnvironmentArgs - environment (optional)
        cfg: Configuration object
    """

    async def _call(client: RetryableClient) -> dict[str, Any]:
        profile = await client.get_profile()
        return {
            "username": profile.username,
            "email": profile.email,
            "timezone": profile.timezone,
            "email_notifications": profile.email_notifications,
            "restricted": profile.restricted,
            "two_factor_auth": profile.two_factor_auth,
            "uid": profile.uid,
        }

    return await execute_tool(cfg, arguments, "retrieve Linode profile", _call)

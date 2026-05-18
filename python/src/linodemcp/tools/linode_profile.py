"""Linode profile tool - user account profile information."""

from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool


def create_linode_profile_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


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


def create_linode_profile_token_revoke_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_revoke tool."""
    return Tool(
        name="linode_profile_token_revoke",
        description="Revokes a Linode personal access token by token ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "token_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the personal access token to revoke",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
            },
            "required": ["token_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_profile_token_revoke(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_revoke tool request."""
    token_id = arguments.get("token_id")
    if isinstance(token_id, bool) or not isinstance(token_id, int) or token_id < 1:
        return error_response("token_id must be a positive integer")
    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes a profile token. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_profile_token(token_id)
        return {"message": f"Profile token {token_id} revoked successfully"}

    return await execute_tool(cfg, arguments, "revoke Linode profile token", _call)

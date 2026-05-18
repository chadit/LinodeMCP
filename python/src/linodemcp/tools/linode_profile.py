"""Linode profile tool - user account profile information."""

from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

PROFILE_TOKEN_LABEL_MAX_LENGTH = 100
PROFILE_TOKEN_SECRET_FIELDS = frozenset({"token", "access_token", "secret"})


def _redact_profile_token(token: dict[str, Any]) -> dict[str, Any]:
    """Drop secret token fields from profile token tool output."""
    return {
        key: value
        for key, value in token.items()
        if key.lower() not in PROFILE_TOKEN_SECRET_FIELDS
    }


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


def create_linode_profile_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_get tool."""
    return Tool(
        name="linode_profile_token_get",
        description="Retrieves a Linode personal access token by token ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "token_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the personal access token to retrieve",
                },
            },
            "required": ["token_id"],
        },
    ), Capability.Read


async def handle_linode_profile_token_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_get tool request."""
    token_id = arguments.get("token_id")
    if isinstance(token_id, bool) or not isinstance(token_id, int) or token_id < 1:
        return error_response("token_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.get_profile_token(token_id)
        return _redact_profile_token(token)

    return await execute_tool(cfg, arguments, "retrieve Linode profile token", _call)


def create_linode_profile_token_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_update tool."""
    return Tool(
        name="linode_profile_token_update",
        description="Updates a Linode personal access token label by token ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "token_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the personal access token to update",
                },
                "label": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": PROFILE_TOKEN_LABEL_MAX_LENGTH,
                    "description": "Display label for the personal access token",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this token update.",
                },
            },
            "required": ["token_id", "label", "confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_token_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_update tool request."""
    token_id = arguments.get("token_id")
    if isinstance(token_id, bool) or not isinstance(token_id, int) or token_id < 1:
        return error_response("token_id must be a positive integer")

    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return error_response("label must be a non-empty string")
    if len(label) > PROFILE_TOKEN_LABEL_MAX_LENGTH:
        return error_response("label must be 100 characters or fewer")
    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a profile token. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_profile_token(token_id, label=label)

    return await execute_tool(cfg, arguments, "update Linode profile token", _call)


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

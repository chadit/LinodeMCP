"""Linode profile tool - user account profile information."""

from typing import Any, cast

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient, build_profile_security_questions_body
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


def create_linode_profile_preferences_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_preferences_get tool."""
    return Tool(
        name="linode_profile_preferences_get",
        description="Gets OAuth client-specific Linode profile preferences.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


async def handle_linode_profile_preferences_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_preferences_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_profile_preferences()

    return await execute_tool(
        cfg, arguments, "retrieve Linode profile preferences", _call
    )


def create_linode_profile_preferences_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_preferences_update tool."""
    return Tool(
        name="linode_profile_preferences_update",
        description="Updates OAuth client-specific Linode profile preferences.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "preferences": {
                    "type": "object",
                    "description": (
                        "Preference key/value object to save for this OAuth client"
                    ),
                    "additionalProperties": True,
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this preferences update.",
                },
            },
            "required": ["preferences", "confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_preferences_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_preferences_update tool request."""
    preferences_arg = arguments.get("preferences")
    if not isinstance(preferences_arg, dict):
        return error_response("preferences must be an object")
    preferences = cast("dict[str, Any]", preferences_arg)
    if arguments.get("confirm") is not True:
        return error_response(
            "This updates profile preferences. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_profile_preferences(preferences)

    return await execute_tool(
        cfg, arguments, "update Linode profile preferences", _call
    )


def create_linode_profile_tfa_enable_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_tfa_enable tool."""
    return Tool(
        name="linode_profile_tfa_enable",
        description=(
            "Generates a two-factor authentication secret for the Linode profile."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to generate a profile TFA secret.",
                },
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_tfa_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tfa_enable tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This generates a profile two-factor authentication secret. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        secret = await client.create_profile_tfa_secret()
        return {
            **secret,
            "warning": (
                "IMPORTANT: Save this two-factor authentication secret now. "
                "It must be confirmed before two-factor authentication is "
                "enabled."
            ),
        }

    return await execute_tool(
        cfg, arguments, "generate profile two-factor authentication secret", _call
    )


def create_linode_profile_tfa_disable_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_tfa_disable tool."""
    return Tool(
        name="linode_profile_tfa_disable",
        description="Disables two-factor authentication for the Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to disable profile two-factor authentication."
                    ),
                },
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_tfa_disable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tfa_disable tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This disables profile two-factor authentication. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.disable_profile_tfa()

    return await execute_tool(
        cfg, arguments, "disable profile two-factor authentication", _call
    )


def create_linode_profile_tfa_enable_confirm_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_tfa_enable_confirm tool."""
    return Tool(
        name="linode_profile_tfa_enable_confirm",
        description=(
            "Confirms enabling two-factor authentication for the Linode profile."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tfa_code": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Two-factor authentication code to confirm setup",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this profile update.",
                },
            },
            "required": ["tfa_code", "confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_tfa_enable_confirm(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tfa_enable_confirm tool request."""
    tfa_code = arguments.get("tfa_code")
    if not isinstance(tfa_code, str) or not tfa_code.strip():
        return error_response("tfa_code must be a non-empty string")
    if arguments.get("confirm") is not True:
        return error_response(
            "This confirms profile two-factor authentication. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.confirm_profile_tfa_enable(tfa_code=tfa_code.strip())

    return await execute_tool(
        cfg, arguments, "confirm profile two-factor authentication", _call
    )


def create_linode_profile_phone_number_verify_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_phone_number_verify tool."""
    return Tool(
        name="linode_profile_phone_number_verify",
        description="Verifies the Linode profile phone number using an SMS code.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "otp_code": {
                    "type": "string",
                    "minLength": 1,
                    "description": "One-time code received by SMS",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to verify this profile phone number.",
                },
            },
            "required": ["otp_code", "confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_phone_number_verify(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_phone_number_verify tool request."""
    otp_code = arguments.get("otp_code")
    if not isinstance(otp_code, str) or not otp_code.strip():
        return error_response("otp_code must be a non-empty string")
    if arguments.get("confirm") is not True:
        return error_response(
            "This verifies a profile phone number. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.verify_profile_phone_number(otp_code.strip())

    return await execute_tool(
        cfg, arguments, "verify Linode profile phone number", _call
    )


def create_linode_profile_security_questions_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_security_questions_list tool."""
    return Tool(
        name="linode_profile_security_questions_list",
        description="Lists available Linode profile security questions.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


async def handle_linode_profile_security_questions_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_security_questions_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_profile_security_questions()

    return await execute_tool(
        cfg, arguments, "list Linode profile security questions", _call
    )


def create_linode_profile_security_questions_answer_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_security_questions_answer tool."""
    return Tool(
        name="linode_profile_security_questions_answer",
        description="Answers profile security questions for the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "security_questions": {
                    "type": "array",
                    "minItems": 3,
                    "maxItems": 3,
                    "items": {
                        "type": "object",
                        "properties": {
                            "question_id": {
                                "type": "integer",
                                "minimum": 1,
                                "description": "ID of the security question",
                            },
                            "response": {
                                "type": "string",
                                "minLength": 3,
                                "maxLength": 17,
                                "description": "Answer for the security question",
                            },
                        },
                        "required": ["question_id", "response"],
                    },
                    "description": "Security question answers to save",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to answer profile security questions.",
                },
            },
            "required": ["security_questions", "confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_security_questions_answer(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_security_questions_answer tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This answers profile security questions. Set confirm=true to proceed."
        )

    security_questions = arguments.get("security_questions")
    try:
        body = build_profile_security_questions_body(security_questions)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    body_questions = body["security_questions"]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.answer_profile_security_questions(body_questions)

    return await execute_tool(
        cfg, arguments, "answer profile security questions", _call
    )


def create_linode_profile_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_create tool."""
    return Tool(
        name="linode_profile_token_create",
        description="Creates a Linode personal access token.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "expiry": {
                    "type": ["string", "null"],
                    "description": (
                        "Expiration timestamp for the token. Omit to keep valid "
                        "until manually revoked."
                    ),
                },
                "label": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": PROFILE_TOKEN_LABEL_MAX_LENGTH,
                    "description": "Display label for the personal access token",
                },
                "scopes": {
                    "type": "string",
                    "description": "Space-separated access scopes for the token",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this token creation.",
                },
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_create tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a profile token. Set confirm=true to proceed."
        )

    label = arguments.get("label")
    if label is not None:
        if not isinstance(label, str) or not label.strip():
            return error_response("label must be a non-empty string")
        if len(label) > PROFILE_TOKEN_LABEL_MAX_LENGTH:
            return error_response("label must be 100 characters or fewer")

    scopes = arguments.get("scopes")
    if scopes is not None and (not isinstance(scopes, str) or not scopes.strip()):
        return error_response("scopes must be a non-empty string")

    expiry = arguments.get("expiry")
    if expiry is not None and not isinstance(expiry, str):
        return error_response("expiry must be a string or null")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.create_profile_token(
            expiry=expiry,
            label=label.strip() if isinstance(label, str) else None,
            scopes=scopes.strip() if isinstance(scopes, str) else None,
        )
        return {
            "warning": (
                "IMPORTANT: The token below is shown ONLY ONCE. "
                "Save it now - it cannot be retrieved later."
            ),
            "token": token,
        }

    return await execute_tool(cfg, arguments, "create Linode profile token", _call)


def create_linode_profile_tokens_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_tokens_list tool."""
    return Tool(
        name="linode_profile_tokens_list",
        description="Lists Linode personal access tokens.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


async def handle_linode_profile_tokens_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tokens_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        tokens = await client.list_profile_tokens()
        return {"tokens": [_redact_profile_token(token) for token in tokens]}

    return await execute_tool(cfg, arguments, "list Linode profile tokens", _call)


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

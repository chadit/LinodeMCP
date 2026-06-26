"""Linode profile tool - user account profile information."""

from typing import Any, cast

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient, build_profile_security_questions_body
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    PARAM_DRY_RUN,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

PROFILE_TOKEN_LABEL_MAX_LENGTH = 100
PROFILE_TOKEN_SECRET_FIELDS = frozenset({"token", "access_token", "secret"})


def _redact_profile_token(token: dict[str, Any]) -> dict[str, Any]:
    """Drop secret token fields from profile token tool output."""
    return {
        key: value
        for key, value in token.items()
        if key.lower() not in PROFILE_TOKEN_SECRET_FIELDS
    }


def create_linode_profile_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_get tool."""
    return Tool(
        name="linode_profile_get",
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


async def handle_linode_profile_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_get tool request.

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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["preferences", "confirm"],
        },
    ), Capability.Write


async def handle_linode_profile_preferences_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_preferences_update tool request."""
    if is_dry_run(arguments):
        if not isinstance(arguments.get("preferences"), dict):
            return error_response("preferences must be an object")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_profile_preferences()

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    "The OAuth client's profile preferences are replaced with "
                    "the supplied values."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_profile_preferences_update",
            "PUT",
            "/profile/preferences",
            _fetch,
            _walk,
        )

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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_tfa_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tfa_enable tool request."""
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_profile_tfa_enable",
            arguments.get("environment", ""),
            "POST",
            "/profile/tfa-enable",
            None,
            side_effects=[
                "A new two-factor authentication secret is generated; it must "
                "be confirmed before two-factor authentication becomes active."
            ],
        )

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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_tfa_disable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tfa_disable tool request."""
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_profile_tfa_disable",
            arguments.get("environment", ""),
            "POST",
            "/profile/tfa-disable",
            None,
            side_effects=["Two-factor authentication is disabled for this profile."],
            warnings=["Disabling two-factor authentication reduces account security."],
        )

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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["tfa_code", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_tfa_enable_confirm(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_tfa_enable_confirm tool request."""
    if is_dry_run(arguments):
        if (
            not isinstance(arguments.get("tfa_code"), str)
            or not str(arguments.get("tfa_code")).strip()
        ):
            return error_response("tfa_code must be a non-empty string")
        return build_dry_run_response(
            "linode_profile_tfa_enable_confirm",
            arguments.get("environment", ""),
            "POST",
            "/profile/tfa-enable-confirm",
            None,
            side_effects=["Two-factor authentication is enabled for this profile."],
        )

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


def create_linode_profile_phone_number_send_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_phone_number_send tool."""
    return Tool(
        name="linode_profile_phone_number_send",
        description="Sends a verification code to a Linode profile phone number.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "iso_code": {
                    "type": "string",
                    "minLength": 1,
                    "description": (
                        "ISO 3166-1 alpha-2 country code for the phone number"
                    ),
                },
                "phone_number": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Phone number to receive the verification code",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to send a verification code to this phone number."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["iso_code", "phone_number", "confirm"],
        },
    ), Capability.Admin


def _profile_required_id(
    arguments: dict[str, Any], name: str
) -> int | list[TextContent]:
    """Parse a required positive-integer id, or return an error response."""
    value = arguments.get(name)
    if isinstance(value, bool) or not isinstance(value, int) or value < 1:
        return error_response(f"{name} must be a positive integer")
    return value


def _token_label_error(label: object) -> list[TextContent] | None:
    """Validate a token label; return an error response or None."""
    if not isinstance(label, str) or not label.strip():
        return error_response("label must be a non-empty string")
    if len(label) > PROFILE_TOKEN_LABEL_MAX_LENGTH:
        return error_response("label must be 100 characters or fewer")
    return None


def _parse_phone_send(
    arguments: dict[str, Any],
) -> tuple[str, str] | list[TextContent]:
    """Parse iso_code and phone_number, or return an error response."""
    iso_code = arguments.get("iso_code")
    if not isinstance(iso_code, str) or not iso_code.strip():
        return error_response("iso_code must be a non-empty string")
    phone_number = arguments.get("phone_number")
    if not isinstance(phone_number, str) or not phone_number.strip():
        return error_response("phone_number must be a non-empty string")
    return iso_code.strip(), phone_number.strip()


async def handle_linode_profile_phone_number_send(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_phone_number_send tool request."""
    if is_dry_run(arguments):
        parsed = _parse_phone_send(arguments)
        if isinstance(parsed, list):
            return parsed
        return build_dry_run_response(
            "linode_profile_phone_number_send",
            arguments.get("environment", ""),
            "POST",
            "/profile/phone-number",
            None,
            side_effects=["A verification code is sent to the supplied phone number."],
        )

    parsed = _parse_phone_send(arguments)
    if isinstance(parsed, list):
        return parsed
    iso_code, phone_number = parsed
    if arguments.get("confirm") is not True:
        return error_response(
            "This sends a profile phone number verification code. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.send_profile_phone_number_verification(
            iso_code, phone_number
        )

    return await execute_tool(
        cfg, arguments, "send Linode profile phone number verification code", _call
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["otp_code", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_phone_number_verify(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_phone_number_verify tool request."""
    if is_dry_run(arguments):
        code = arguments.get("otp_code")
        if not isinstance(code, str) or not code.strip():
            return error_response("otp_code must be a non-empty string")
        return build_dry_run_response(
            "linode_profile_phone_number_verify",
            arguments.get("environment", ""),
            "POST",
            "/profile/phone-number/verify",
            None,
            side_effects=["The phone number is verified and added to the profile."],
        )

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


def create_linode_profile_phone_number_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_phone_number_delete tool."""
    return Tool(
        name="linode_profile_phone_number_delete",
        description="Deletes the verified Linode profile phone number.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to delete this profile phone number.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_phone_number_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_phone_number_delete tool request."""
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_profile_phone_number_delete",
            arguments.get("environment", ""),
            "DELETE",
            "/profile/phone-number",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a profile phone number. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.delete_profile_phone_number()

    return await execute_tool(
        cfg, arguments, "delete Linode profile phone number", _call
    )


def create_linode_profile_security_question_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_security_question_list tool."""
    return Tool(
        name="linode_profile_security_question_list",
        description="Lists available Linode profile security questions.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


async def handle_linode_profile_security_question_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_security_question_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_profile_security_questions()

    return await execute_tool(
        cfg, arguments, "list Linode profile security questions", _call
    )


def create_linode_profile_security_question_answer_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_security_question_answer tool."""
    return Tool(
        name="linode_profile_security_question_answer",
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["security_questions", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_security_question_answer(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_security_question_answer tool request."""
    if is_dry_run(arguments):
        try:
            build_profile_security_questions_body(arguments.get("security_questions"))
        except (TypeError, ValueError) as exc:
            return error_response(str(exc))
        return build_dry_run_response(
            "linode_profile_security_question_answer",
            arguments.get("environment", ""),
            "POST",
            "/profile/security-questions",
            None,
            side_effects=["The profile's security question answers are saved."],
        )

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
                    "type": "string",
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Admin


def _token_create_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate the optional token-create fields; return an error or None."""
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
    return None


async def handle_linode_profile_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_create tool request."""
    if is_dry_run(arguments):
        field_error = _token_create_error(arguments)
        if field_error is not None:
            return field_error
        label = arguments.get("label")
        effect = (
            f"A new personal access token {label!r} will be created."
            if label
            else "A new personal access token will be created."
        )
        return build_dry_run_response(
            "linode_profile_token_create",
            arguments.get("environment", ""),
            "POST",
            "/profile/tokens",
            None,
            side_effects=[effect],
            warnings=["The token secret is returned only once, at creation time."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a profile token. Set confirm=true to proceed."
        )

    field_error = _token_create_error(arguments)
    if field_error is not None:
        return field_error

    label = arguments.get("label")
    scopes = arguments.get("scopes")
    expiry = arguments.get("expiry")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.create_profile_token(
            expiry=expiry if isinstance(expiry, str) else None,
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


def create_linode_profile_token_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_list tool."""
    return Tool(
        name="linode_profile_token_list",
        description="Lists Linode personal access tokens.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_profile_token_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        tokens = await client.list_profile_tokens(page=page, page_size=page_size)
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


def create_linode_profile_login_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_login_list tool."""
    return Tool(
        name="linode_profile_login_list",
        description="Lists recent successful Linode profile logins.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_profile_login_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_login_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return {
            "logins": await client.list_profile_logins(page=page, page_size=page_size)
        }

    return await execute_tool(cfg, arguments, "list Linode profile logins", _call)


def create_linode_profile_device_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_device_list tool."""
    return Tool(
        name="linode_profile_device_list",
        description="Lists trusted devices for the Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_profile_device_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_device_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return {
            "devices": await client.list_profile_devices(page=page, page_size=page_size)
        }

    return await execute_tool(
        cfg, arguments, "list Linode profile trusted devices", _call
    )


def create_linode_profile_login_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_login_get tool."""
    return Tool(
        name="linode_profile_login_get",
        description="Retrieves a Linode profile login by login ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "login_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the profile login to retrieve",
                },
            },
            "required": ["login_id"],
        },
    ), Capability.Read


async def handle_linode_profile_login_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_login_get tool request."""
    login_id = arguments.get("login_id")
    if isinstance(login_id, bool) or not isinstance(login_id, int) or login_id < 1:
        return error_response("login_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_profile_login(login_id)

    return await execute_tool(cfg, arguments, "retrieve Linode profile login", _call)


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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["token_id", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_token_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_update tool request."""
    if is_dry_run(arguments):
        tid = _profile_required_id(arguments, "token_id")
        if isinstance(tid, list):
            return tid
        token_id = tid

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_profile_token(token_id)

        label = arguments.get("label")

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            effect = (
                f"The personal access token's label is set to {label!r}."
                if label
                else "The personal access token is updated."
            )
            return {"side_effects": [effect]}

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_profile_token_update",
            "PUT",
            f"/profile/tokens/{token_id}",
            _fetch,
            _walk,
        )

    parsed_id = _profile_required_id(arguments, "token_id")
    if isinstance(parsed_id, list):
        return parsed_id
    token_id = parsed_id

    label = arguments.get("label")
    label_error = _token_label_error(label)
    if label_error is not None:
        return label_error
    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a profile token. Set confirm=true to proceed."
        )
    label_value = cast("str", label)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_profile_token(token_id, label=label_value)

    return await execute_tool(cfg, arguments, "update Linode profile token", _call)


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_profile_app_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_app_list tool."""
    return Tool(
        name="linode_profile_app_list",
        description="Lists OAuth app authorizations from the Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_profile_app_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_app_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_profile_apps(page=page, page_size=page_size)

    return await execute_tool(
        cfg, arguments, "list Linode profile OAuth app authorizations", _call
    )


def create_linode_profile_app_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_app_get tool."""
    return Tool(
        name="linode_profile_app_get",
        description=(
            "Retrieves an OAuth app authorization from the Linode profile by app ID."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "app_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the OAuth app authorization to retrieve",
                },
            },
            "required": ["app_id"],
        },
    ), Capability.Read


async def handle_linode_profile_app_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_app_get tool request."""
    app_id = arguments.get("app_id")
    if isinstance(app_id, bool) or not isinstance(app_id, int) or app_id < 1:
        return error_response("app_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_profile_app(app_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode profile OAuth app authorization", _call
    )


def create_linode_profile_app_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_app_delete tool."""
    return Tool(
        name="linode_profile_app_delete",
        description="Revokes OAuth app access from the Linode profile by app ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "app_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the OAuth app authorization to revoke",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["app_id", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_app_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_app_delete tool request."""
    if is_dry_run(arguments):
        aid = _profile_required_id(arguments, "app_id")
        if isinstance(aid, list):
            return aid

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_profile_app(aid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_profile_app_delete",
            "DELETE",
            f"/profile/apps/{aid}",
            _fetch,
        )

    app_id = arguments.get("app_id")
    if isinstance(app_id, bool) or not isinstance(app_id, int) or app_id < 1:
        return error_response("app_id must be a positive integer")
    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes profile OAuth app access. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_profile_app(app_id)
        return {"message": f"Profile app {app_id} revoked successfully"}

    return await execute_tool(
        cfg, arguments, "revoke Linode profile OAuth app access", _call
    )


def create_linode_profile_device_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_device_get tool."""
    return Tool(
        name="linode_profile_device_get",
        description="Retrieves a trusted device from the Linode profile by device ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "device_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the trusted device to retrieve",
                },
            },
            "required": ["device_id"],
        },
    ), Capability.Read


async def handle_linode_profile_device_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_device_get tool request."""
    device_id = arguments.get("device_id")
    if isinstance(device_id, bool) or not isinstance(device_id, int) or device_id < 1:
        return error_response("device_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_profile_device(device_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode profile trusted device", _call
    )


def create_linode_profile_device_revoke_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_device_revoke tool."""
    return Tool(
        name="linode_profile_device_revoke",
        description="Revokes a trusted device from the Linode profile by device ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "device_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "ID of the trusted device to revoke",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["device_id", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_device_revoke(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_device_revoke tool request."""
    if is_dry_run(arguments):
        did = _profile_required_id(arguments, "device_id")
        if isinstance(did, list):
            return did

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_profile_device(did)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_profile_device_revoke",
            "DELETE",
            f"/profile/devices/{did}",
            _fetch,
        )

    device_id = arguments.get("device_id")
    if isinstance(device_id, bool) or not isinstance(device_id, int) or device_id < 1:
        return error_response("device_id must be a positive integer")
    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes a trusted profile device. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_profile_device(device_id)
        return {"message": f"Profile trusted device {device_id} revoked successfully"}

    return await execute_tool(
        cfg, arguments, "revoke Linode profile trusted device", _call
    )


def create_linode_profile_token_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_delete tool."""
    return Tool(
        name="linode_profile_token_delete",
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["token_id", "confirm"],
        },
    ), Capability.Admin


async def handle_linode_profile_token_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_delete tool request."""
    if is_dry_run(arguments):
        tid = _profile_required_id(arguments, "token_id")
        if isinstance(tid, list):
            return tid

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_profile_token(tid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_profile_token_delete",
            "DELETE",
            f"/profile/tokens/{tid}",
            _fetch,
        )

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

"""Linode profile tool - user account profile information."""

from typing import Any, cast

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.genpb.linode.mcp.v1 import account_pb2, common_pb2, profile_pb2
from linodemcp.linode import RetryableClient, build_profile_security_questions_body
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    required_int_id,
)
from linodemcp.tools.proto_response import (
    raw_str,
    serialize_api_response,
    serialize_list_response,
    serialize_struct_response,
)
from linodemcp.tools.toolschemas import schema

PROFILE_TOKEN_LABEL_MAX_LENGTH = 100


def create_linode_profile_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_get tool."""
    return Tool(
        name="linode_profile_get",
        description="Retrieves Linode user account profile information",
        inputSchema=schema("linode.mcp.v1.ProfileGetInput"),
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
        return serialize_api_response(
            await client.get_raw("/profile"), profile_pb2.Profile()
        )

    return await execute_tool(cfg, arguments, "retrieve Linode profile", _call)


def create_linode_profile_preferences_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_preferences_get tool."""
    return Tool(
        name="linode_profile_preferences_get",
        description="Gets OAuth client-specific Linode profile preferences.",
        inputSchema=schema("linode.mcp.v1.ProfilePreferencesGetInput"),
    ), Capability.Read


async def handle_linode_profile_preferences_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_preferences_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_struct_response(await client.get_profile_preferences())

    return await execute_tool(
        cfg, arguments, "retrieve Linode profile preferences", _call
    )


def create_linode_profile_preferences_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_preferences_update tool."""
    return Tool(
        name="linode_profile_preferences_update",
        description="Updates OAuth client-specific Linode profile preferences.",
        inputSchema=schema("linode.mcp.v1.ProfilePreferencesUpdateInput"),
    ), Capability.Write


async def handle_linode_profile_preferences_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_preferences_update tool request."""
    if is_dry_run(arguments):
        preferences_arg = arguments.get("preferences")
        if not isinstance(preferences_arg, dict) or not preferences_arg:
            return error_response("preferences must be a non-empty object")

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
    if not isinstance(preferences_arg, dict) or not preferences_arg:
        return error_response("preferences must be a non-empty object")
    preferences = cast("dict[str, Any]", preferences_arg)
    if arguments.get("confirm") is not True:
        return error_response(
            "This updates profile preferences. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        updated = await client.update_profile_preferences(preferences)
        return serialize_api_response(
            {
                "message": "Profile preferences updated successfully",
                "preferences": updated,
            },
            profile_pb2.ProfilePreferencesUpdateResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.ProfileTfaEnableInput"),
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
            "This generates a two-factor authentication secret. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        secret = await client.create_profile_tfa_secret()
        # The one-time secret is returned to the user by design (it must be
        # confirmed to activate two-factor auth), so it is not output-redacted.
        return serialize_api_response(
            {
                **secret,
                "warning": (
                    "IMPORTANT: Save this two-factor authentication secret now. "
                    "It must be confirmed before two-factor authentication is "
                    "enabled."
                ),
            },
            profile_pb2.ProfileTfaEnableResponse(),
        )

    return await execute_tool(
        cfg, arguments, "generate profile two-factor authentication secret", _call
    )


def create_linode_profile_tfa_disable_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_tfa_disable tool."""
    return Tool(
        name="linode_profile_tfa_disable",
        description="Disables two-factor authentication for the Linode profile.",
        inputSchema=schema("linode.mcp.v1.ProfileTfaDisableInput"),
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
            "This disables two-factor authentication for the profile. Set confirm=true "
            "to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The disable endpoint returns no useful resource body, so the response
        # is a bare confirmation message.
        await client.disable_profile_tfa()
        message = "Profile two-factor authentication disabled successfully"
        return serialize_api_response(
            {"message": message},
            common_pb2.MessageResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.ProfileTfaEnableConfirmInput"),
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
            "This enables two-factor authentication for the profile. Set confirm=true "
            "to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        confirmed = await client.confirm_profile_tfa_enable(tfa_code=tfa_code.strip())
        # scratch is the one-time backup code the API returns on confirmation; it
        # is returned to the user by design (account recovery), not redacted.
        return serialize_api_response(
            {
                "message": ("Profile two-factor authentication enabled successfully"),
                "scratch": raw_str(confirmed, "scratch"),
                "expiry": raw_str(confirmed, "expiry"),
            },
            profile_pb2.ProfileTfaEnableConfirmResponse(),
        )

    return await execute_tool(
        cfg, arguments, "confirm profile two-factor authentication", _call
    )


def create_linode_profile_phone_number_send_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_phone_number_send tool."""
    return Tool(
        name="linode_profile_phone_number_send",
        description="Sends a verification code to a Linode profile phone number.",
        inputSchema=schema("linode.mcp.v1.ProfilePhoneNumberSendInput"),
    ), Capability.Admin


def _profile_required_id(
    arguments: dict[str, Any], name: str
) -> int | list[TextContent]:
    """Parse a required positive-integer id, or return an error response."""
    value, error = required_int_id(arguments, name)
    if value is None:
        return error_response(error)
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
            "This sends a phone number verification code. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The send endpoint returns no useful resource body, so the response is
        # a bare confirmation message.
        await client.send_profile_phone_number_verification(iso_code, phone_number)
        return serialize_api_response(
            {"message": "Profile phone number verification code sent successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(
        cfg, arguments, "send Linode profile phone number verification code", _call
    )


def create_linode_profile_phone_number_verify_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_phone_number_verify tool."""
    return Tool(
        name="linode_profile_phone_number_verify",
        description="Verifies the Linode profile phone number using an SMS code.",
        inputSchema=schema("linode.mcp.v1.ProfilePhoneNumberVerifyInput"),
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
        # The verify endpoint returns no useful resource body, so the response
        # is a bare confirmation message.
        await client.verify_profile_phone_number(otp_code.strip())
        return serialize_api_response(
            {"message": "Profile phone number verified successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(
        cfg, arguments, "verify Linode profile phone number", _call
    )


def create_linode_profile_phone_number_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_phone_number_delete tool."""
    return Tool(
        name="linode_profile_phone_number_delete",
        description="Deletes the verified Linode profile phone number.",
        inputSchema=schema("linode.mcp.v1.ProfilePhoneNumberDeleteInput"),
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
            "This deletes the profile phone number. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The delete endpoint returns no useful resource body, so the response
        # is a bare confirmation message.
        await client.delete_profile_phone_number()
        return serialize_api_response(
            {"message": "Profile phone number deleted successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(
        cfg, arguments, "delete Linode profile phone number", _call
    )


def create_linode_profile_security_question_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_security_question_list tool."""
    return Tool(
        name="linode_profile_security_question_list",
        description="Lists available Linode profile security questions.",
        inputSchema=schema("linode.mcp.v1.SecurityQuestionListInput"),
    ), Capability.Read


async def handle_linode_profile_security_question_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_security_question_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The endpoint wraps its elements under "security_questions", not a
        # {data} page envelope; unwrap and rewrap for the list helper.
        raw = await client.list_profile_security_questions()
        items: list[Any] = raw.get("security_questions") or []
        return serialize_list_response(
            {"data": items},
            "security_questions",
            profile_pb2.SecurityQuestionListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode profile security questions", _call
    )


def create_linode_profile_security_question_answer_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_security_question_answer tool."""
    return Tool(
        name="linode_profile_security_question_answer",
        description="Answers profile security questions for the Linode account.",
        inputSchema=schema("linode.mcp.v1.SecurityQuestionAnswerInput"),
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
            "This submits profile security question answers. Set confirm=true to "
            "proceed."
        )

    security_questions = arguments.get("security_questions")
    try:
        body = build_profile_security_questions_body(security_questions)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    body_questions = body["security_questions"]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The API echoes the answered questions, but the answers are sensitive
        # and the Go reference returns only a confirmation message, so the
        # response body is discarded in favor of a bare message.
        await client.answer_profile_security_questions(body_questions)
        return serialize_api_response(
            {"message": "Profile security questions answered successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(
        cfg, arguments, "answer profile security questions", _call
    )


def create_linode_profile_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_create tool."""
    return Tool(
        name="linode_profile_token_create",
        description="Creates a Linode personal access token.",
        inputSchema=schema("linode.mcp.v1.ProfileTokenCreateInput"),
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
            "This creates a personal access token. Set confirm=true to proceed."
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
        # The one-time token secret is returned to the user by design (it is
        # shown only at creation), so it is not output-redacted.
        return serialize_api_response(
            {
                "warning": (
                    "IMPORTANT: The token below is shown ONLY ONCE. "
                    "Save it now - it cannot be retrieved later."
                ),
                "token": token,
            },
            profile_pb2.ProfileTokenCreateResponse(),
        )

    return await execute_tool(cfg, arguments, "create Linode profile token", _call)


def create_linode_profile_token_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_list tool."""
    return Tool(
        name="linode_profile_token_list",
        description="Lists Linode personal access tokens.",
        inputSchema=schema("linode.mcp.v1.ProfileTokenListInput"),
    ), Capability.Read


async def handle_linode_profile_token_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        tokens = await client.list_profile_tokens(page=page, page_size=page_size)
        # The proto PersonalAccessToken models no secret field, so any token
        # value the API returns is dropped on serialize: the list metadata-only
        # output never leaks a token secret.
        return serialize_list_response(
            {"data": tokens},
            "profile_tokens",
            profile_pb2.PersonalAccessTokenListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode profile tokens", _call)


def create_linode_profile_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_get tool."""
    return Tool(
        name="linode_profile_token_get",
        description="Retrieves a Linode personal access token by token ID.",
        inputSchema=schema("linode.mcp.v1.ProfileTokenGetInput"),
    ), Capability.Read


async def handle_linode_profile_token_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_token_get tool request."""
    token_id, error = required_int_id(arguments, "token_id")
    if token_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_profile_token(token_id),
            profile_pb2.PersonalAccessToken(),
        )

    return await execute_tool(cfg, arguments, "retrieve Linode profile token", _call)


def create_linode_profile_login_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_login_list tool."""
    return Tool(
        name="linode_profile_login_list",
        description="Lists recent successful Linode profile logins.",
        inputSchema=schema("linode.mcp.v1.ProfileLoginListInput"),
    ), Capability.Read


async def handle_linode_profile_login_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_login_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        logins = await client.list_profile_logins(page=page, page_size=page_size)
        return serialize_list_response(
            {"data": logins},
            "profile_logins",
            account_pb2.ProfileLoginListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode profile logins", _call)


def create_linode_profile_device_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_device_list tool."""
    return Tool(
        name="linode_profile_device_list",
        description="Lists trusted devices for the Linode profile.",
        inputSchema=schema("linode.mcp.v1.ProfileDeviceListInput"),
    ), Capability.Read


async def handle_linode_profile_device_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_device_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        devices = await client.list_profile_devices(page=page, page_size=page_size)
        return serialize_list_response(
            {"data": devices},
            "profile_devices",
            profile_pb2.TrustedDeviceListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode profile trusted devices", _call
    )


def create_linode_profile_login_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_login_get tool."""
    return Tool(
        name="linode_profile_login_get",
        description="Retrieves a Linode profile login by login ID.",
        inputSchema=schema("linode.mcp.v1.ProfileLoginGetInput"),
    ), Capability.Read


async def handle_linode_profile_login_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_login_get tool request."""
    login_id = arguments.get("login_id")
    if isinstance(login_id, bool) or not isinstance(login_id, int) or login_id < 1:
        return error_response("login_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_profile_login(login_id), account_pb2.AccountLogin()
        )

    return await execute_tool(cfg, arguments, "retrieve Linode profile login", _call)


def create_linode_profile_token_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_update tool."""
    return Tool(
        name="linode_profile_token_update",
        description="Updates a Linode personal access token label by token ID.",
        inputSchema=schema("linode.mcp.v1.ProfileTokenUpdateInput"),
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
            "This updates a personal access token. Set confirm=true to proceed."
        )
    label_value = cast("str", label)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.update_profile_token(token_id, label=label_value)
        return serialize_api_response(
            {"message": "Profile token updated successfully", "token": token},
            profile_pb2.PersonalAccessTokenWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode profile token", _call)


def create_linode_profile_app_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_app_list tool."""
    return Tool(
        name="linode_profile_app_list",
        description="Lists OAuth app authorizations from the Linode profile.",
        inputSchema=schema("linode.mcp.v1.ProfileAppListInput"),
    ), Capability.Read


async def handle_linode_profile_app_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_app_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_profile_apps(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "profile_apps",
            profile_pb2.ProfileAppListResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.ProfileAppGetInput"),
    ), Capability.Read


async def handle_linode_profile_app_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_app_get tool request."""
    app_id = arguments.get("app_id")
    if isinstance(app_id, bool) or not isinstance(app_id, int) or app_id < 1:
        return error_response("app_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_profile_app(app_id), profile_pb2.ProfileApp()
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode profile OAuth app authorization", _call
    )


def create_linode_profile_app_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_app_delete tool."""
    return Tool(
        name="linode_profile_app_delete",
        description="Revokes OAuth app access from the Linode profile by app ID.",
        inputSchema=schema("linode.mcp.v1.ProfileAppDeleteInput"),
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
            "This revokes OAuth app access. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_profile_app(app_id)
        return serialize_api_response(
            {
                "message": f"Profile app {app_id} revoked successfully",
                "app_id": app_id,
            },
            profile_pb2.ProfileAppIDResponse(),
        )

    return await execute_tool(
        cfg, arguments, "revoke Linode profile OAuth app access", _call
    )


def create_linode_profile_device_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_device_get tool."""
    return Tool(
        name="linode_profile_device_get",
        description="Retrieves a trusted device from the Linode profile by device ID.",
        inputSchema=schema("linode.mcp.v1.ProfileDeviceGetInput"),
    ), Capability.Read


async def handle_linode_profile_device_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile_device_get tool request."""
    device_id = arguments.get("device_id")
    if isinstance(device_id, bool) or not isinstance(device_id, int) or device_id < 1:
        return error_response("device_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_profile_device(device_id),
            profile_pb2.TrustedDevice(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode profile trusted device", _call
    )


def create_linode_profile_device_revoke_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_device_revoke tool."""
    return Tool(
        name="linode_profile_device_revoke",
        description="Revokes a trusted device from the Linode profile by device ID.",
        inputSchema=schema("linode.mcp.v1.ProfileDeviceRevokeInput"),
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
            "This revokes a trusted device. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_profile_device(device_id)
        message = f"Profile trusted device {device_id} revoked successfully"
        return serialize_api_response(
            {"message": message, "device_id": device_id},
            profile_pb2.ProfileDeviceIDResponse(),
        )

    return await execute_tool(
        cfg, arguments, "revoke Linode profile trusted device", _call
    )


def create_linode_profile_token_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_profile_token_delete tool."""
    return Tool(
        name="linode_profile_token_delete",
        description="Revokes a Linode personal access token by token ID.",
        inputSchema=schema("linode.mcp.v1.ProfileTokenDeleteInput"),
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

    token_id, error = required_int_id(arguments, "token_id")
    if token_id is None:
        return error_response(error)
    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes a personal access token. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_profile_token(token_id)
        return serialize_api_response(
            {
                "message": f"Profile token {token_id} revoked successfully",
                "token_id": token_id,
            },
            profile_pb2.ProfileTokenIDResponse(),
        )

    return await execute_tool(cfg, arguments, "revoke Linode profile token", _call)

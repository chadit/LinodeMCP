"""Linode account tool - authenticated user account information."""

import base64
import binascii
import re
from pathlib import Path
from typing import Any, cast
from urllib.parse import quote

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.genpb.linode.mcp.v1 import (
    account_availability_pb2,
    account_beta_program_pb2,
    account_event_pb2,
    account_pb2,
    account_service_transfer_pb2,
    account_user_pb2,
    beta_program_pb2,
    common_pb2,
    managed_issue_pb2,
    managed_pb2,
    oauth_client_thumbnail_pb2,
    support_ticket_pb2,
    tag_pb2,
)
from linodemcp.linode import RetryableClient
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    PARAM_DRY_RUN,
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    required_int_id,
)
from linodemcp.tools.proto_enum import enum_choice_error, enum_value_names
from linodemcp.tools.proto_response import (
    raw_str,
    serialize_api_response,
    serialize_list_response,
    serialize_struct_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

_CHILD_ACCOUNT_EUUID_PATTERN = re.compile(
    r"^[A-Za-z0-9]{8}-[A-Za-z0-9]{4}-[A-Za-z0-9]{4}-[A-Za-z0-9]{16}$"
)
_OAUTH_CLIENT_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")
_ACCOUNT_USERNAME_PATTERN_TEXT = r"^[A-Za-z0-9][A-Za-z0-9_-]*$"
_ACCOUNT_USERNAME_PATTERN = re.compile(_ACCOUNT_USERNAME_PATTERN_TEXT)
_ACCOUNT_GRANT_FIELDS = (
    "database",
    "domain",
    "firewall",
    "global",
    "image",
    "linode",
    "lkecluster",
    "longview",
    "nodebalancer",
    "stackscript",
    "volume",
    "vpc",
)
_GLOBAL_PERMISSION_FIELDS = {"account_access"}
_GLOBAL_BOOLEAN_FIELDS = {
    "add_databases",
    "add_domains",
    "add_firewalls",
    "add_images",
    "add_linodes",
    "add_longview",
    "add_nodebalancers",
    "add_stackscripts",
    "add_volumes",
    "add_vpcs",
    "cancel_account",
    "longview_subscription",
}
_GLOBAL_NULLABLE_BOOLEAN_FIELDS = {"child_account_access"}
_MANAGED_LINODE_SSH_PORT_MAX = 65535
_MANAGED_LINODE_SSH_USER_MAX_LENGTH = 32
_MANAGED_SERVICE_TIMEOUT_MAX = 255
_RESOURCE_GRANT_FIELDS = tuple(
    field for field in _ACCOUNT_GRANT_FIELDS if field != "global"
)


def _validate_account_username(value: object) -> tuple[str | None, str | None]:
    """Validate a finite account username path parameter."""
    if value is None:
        return None, "username is required"
    if not isinstance(value, str):
        return None, "username must be a string"

    username = value.strip()
    if not username:
        return None, "username is required"
    if username != value or not _ACCOUNT_USERNAME_PATTERN.fullmatch(username):
        return (
            None,
            "username must contain only letters, numbers, underscores, or hyphens",
        )
    return username, None


def _validate_resource_grant_entry(
    field: str, entry: object, index: int
) -> tuple[dict[str, int | str | None] | None, str | None]:
    """Validate one per-resource grant entry."""
    if not isinstance(entry, dict):
        return None, f"{field}[{index}] must be an object"
    typed_entry = cast("dict[str, Any]", entry)
    allowed_entry_keys: set[str] = {"id", "permissions"}
    unknown_keys = sorted(set(typed_entry) - allowed_entry_keys)
    if unknown_keys:
        return None, f"{field}[{index}] has unknown fields: " + ", ".join(unknown_keys)

    resource_id = typed_entry.get("id")
    if not isinstance(resource_id, int) or isinstance(resource_id, bool):
        return None, f"{field}[{index}].id must be an integer"

    if "permissions" not in typed_entry:
        return None, f"{field}[{index}].permissions is required"

    permissions = typed_entry.get("permissions")
    allowed = enum_value_names(account_user_pb2.GrantPermission.Value)
    if permissions is not None and (
        not isinstance(permissions, str) or permissions not in allowed
    ):
        return (
            None,
            f"{field}[{index}].permissions must be one of: " + ", ".join(allowed),
        )
    return {"id": resource_id, "permissions": permissions}, None


def _validate_resource_grants(
    field: str, value: object
) -> tuple[list[dict[str, int | str | None]] | None, str | None]:
    """Validate a per-resource grant list."""
    if not isinstance(value, list):
        return None, f"{field} must be an array of grant objects"

    entries = cast("list[object]", value)
    grants: list[dict[str, int | str | None]] = []
    for index, entry in enumerate(entries):
        grant, error = _validate_resource_grant_entry(field, entry, index)
        if error is not None or grant is None:
            return None, error or f"{field}[{index}] is invalid"
        grants.append(grant)
    return grants, None


def _validate_global_grants(
    value: object,
) -> tuple[dict[str, str | bool | None] | None, str | None]:
    """Validate the global grants object."""
    if not isinstance(value, dict):
        return None, "global must be an object"

    typed_value = cast("dict[str, Any]", value)
    allowed_keys: set[str] = (
        _GLOBAL_PERMISSION_FIELDS
        | _GLOBAL_BOOLEAN_FIELDS
        | _GLOBAL_NULLABLE_BOOLEAN_FIELDS
    )
    unknown_keys = sorted(set(typed_value) - allowed_keys)
    if unknown_keys:
        return None, "global has unknown fields: " + ", ".join(unknown_keys)

    grants: dict[str, str | bool | None] = {}
    for field, grant_value in typed_value.items():
        if field in _GLOBAL_PERMISSION_FIELDS:
            allowed = enum_value_names(account_user_pb2.GrantPermission.Value)
            if grant_value is not None and (
                not isinstance(grant_value, str) or grant_value not in allowed
            ):
                return (
                    None,
                    f"global.{field} must be one of: " + ", ".join(allowed),
                )
        elif field in _GLOBAL_BOOLEAN_FIELDS:
            if type(grant_value) is not bool:
                return None, f"global.{field} must be a boolean"
        elif type(grant_value) is not bool and grant_value is not None:
            return None, f"global.{field} must be a boolean or null"
        grants[field] = grant_value
    return grants, None


def _collect_account_user_grants(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    """Collect and validate account user grant update fields."""
    allowed_keys = {"environment", "username", "confirm", PARAM_DRY_RUN}
    allowed_keys.update(_ACCOUNT_GRANT_FIELDS)
    unknown_keys = sorted(set(arguments) - allowed_keys)
    if unknown_keys:
        return None, "unknown grant update fields: " + ", ".join(unknown_keys)

    grants: dict[str, Any] = {}
    if "global" in arguments:
        global_grants, error = _validate_global_grants(arguments["global"])
        if error is not None or global_grants is None:
            return None, error or "global is invalid"
        grants["global"] = global_grants

    for field in _RESOURCE_GRANT_FIELDS:
        if field not in arguments:
            continue
        resource_grants, error = _validate_resource_grants(field, arguments[field])
        if error is not None or resource_grants is None:
            return None, error or f"{field} is invalid"
        grants[field] = resource_grants

    if not grants:
        return None, "at least one grant field is required"
    return grants, None


def _validate_service_transfer_token(value: object) -> tuple[str | None, str | None]:
    """Validate a service transfer token supplied by an MCP caller.

    Mirrors Go's accountTransferTokenFromTool: missing -> "token is required";
    non-string/blank -> "token must be a non-empty string"; surrounding
    whitespace or path/query/traversal characters -> the path-safety message.
    """
    if value is None:
        return None, "token is required"
    if not isinstance(value, str) or not value.strip():
        return None, "token must be a non-empty string"
    if value != value.strip() or "/" in value or "?" in value or ".." in value:
        return (
            None,
            (
                "token must not contain path separators, "
                "query separators, or traversal segments"
            ),
        )
    return value, None


_OAUTH_CLIENT_UPDATE_FIELDS = (
    "label",
    "public",
    "redirect_uri",
)
_ACCOUNT_OAUTH_CLIENT_ID_PATTERN_TEXT = r"^[A-Za-z0-9][A-Za-z0-9_-]*$"
_ACCOUNT_OAUTH_CLIENT_ID_PATTERN = re.compile(_ACCOUNT_OAUTH_CLIENT_ID_PATTERN_TEXT)
_USD_AMOUNT_PATTERN = re.compile(r"^(?!0+(?:\.0{1,2})?$)\d+(?:\.\d{1,2})?$")


def create_linode_account_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_get tool."""
    return Tool(
        name="linode_account_get",
        description=(
            "Retrieves the authenticated user's Linode account information "
            "including billing details and capabilities"
        ),
        inputSchema=schema("linode.mcp.v1.AccountGetInput"),
    ), Capability.Read


async def handle_linode_account_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_get tool request.

    Args:
        arguments: EnvironmentArgs - environment (optional)
        cfg: Configuration object
    """

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_raw("/account"), account_pb2.Account()
        )

    return await execute_tool(cfg, arguments, "retrieve Linode account", _call)


def create_linode_account_user_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_create tool."""
    return Tool(
        name="linode_account_user_create",
        description="Creates a user on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountUserCreateInput"),
    ), Capability.Admin


async def handle_linode_account_user_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_create tool request."""
    try:
        username = _required_string_argument(arguments, "username")
        email = _required_string_argument(arguments, "email")
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    restricted = arguments.get("restricted", False)
    if type(restricted) is not bool:
        return error_response("restricted must be a boolean")

    body = {"username": username, "email": email, "restricted": restricted}

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_user_create",
            arguments.get("environment", ""),
            "POST",
            "/account/users",
            None,
            side_effects=[
                (
                    f"A new account user {username!r} will be created "
                    f"with restricted={restricted}."
                )
            ],
            request_body=body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an account user. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        user = await client.create_account_user(username, email, restricted)
        return serialize_api_response(
            {"message": "Account user created successfully", "user": user},
            account_user_pb2.AccountUserWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Linode account user", _call)


def create_linode_account_user_grants_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_grants_update tool."""
    return Tool(
        name="linode_account_user_grants_update",
        description="Updates grants for an account user.",
        inputSchema=schema("linode.mcp.v1.AccountUserGrantsUpdateInput"),
    ), Capability.Admin


async def handle_linode_account_user_grants_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_grants_update tool request."""
    username, username_error = _validate_account_username(arguments.get("username"))
    if username_error is not None or username is None:
        return error_response(username_error or "username is required")

    grants, grants_error = _collect_account_user_grants(arguments)
    if grants_error is not None or grants is None:
        return error_response(grants_error or "at least one grant field is required")

    encoded_username = quote(username, safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_user_grants_update",
            arguments.get("environment", ""),
            "PUT",
            f"/account/users/{encoded_username}/grants",
            None,
            side_effects=[f"Account grants for user {username!r} will be updated."],
            request_body=grants,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates account user grants. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_account_user_grants(username, grants)
        return serialize_api_response(
            {"message": "Account user grants updated successfully", "grants": result},
            account_user_pb2.AccountUserGrantsWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"update Linode account user grants for {username}", _call
    )


def create_linode_account_agreement_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_agreement_list tool."""
    return Tool(
        name="linode_account_agreement_list",
        description="Lists agreements on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountAgreementsListInput"),
    ), Capability.Read


async def handle_linode_account_agreement_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_agreement_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_agreements(), account_pb2.AccountAgreements()
        )

    return await execute_tool(cfg, arguments, "list Linode account agreements", _call)


def create_linode_account_login_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_login_list tool."""
    return Tool(
        name="linode_account_login_list",
        description="Lists user logins on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountLoginListInput"),
    ), Capability.Read


async def handle_linode_account_login_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_login_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_logins(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_logins",
            account_pb2.AccountLoginListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account logins", _call)


def create_linode_account_user_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_list tool."""
    return Tool(
        name="linode_account_user_list",
        description="Lists users on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountUserListInput"),
    ), Capability.Read


async def handle_linode_account_user_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_users(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_users",
            account_user_pb2.AccountUserListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account users", _call)


def create_linode_account_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_settings_get tool."""
    return Tool(
        name="linode_account_settings_get",
        description="Gets settings for the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountSettingsGetInput"),
    ), Capability.Read


async def handle_linode_account_settings_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_settings_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_settings(), account_pb2.AccountSettings()
        )

    return await execute_tool(cfg, arguments, "get Linode account settings", _call)


def create_linode_account_settings_managed_enable_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_settings_managed_enable tool."""
    return Tool(
        name="linode_account_settings_managed_enable",
        description=(
            "Enables Linode Managed for the account. Pass dry_run=true to "
            "preview without enabling it."
        ),
        inputSchema=schema("linode.mcp.v1.AccountSettingsManagedEnableInput"),
    ), Capability.Admin


async def handle_linode_account_settings_managed_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_settings_managed_enable tool request."""
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_settings_managed_enable",
            arguments.get("environment", ""),
            "POST",
            "/account/settings/managed-enable",
            None,
            side_effects=["Linode Managed is enabled for this account."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This enables Linode Managed for the account. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.enable_account_managed()
        return serialize_api_response(
            {"message": "Linode Managed enabled successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(cfg, arguments, "enable Linode Managed", _call)


def create_linode_account_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_transfer_get tool."""
    return Tool(
        name="linode_account_transfer_get",
        description="Gets network transfer usage for the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountTransferGetInput"),
    ), Capability.Read


async def handle_linode_account_transfer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_transfer_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_transfer(), account_pb2.AccountTransfer()
        )

    return await execute_tool(cfg, arguments, "get Linode account transfer", _call)


def create_linode_account_maintenance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_maintenance_list tool."""
    return Tool(
        name="linode_account_maintenance_list",
        description="Lists maintenances on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountMaintenanceListInput"),
    ), Capability.Read


async def handle_linode_account_maintenance_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_maintenance_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_maintenance(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_maintenances",
            account_pb2.AccountMaintenanceListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account maintenance", _call)


def create_linode_maintenance_policy_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_maintenance_policy_list tool."""
    return Tool(
        name="linode_maintenance_policy_list",
        description="Lists available maintenance policies.",
        inputSchema=schema("linode.mcp.v1.MaintenancePolicyListInput"),
    ), Capability.Read


async def handle_linode_maintenance_policy_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_maintenance_policy_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_maintenance_policies(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "maintenance_policies",
            account_pb2.MaintenancePolicyListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode maintenance policies", _call)


def create_linode_account_oauth_client_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_list tool."""
    return Tool(
        name="linode_account_oauth_client_list",
        description="Lists OAuth clients on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientListInput"),
    ), Capability.Read


async def handle_linode_account_oauth_client_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_oauth_clients(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_oauth_clients",
            account_pb2.OAuthClientListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode account OAuth clients", _call
    )


def create_linode_account_oauth_client_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_update tool."""
    return Tool(
        name="linode_account_oauth_client_update",
        description=(
            "Updates one Linode account OAuth client. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientUpdateInput"),
    ), Capability.Admin


def create_linode_account_oauth_client_thumbnail_update_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_account_oauth_client_thumbnail_update tool."""
    return Tool(
        name="linode_account_oauth_client_thumbnail_update",
        description=(
            "Updates an account OAuth client's thumbnail. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientThumbnailUpdateInput"),
    ), Capability.Admin


def _validate_oauth_client_id(value: Any) -> str | None:
    """Validate an OAuth client ID tool argument."""
    if not isinstance(value, str):
        return None
    client_id = value.strip()
    if (
        not client_id
        or client_id != value
        or _OAUTH_CLIENT_ID_PATTERN.fullmatch(client_id) is None
        or ".." in client_id
    ):
        return None
    return client_id


async def handle_linode_account_oauth_client_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_update tool request."""
    client_id = _validate_oauth_client_id(arguments.get("client_id"))
    if client_id is None:
        return error_response(
            "client_id must be a non-empty ID without path separators, "
            "query separators, or traversal segments"
        )

    update_fields = {
        key: arguments.get(key)
        for key in _OAUTH_CLIENT_UPDATE_FIELDS
        if arguments.get(key) is not None
    }
    if not update_fields:
        return error_response("At least one OAuth client field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_oauth_client_update",
            arguments.get("environment", ""),
            "PUT",
            f"/account/oauth-clients/{quote(client_id, safe='')}",
            None,
            side_effects=["The selected OAuth client configuration is updated."],
            request_body=update_fields,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an OAuth client. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        oauth_client = await client.update_account_oauth_client(
            client_id, **update_fields
        )
        return serialize_api_response(
            {
                "message": "OAuth client updated successfully",
                "client": oauth_client,
            },
            account_pb2.OAuthClientWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "update Linode account OAuth client", _call
    )


def _decode_thumbnail_png(value: Any) -> tuple[bytes | None, str | None]:
    """Decode the thumbnail_png_base64 argument; return (bytes, error message).

    Mirrors the Go handler, which requires a non-empty standard-base64 string
    and posts the decoded PNG bytes to the thumbnail endpoint.
    """
    if not isinstance(value, str) or not value.strip():
        return None, "thumbnail_png_base64 must be a non-empty string"
    try:
        # validate=True rejects non-alphabet characters, matching Go's strict
        # base64.StdEncoding.DecodeString rather than silently dropping them.
        thumbnail_png = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        return None, "thumbnail_png_base64 must be valid standard base64"
    return thumbnail_png, None


async def handle_linode_account_oauth_client_thumbnail_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_thumbnail_update tool request."""
    client_id = _validate_oauth_client_id(arguments.get("client_id"))
    if client_id is None:
        return error_response(
            "client_id must be a non-empty ID without path separators, "
            "query separators, or traversal segments"
        )

    thumbnail_png, thumbnail_err = _decode_thumbnail_png(
        arguments.get("thumbnail_png_base64")
    )
    if thumbnail_png is None:
        return error_response(
            thumbnail_err or "thumbnail_png_base64 must be a non-empty string"
        )

    encoded_client_id = quote(client_id, safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_oauth_client_thumbnail_update",
            arguments.get("environment", ""),
            "PUT",
            f"/account/oauth-clients/{encoded_client_id}/thumbnail",
            None,
            request_body={},
            side_effects=["The account OAuth client's thumbnail is updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an OAuth client thumbnail. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.update_account_oauth_client_thumbnail(client_id, thumbnail_png)
        return serialize_api_response(
            {
                "message": "OAuth client thumbnail updated successfully",
                "client_id": client_id,
            },
            account_pb2.OAuthClientIDResponse(),
        )

    return await execute_tool(
        cfg, arguments, "update Linode account OAuth client thumbnail", _call
    )


def create_linode_account_event_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_list tool."""
    return Tool(
        name="linode_account_event_list",
        description="Lists events on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountEventListInput"),
    ), Capability.Read


async def handle_linode_account_event_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_event_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_events(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_events",
            account_event_pb2.AccountEventListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account events", _call)


def create_linode_account_invoice_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_invoice_list tool."""
    return Tool(
        name="linode_account_invoice_list",
        description="Lists invoices on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountInvoiceListInput"),
    ), Capability.Read


async def handle_linode_account_invoice_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_invoice_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_invoices(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_invoices",
            account_pb2.AccountInvoiceListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account invoices", _call)


def create_linode_account_payment_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_list tool."""
    return Tool(
        name="linode_account_payment_list",
        description="Lists payments on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountPaymentListInput"),
    ), Capability.Read


async def handle_linode_account_payment_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_payments(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_payments",
            account_pb2.AccountPaymentListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account payments", _call)


def create_linode_account_payment_method_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_method_list tool."""
    return Tool(
        name="linode_account_payment_method_list",
        description="Lists payment methods on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountPaymentMethodListInput"),
    ), Capability.Read


async def handle_linode_account_payment_method_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_payment_methods(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_payment_methods",
            account_pb2.AccountPaymentMethodListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode account payment methods", _call
    )


def create_linode_account_payment_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_create tool."""
    return Tool(
        name="linode_account_payment_create",
        description=(
            "Makes a payment on the Linode account. "
            "Pass dry_run=true to preview without creating a payment."
        ),
        inputSchema=schema("linode.mcp.v1.AccountPaymentCreateInput"),
    ), Capability.Admin


def _account_payment_create_body(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    usd, usd_error = _required_nonempty_string_argument(arguments, "usd")
    if usd_error is not None or usd is None:
        return None, usd_error or "usd is required"
    if _USD_AMOUNT_PATTERN.fullmatch(usd) is None:
        return None, "usd must be a positive dollar amount with up to two decimals"

    body: dict[str, Any] = {"usd": usd}

    # payment_method_id is optional; when omitted the API charges the
    # account's default method. Validate only when the caller supplies it.
    payment_method_id = arguments.get("payment_method_id")
    if payment_method_id is not None:
        if (
            isinstance(payment_method_id, bool)
            or not isinstance(payment_method_id, int)
            or payment_method_id < 1
        ):
            return None, "payment_method_id must be a positive integer"
        body["payment_method_id"] = payment_method_id

    return body, None


async def handle_linode_account_payment_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_create tool request."""
    request_body, validation_error = _account_payment_create_body(arguments)
    if validation_error is not None or request_body is None:
        return error_response(validation_error or "account payment body is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_payment_create",
            arguments.get("environment", ""),
            "POST",
            "/account/payments",
            None,
            request_body=request_body,
            side_effects=["A payment is created on the Linode account."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This makes an account payment. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        payment = await client.create_account_payment(
            str(request_body["usd"]),
            payment_method_id=request_body.get("payment_method_id"),
        )
        return serialize_api_response(
            {
                "message": "Account payment created successfully",
                "payment": payment,
            },
            account_pb2.AccountPaymentWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Linode account payment", _call)


def create_linode_account_service_transfer_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfer_create tool."""
    return Tool(
        name="linode_account_service_transfer_create",
        description=(
            "Requests a service transfer for Linode IDs on the account. "
            "Pass dry_run=true to preview without creating a service transfer."
        ),
        inputSchema=schema("linode.mcp.v1.AccountServiceTransferCreateInput"),
    ), Capability.Admin


def _account_service_transfer_linode_ids(
    arguments: dict[str, Any],
) -> tuple[list[int] | None, str | None]:
    raw_linode_ids = arguments.get("linode_ids")
    if raw_linode_ids is None:
        return None, "linode_ids is required"
    if not isinstance(raw_linode_ids, list) or not raw_linode_ids:
        return None, "linode_ids must be a non-empty list of positive integers"

    raw_linode_id_list = cast("list[object]", raw_linode_ids)
    linode_ids: list[int] = []
    for raw_linode_id in raw_linode_id_list:
        if (
            isinstance(raw_linode_id, bool)
            or not isinstance(raw_linode_id, int)
            or raw_linode_id < 1
        ):
            return None, "linode_ids must be a non-empty list of positive integers"
        linode_ids.append(raw_linode_id)
    return linode_ids, None


async def handle_linode_account_service_transfer_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_create tool request."""
    linode_ids, validation_error = _account_service_transfer_linode_ids(arguments)
    if validation_error is not None or linode_ids is None:
        return error_response(validation_error or "linode_ids are required")

    request_body = {"entities": {"linodes": linode_ids}}
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_service_transfer_create",
            arguments.get("environment", ""),
            "POST",
            "/account/service-transfers",
            None,
            request_body=request_body,
            side_effects=[
                "A service transfer request is created for the listed Linodes."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an account service transfer. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        transfer = await client.create_account_service_transfer(linode_ids)
        return serialize_api_response(
            {
                "message": "Account service transfer created successfully",
                "transfer": transfer,
            },
            account_service_transfer_pb2.AccountServiceTransferWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "create Linode account service transfer", _call
    )


def create_linode_account_payment_method_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_method_delete tool."""
    return Tool(
        name="linode_account_payment_method_delete",
        description=(
            "Deletes a payment method on the Linode account. "
            "Pass dry_run=true to preview without deleting."
        ),
        inputSchema=schema("linode.mcp.v1.AccountPaymentMethodDeleteInput"),
    ), Capability.Admin


async def handle_linode_account_payment_method_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_delete tool request."""
    payment_method_id = arguments.get("payment_method_id")
    if (
        not isinstance(payment_method_id, int)
        or isinstance(payment_method_id, bool)
        or payment_method_id < 1
    ):
        return error_response("payment_method_id must be a positive integer")

    encoded_payment_method_id = quote(str(payment_method_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_payment_method_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/account/payment-methods/{encoded_payment_method_id}",
            None,
            side_effects=["The selected account payment method is deleted."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a payment method. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_account_payment_method(payment_method_id)
        return serialize_api_response(
            {
                "message": "Payment method deleted successfully",
                "payment_method_id": payment_method_id,
            },
            account_pb2.AccountPaymentMethodIDResponse(),
        )

    return await execute_tool(
        cfg,
        arguments,
        f"delete Linode account payment method {payment_method_id}",
        _call,
    )


def create_linode_account_notification_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_notification_list tool."""
    return Tool(
        name="linode_account_notification_list",
        description="Lists notifications on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountNotificationListInput"),
    ), Capability.Read


async def handle_linode_account_notification_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_notification_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_notifications(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_notifications",
            account_pb2.AccountNotificationListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode account notifications", _call
    )


def create_linode_account_invoice_item_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_invoice_item_list tool."""
    return Tool(
        name="linode_account_invoice_item_list",
        description="Lists items on a Linode account invoice.",
        inputSchema=schema("linode.mcp.v1.AccountInvoiceItemListInput"),
    ), Capability.Read


async def handle_linode_account_invoice_item_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_invoice_item_list tool request."""
    invoice_id = arguments.get("invoice_id")
    if (
        not isinstance(invoice_id, int)
        or isinstance(invoice_id, bool)
        or invoice_id < 1
    ):
        return error_response("invoice_id must be a positive integer")

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_invoice_items(
            invoice_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "account_invoice_items",
            account_pb2.AccountInvoiceItemListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode account invoice items", _call
    )


def create_linode_account_event_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_get tool."""
    return Tool(
        name="linode_account_event_get",
        description="Gets a Linode account event by ID.",
        inputSchema=schema("linode.mcp.v1.AccountEventGetInput"),
    ), Capability.Read


async def handle_linode_account_event_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_event_get tool request."""
    event_id = arguments.get("event_id")
    if not isinstance(event_id, int) or isinstance(event_id, bool) or event_id < 1:
        return error_response("event_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_event(event_id),
            account_event_pb2.AccountEvent(),
        )

    return await execute_tool(cfg, arguments, "get Linode account event", _call)


def create_linode_account_event_seen_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_seen tool."""
    return Tool(
        name="linode_account_event_seen",
        description=(
            "Marks a Linode account event as seen. "
            "Pass dry_run=true to preview without marking the event seen."
        ),
        inputSchema=schema("linode.mcp.v1.AccountEventSeenInput"),
    ), Capability.Write


async def handle_linode_account_event_seen(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_event_seen tool request."""
    event_id = arguments.get("event_id")
    if not isinstance(event_id, int) or isinstance(event_id, bool) or event_id < 1:
        return error_response("event_id must be a positive integer")

    if is_dry_run(arguments):
        # Dry-run previews the current event with a safe GET. The response
        # still reports the POST that would run; it must not mark the event
        # seen when dry_run=true.
        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    (
                        "The specified account event and all earlier events are "
                        "marked as seen."
                    )
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_event_seen",
            "POST",
            f"/account/events/{event_id}/seen",
            lambda client: client.get_account_event(event_id),
            details_fn=_walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This marks an account event as seen. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.mark_account_event_seen(event_id)
        return serialize_api_response(
            {
                "message": "Account event marked as seen successfully",
                "event_id": event_id,
            },
            account_pb2.AccountEventSeenResponse(),
        )

    return await execute_tool(cfg, arguments, "mark Linode account event seen", _call)


_ACCOUNT_AGREEMENT_FIELDS = (
    "billing_agreement",
    "eu_model",
    "master_service_agreement",
    "privacy_policy",
)


def create_linode_account_agreement_acknowledge_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_agreement_acknowledge tool."""
    return Tool(
        name="linode_account_agreement_acknowledge",
        description="Acknowledges agreements on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountAgreementsAcknowledgeInput"),
    ), Capability.Admin


async def handle_linode_account_agreement_acknowledge(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_agreement_acknowledge tool request."""
    agreements: dict[str, bool] = {}
    for field in _ACCOUNT_AGREEMENT_FIELDS:
        value = arguments.get(field)
        if value is None:
            continue
        if not isinstance(value, bool):
            return error_response(f"{field} must be a boolean")
        if not value:
            return error_response(f"{field} must be true when provided")
        agreements[field] = value

    if not agreements:
        return error_response("At least one account agreement field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_agreement_acknowledge",
            arguments.get("environment", ""),
            "POST",
            "/account/agreements",
            None,
            side_effects=[
                "The selected account agreements are acknowledged for this account."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This acknowledges account agreements. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.acknowledge_account_agreements(agreements)
        return serialize_api_response(
            {"message": "Account agreements acknowledged successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(
        cfg, arguments, "acknowledge Linode account agreements", _call
    )


def create_linode_account_beta_enroll_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_beta_enroll tool."""
    return Tool(
        name="linode_account_beta_enroll",
        description=(
            "Enrolls the Linode account in a Beta program. "
            "Pass dry_run=true to preview without enrolling."
        ),
        inputSchema=schema("linode.mcp.v1.AccountBetaEnrollInput"),
    ), Capability.Admin


async def handle_linode_account_beta_enroll(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_enroll tool request."""
    beta_id, beta_id_error = _required_pathsafe_string(arguments, "id")
    if beta_id_error is not None or beta_id is None:
        return error_response(beta_id_error or "id is required")
    if beta_id != beta_id.strip() or not _is_account_beta_id(beta_id):
        return error_response(
            "id must contain only letters, numbers, underscores, and hyphens"
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_beta_enroll",
            arguments.get("environment", ""),
            "POST",
            "/account/betas",
            None,
            request_body={"id": beta_id},
            side_effects=[f"The account is enrolled in beta program {beta_id!r}."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This enrolls the account in a beta program. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.enroll_account_beta(beta_id)
        return serialize_api_response(
            {
                "message": "Account beta enrollment requested successfully",
                "id": beta_id,
            },
            account_pb2.AccountBetaEnrollResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"enroll Linode account in beta {beta_id}", _call
    )


def create_linode_account_oauth_client_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_create tool."""
    return Tool(
        name="linode_account_oauth_client_create",
        description=(
            "Creates an account OAuth client. The client secret is only "
            "shown once in the response. Pass dry_run=true to preview "
            "without creating the client."
        ),
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientCreateInput"),
    ), Capability.Admin


def create_linode_account_oauth_client_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_delete tool."""
    return Tool(
        name="linode_account_oauth_client_delete",
        description=(
            "Deletes an account OAuth client by client ID. "
            "Pass dry_run=true to preview without deleting."
        ),
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientDeleteInput"),
    ), Capability.Admin


def _required_nonempty_string_argument(
    arguments: dict[str, Any], name: str
) -> tuple[str | None, str | None]:
    raw_value = arguments.get(name)
    if raw_value is None:
        return None, f"{name} is required"
    if not isinstance(raw_value, str):
        return None, f"{name} must be a string"
    value = raw_value.strip()
    if not value:
        return None, f"{name} is required"
    return value, None


def _required_pathsafe_string(
    arguments: dict[str, Any], name: str
) -> tuple[str | None, str | None]:
    """Mirror Go's string-id parsers: absent -> "<name> is required"; present but
    not a non-blank string -> "<name> must be a non-empty string". Returns the
    ORIGINAL (un-stripped) value so the caller can run charset/whitespace checks
    on the caller-supplied text, matching Go which validates the raw value."""
    if name not in arguments:
        return None, f"{name} is required"
    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip():
        return None, f"{name} must be a non-empty string"
    return value, None


def _is_account_beta_id(value: str) -> bool:
    """True when every char is an ASCII letter, digit, underscore, or hyphen.

    Mirrors Go's isAccountBetaID so beta ids reject identically across languages.
    """
    return all(
        ("a" <= c <= "z") or ("A" <= c <= "Z") or ("0" <= c <= "9") or c in "_-"
        for c in value
    )


def _validate_account_oauth_client_id(value: str) -> str | None:
    if not _ACCOUNT_OAUTH_CLIENT_ID_PATTERN.fullmatch(value):
        return "client_id must contain only letters, numbers, underscores, or hyphens"
    return None


async def handle_linode_account_oauth_client_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_delete tool request."""
    client_id, client_id_error = _required_nonempty_string_argument(
        arguments, "client_id"
    )
    if client_id_error is not None or client_id is None:
        return error_response(client_id_error or "client_id is required")

    validation_error = _validate_account_oauth_client_id(client_id)
    if validation_error is not None:
        return error_response(validation_error)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_oauth_client_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/account/oauth-clients/{quote(client_id, safe='')}",
            None,
            side_effects=["The account OAuth client is deleted."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes an OAuth client. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_account_oauth_client(client_id)
        return serialize_api_response(
            {
                "message": "OAuth client deleted successfully",
                "client_id": client_id,
            },
            account_pb2.OAuthClientIDResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"delete Linode account OAuth client {client_id}", _call
    )


async def handle_linode_account_oauth_client_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_create tool request."""
    label, label_error = _required_nonempty_string_argument(arguments, "label")
    if label_error is not None or label is None:
        return error_response(label_error or "label is required")

    redirect_uri, redirect_uri_error = _required_nonempty_string_argument(
        arguments, "redirect_uri"
    )
    if redirect_uri_error is not None or redirect_uri is None:
        return error_response(redirect_uri_error or "redirect_uri is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_oauth_client_create",
            arguments.get("environment", ""),
            "POST",
            "/account/oauth-clients",
            None,
            request_body={"label": label, "redirect_uri": redirect_uri},
            side_effects=[
                (
                    "A new account OAuth client is created. The returned client "
                    "secret is shown once and cannot be retrieved later."
                )
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an OAuth client. The secret is only shown once. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        oauth_client = await client.create_account_oauth_client(label, redirect_uri)
        return serialize_api_response(
            {
                "message": "OAuth client created successfully",
                "warning": (
                    "IMPORTANT: The secret below is shown ONLY ONCE. Save it now"
                    " - it cannot be retrieved later."
                ),
                "client": oauth_client,
            },
            account_pb2.OAuthClientCreateWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"create Linode account OAuth client {label}", _call
    )


def create_linode_account_payment_method_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_method_create tool."""
    return Tool(
        name="linode_account_payment_method_create",
        description=(
            "Adds a payment method to the Linode account. "
            "Pass dry_run=true to preview without creating it."
        ),
        inputSchema=schema("linode.mcp.v1.AccountPaymentMethodCreateInput"),
    ), Capability.Admin


def _payment_method_create_body(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    payment_type, payment_type_error = _required_nonempty_string_argument(
        arguments, "type"
    )
    if payment_type_error is not None or payment_type is None:
        return None, payment_type_error or "type is required"
    if payment_type != "credit_card":
        return None, "type must be credit_card"

    payment_data = arguments.get("data")
    if payment_data is None:
        return None, "data is required"
    if not isinstance(payment_data, dict):
        return None, "data must be an object"

    is_default = arguments.get("is_default")
    if not isinstance(is_default, bool):
        return None, "is_default must be a boolean"

    return {
        "type": payment_type,
        "data": payment_data,
        "is_default": is_default,
    }, None


async def handle_linode_account_payment_method_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_create tool request."""
    request_body, validation_error = _payment_method_create_body(arguments)
    if validation_error is not None or request_body is None:
        return error_response(validation_error or "payment method body is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_payment_method_create",
            arguments.get("environment", ""),
            "POST",
            "/account/payment-methods",
            None,
            request_body={**request_body, "data": {"redacted": True}},
            side_effects=[
                (
                    "A new account payment method is created and may become the "
                    "default payment method."
                )
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a payment method. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        method = await client.create_account_payment_method(
            str(request_body["type"]),
            cast("dict[str, Any]", request_body["data"]),
            bool(request_body["is_default"]),
        )
        return serialize_api_response(
            {
                "message": "Payment method created successfully",
                "payment_method": method,
            },
            account_pb2.AccountPaymentMethodWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "create Linode account payment method", _call
    )


def create_linode_account_promo_credit_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_promo_credit_add tool."""
    return Tool(
        name="linode_account_promo_credit_add",
        description=(
            "Adds a promo credit to the Linode account. "
            "Pass dry_run=true to preview without applying the promo code."
        ),
        inputSchema=schema("linode.mcp.v1.AccountPromoCreditAddInput"),
    ), Capability.Admin


async def handle_linode_account_promo_credit_add(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_promo_credit_add tool request."""
    promo_code, promo_code_error = _required_pathsafe_string(arguments, "promo_code")
    if promo_code_error is not None or promo_code is None:
        return error_response(promo_code_error or "promo_code is required")
    if promo_code != promo_code.strip():
        return error_response(
            "promo_code must not include leading or trailing whitespace"
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_promo_credit_add",
            arguments.get("environment", ""),
            "POST",
            "/account/promo-codes",
            None,
            request_body={"promo_code": promo_code},
            side_effects=[
                "The promo code is applied to the account and may add account credit."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This applies a promo credit to the account. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.add_account_promo_credit(promo_code)
        return serialize_api_response(
            {
                "message": "Account promo credit applied successfully",
                "promo_code": promo_code,
            },
            account_pb2.AccountPromoResponse(),
        )

    return await execute_tool(cfg, arguments, "add Linode account promo credit", _call)


def create_linode_account_cancel_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_cancel tool."""
    return Tool(
        name="linode_account_cancel",
        description=(
            "Cancels the Linode account. "
            "Pass dry_run=true to preview without canceling."
        ),
        inputSchema=schema("linode.mcp.v1.AccountCancelInput"),
    ), Capability.Admin


async def handle_linode_account_cancel(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_cancel tool request."""
    comments = arguments.get("comments")
    if comments is not None and not isinstance(comments, str):
        return error_response("comments must be a string")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_cancel",
            arguments.get("environment", ""),
            "POST",
            "/account/cancel",
            None,
            request_body={"comments": comments} if comments is not None else None,
            side_effects=[
                "The Linode account is closed and all of its resources are removed."
            ],
            warnings=[
                (
                    "Account cancellation is permanent and irreversible; every "
                    "resource on the account is destroyed and access is lost."
                )
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This cancels the active account. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        cancel_info = await client.cancel_account(comments=comments)
        return serialize_api_response(
            {
                "message": "Account canceled successfully",
                "cancel_info": cancel_info,
            },
            account_pb2.AccountCancelWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "cancel Linode account", _call)


_ACCOUNT_SETTINGS_BOOLEAN_FIELDS = frozenset(
    {"backups_enabled", "managed", "network_helper"}
)
_ACCOUNT_SETTINGS_STRING_FIELDS = frozenset(
    {
        "interfaces_for_new_linodes",
        "longview_subscription",
        "maintenance_policy",
        "object_storage",
    }
)
_ACCOUNT_SETTINGS_UPDATE_FIELDS = tuple(
    sorted(_ACCOUNT_SETTINGS_BOOLEAN_FIELDS | _ACCOUNT_SETTINGS_STRING_FIELDS)
)
_ACCOUNT_SETTINGS_ALLOWED_ARGUMENTS = frozenset(
    {"environment", "confirm", PARAM_DRY_RUN, *_ACCOUNT_SETTINGS_UPDATE_FIELDS}
)

_ACCOUNT_USER_UPDATE_BOOLEAN_FIELDS = frozenset({"restricted"})
_ACCOUNT_USER_UPDATE_STRING_FIELDS = frozenset({"email", "new_username"})
_ACCOUNT_USER_UPDATE_LIST_FIELDS = frozenset({"ssh_keys"})
_ACCOUNT_USER_UPDATE_FIELDS = tuple(
    sorted(
        _ACCOUNT_USER_UPDATE_BOOLEAN_FIELDS
        | _ACCOUNT_USER_UPDATE_STRING_FIELDS
        | _ACCOUNT_USER_UPDATE_LIST_FIELDS
    )
)
_ACCOUNT_USER_UPDATE_ALLOWED_ARGUMENTS = frozenset(
    {
        "environment",
        "confirm",
        PARAM_DRY_RUN,
        "username",
        *_ACCOUNT_USER_UPDATE_FIELDS,
    }
)
_ACCOUNT_USER_USERNAME_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")


def _validate_account_user_update_username(
    value: object,
) -> tuple[str | None, str | None]:
    if value is None:
        return None, "username is required"
    if not isinstance(value, str):
        return None, "username must be a string"
    username = value.strip()
    if not username:
        return None, "username is required"
    if username != value or not _ACCOUNT_USER_USERNAME_PATTERN.fullmatch(username):
        return (
            None,
            "username must contain only letters, numbers, underscores, and hyphens",
        )
    return username, None


def _account_user_update_body_error(arguments: dict[str, Any]) -> str | None:
    unknown_fields = set(arguments) - _ACCOUNT_USER_UPDATE_ALLOWED_ARGUMENTS
    if unknown_fields:
        fields = ", ".join(sorted(unknown_fields))
        return f"Unsupported account user field(s): {fields}"

    for field in _ACCOUNT_USER_UPDATE_BOOLEAN_FIELDS:
        value = arguments.get(field)
        if value is not None and type(value) is not bool:
            return f"{field} must be a boolean"

    for field in _ACCOUNT_USER_UPDATE_STRING_FIELDS:
        value = arguments.get(field)
        if value is not None and not isinstance(value, str):
            return f"{field} must be a string"

    value = arguments.get("ssh_keys")
    if value is not None:
        if not isinstance(value, list):
            return "ssh_keys must be a list of strings"
        ssh_keys = cast("list[object]", value)
        if any(not isinstance(item, str) for item in ssh_keys):
            return "ssh_keys must be a list of strings"

    return None


def _account_user_update_body(arguments: dict[str, Any]) -> dict[str, Any]:
    body = {
        key: arguments[key]
        for key in _ACCOUNT_USER_UPDATE_FIELDS
        if key != "new_username" and arguments.get(key) is not None
    }
    # new_username is the tool arg; the PUT body renames the user via "username".
    if arguments.get("new_username") is not None:
        body["username"] = arguments["new_username"]
    return body


def create_linode_account_settings_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_settings_update tool."""
    return Tool(
        name="linode_account_settings_update",
        description=(
            "Updates Linode account-wide settings. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema=schema("linode.mcp.v1.AccountSettingsUpdateInput"),
    ), Capability.Admin


def _account_settings_update_body_error(arguments: dict[str, Any]) -> str | None:
    unknown_fields = set(arguments) - _ACCOUNT_SETTINGS_ALLOWED_ARGUMENTS
    if unknown_fields:
        fields = ", ".join(sorted(unknown_fields))
        return f"Unsupported account settings field(s): {fields}"

    for field in _ACCOUNT_SETTINGS_BOOLEAN_FIELDS:
        value = arguments.get(field)
        if value is not None and type(value) is not bool:
            return f"{field} must be a boolean"

    for field in _ACCOUNT_SETTINGS_STRING_FIELDS:
        value = arguments.get(field)
        if value is not None and not isinstance(value, str):
            return f"{field} must be a string"

    return None


def _account_settings_update_body(arguments: dict[str, Any]) -> dict[str, Any]:
    return {
        key: arguments[key]
        for key in _ACCOUNT_SETTINGS_UPDATE_FIELDS
        if arguments.get(key) is not None
    }


async def handle_linode_account_settings_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_settings_update tool request."""
    validation_error = _account_settings_update_body_error(arguments)
    if validation_error is not None:
        return error_response(validation_error)

    update_fields = _account_settings_update_body(arguments)
    if not update_fields:
        return error_response("At least one account settings field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_settings_update",
            arguments.get("environment", ""),
            "PUT",
            "/account/settings",
            None,
            request_body=update_fields,
            side_effects=["Linode account settings are updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates account-wide settings. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_account_settings(**update_fields)
        return serialize_api_response(
            {
                "message": "Account settings updated successfully",
                "settings": result,
            },
            account_pb2.AccountSettingsWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode account settings", _call)


def create_linode_account_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_update tool."""
    return Tool(
        name="linode_account_update",
        description=(
            "Updates Linode account contact and billing-address information. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema=schema("linode.mcp.v1.AccountUpdateInput"),
    ), Capability.Admin


async def handle_linode_account_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_update tool request."""
    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_account()

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_update",
            "PUT",
            "/account",
            _fetch,
        )

    if not arguments.get("confirm"):
        return error_response(
            "This updates account billing/contact information. Set confirm=true to "
            "proceed."
        )

    update_fields = {
        key: arguments.get(key)
        for key in (
            "first_name",
            "last_name",
            "email",
            "company",
            "address_1",
            "address_2",
            "city",
            "state",
            "zip",
            "country",
            "phone",
            "tax_id",
        )
        if arguments.get(key) is not None
    }
    if not update_fields:
        return error_response("At least one account field is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        account = await client.put_raw("/account", update_fields)
        return serialize_api_response(
            {
                "message": "Account updated successfully",
                "account": account,
            },
            account_pb2.AccountWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode account", _call)


def create_linode_account_user_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_update tool."""
    return Tool(
        name="linode_account_user_update",
        description=(
            "Updates an account user by username. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema=schema("linode.mcp.v1.AccountUserUpdateInput"),
    ), Capability.Admin


async def handle_linode_account_user_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_update tool request."""
    current_username, username_error = _validate_account_user_update_username(
        arguments.get("username")
    )
    if username_error is not None or current_username is None:
        return error_response(username_error or "username is required")

    validation_error = _account_user_update_body_error(arguments)
    if validation_error is not None:
        return error_response(validation_error)

    update_fields = _account_user_update_body(arguments)
    if not update_fields:
        return error_response("At least one account user field is required")

    if is_dry_run(arguments):
        encoded_username = quote(current_username, safe="")
        return build_dry_run_response(
            "linode_account_user_update",
            arguments.get("environment", ""),
            "PUT",
            f"/account/users/{encoded_username}",
            None,
            request_body=update_fields,
            side_effects=["The account user is updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an account user. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_account_user(current_username, **update_fields)
        return serialize_api_response(
            {
                "message": "Account user updated successfully",
                "user": result,
            },
            account_user_pb2.AccountUserWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode account user", _call)


def _is_region_id(value: str) -> bool:
    """Return True when every char is a-z, 0-9, or hyphen.

    Mirrors Go's isAccountAvailabilityRegionSlug so region ids reject identically
    across languages (Go accepts a no-hyphen slug like "useast", so the previous
    ">= 2 hyphen parts" rule was stricter than the API contract).
    """
    return all(("a" <= c <= "z") or ("0" <= c <= "9") or c == "-" for c in value)


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


def create_linode_account_beta_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_beta_get tool."""
    return Tool(
        name="linode_account_beta_get",
        description="Gets an enrolled Beta program on the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountBetaGetInput"),
    ), Capability.Read


async def handle_linode_account_beta_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_get tool request."""
    beta_id, beta_id_error = _required_pathsafe_string(arguments, "beta_id")
    if beta_id_error is not None or beta_id is None:
        return error_response(beta_id_error or "beta_id is required")
    if beta_id != beta_id.strip() or not _is_account_beta_id(beta_id):
        return error_response(
            "beta_id must contain only letters, numbers, underscores, and hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_beta(beta_id),
            account_beta_program_pb2.AccountBetaProgram(),
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account beta {beta_id}", _call
    )


def create_linode_account_child_account_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_account_get tool."""
    return Tool(
        name="linode_account_child_account_get",
        description="Gets a child account by EUUID.",
        inputSchema=schema("linode.mcp.v1.AccountChildAccountGetInput"),
    ), Capability.Read


async def handle_linode_account_child_account_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_account_get tool request."""
    euuid, euuid_error = _required_pathsafe_string(arguments, "euuid")
    if euuid_error is not None or euuid is None:
        return error_response(euuid_error or "euuid is required")
    if euuid != euuid.strip() or "/" in euuid or "?" in euuid or ".." in euuid:
        return error_response(
            "euuid must not contain path separators, "
            "query separators, or traversal segments"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_child_account(euuid), account_pb2.ChildAccount()
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode child account {euuid}", _call
    )


def create_linode_account_service_transfer_accept_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfer_accept tool."""
    return Tool(
        name="linode_account_service_transfer_accept",
        description=(
            "Accepts an account service transfer request by token. "
            "Pass dry_run=true to preview without accepting the transfer."
        ),
        inputSchema=schema("linode.mcp.v1.AccountServiceTransferAcceptInput"),
    ), Capability.Admin


async def handle_linode_account_service_transfer_accept(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_accept tool request."""
    token, validation_error = _validate_service_transfer_token(arguments.get("token"))
    if validation_error is not None:
        return error_response(validation_error)
    token = cast("str", token)

    if is_dry_run(arguments):
        encoded_token = quote(token, safe="")
        return build_dry_run_response(
            "linode_account_service_transfer_accept",
            arguments.get("environment", ""),
            "POST",
            f"/account/service-transfers/{encoded_token}/accept",
            None,
            side_effects=["The service transfer is accepted for the Linode account."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This accepts an account service transfer. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.accept_account_service_transfer(token)
        return serialize_api_response(
            {
                "message": "Account service transfer accepted successfully",
                "token": token,
            },
            account_pb2.AccountServiceTransferActionResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"accept Linode account service transfer {token}", _call
    )


def create_linode_account_service_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfer_get tool."""
    return Tool(
        name="linode_account_service_transfer_get",
        description="Gets an account service transfer request by token.",
        inputSchema=schema("linode.mcp.v1.AccountServiceTransferGetInput"),
    ), Capability.Read


async def handle_linode_account_service_transfer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_get tool request."""
    token, message = _validate_service_transfer_token(arguments.get("token"))
    if token is None:
        return error_response(message or "token is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_service_transfer(token),
            account_service_transfer_pb2.AccountEntityTransfer(),
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account service transfer {token}", _call
    )


def create_linode_account_service_transfer_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfer_delete tool."""
    return Tool(
        name="linode_account_service_transfer_delete",
        description=(
            "Cancels an account service transfer request by token. "
            "Pass dry_run=true to preview without canceling."
        ),
        inputSchema=schema("linode.mcp.v1.AccountServiceTransferDeleteInput"),
    ), Capability.Admin


async def handle_linode_account_service_transfer_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_delete tool request."""
    token, message = _validate_service_transfer_token(arguments.get("token"))
    if token is None:
        return error_response(message or "token is required")

    encoded_token = quote(token, safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_service_transfer_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/account/service-transfers/{encoded_token}",
            None,
            side_effects=["The selected account service transfer is canceled."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This cancels an account service transfer. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_account_service_transfer(token)
        return serialize_api_response(
            {
                "message": "Account service transfer canceled successfully",
                "token": token,
            },
            account_pb2.AccountServiceTransferActionResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"cancel Linode account service transfer {token}", _call
    )


def create_linode_account_oauth_client_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_get tool."""
    return Tool(
        name="linode_account_oauth_client_get",
        description="Gets an OAuth client by client ID.",
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientGetInput"),
    ), Capability.Read


async def handle_linode_account_oauth_client_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_get tool request."""
    client_id, error = _validated_oauth_client_id(arguments)
    if error is not None or client_id is None:
        return error_response(error or "client_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_oauth_client(client_id), account_pb2.OAuthClient()
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account OAuth client {client_id}", _call
    )


def create_linode_account_oauth_client_secret_reset_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_secret_reset tool."""
    return Tool(
        name="linode_account_oauth_client_secret_reset",
        description=(
            "Resets an OAuth client secret. The new secret is only shown once "
            "in the response. Pass dry_run=true to preview without resetting."
        ),
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientSecretResetInput"),
    ), Capability.Admin


def _validated_oauth_client_id(
    arguments: dict[str, Any], name: str = "client_id"
) -> tuple[str | None, str | None]:
    raw_client_id = arguments.get(name)
    if raw_client_id is None:
        return None, f"{name} is required"
    if not isinstance(raw_client_id, str):
        return None, f"{name} must be a string"

    client_id = raw_client_id.strip()
    if not client_id:
        return None, f"{name} is required"
    if "/" in client_id or "?" in client_id or ".." in client_id:
        return None, f"{name} must not contain '/', '?', or '..'"
    return client_id, None


async def handle_linode_account_oauth_client_secret_reset(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_secret_reset tool request."""
    client_id, error = _validated_oauth_client_id(arguments)
    if error is not None or client_id is None:
        return error_response(error or "client_id is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_oauth_client_secret_reset",
            arguments.get("environment", ""),
            "POST",
            f"/account/oauth-clients/{quote(client_id, safe='')}/reset-secret",
            None,
            side_effects=[
                (
                    "The OAuth client secret is reset. The replacement secret is "
                    "shown once and cannot be retrieved later."
                )
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This resets an OAuth client secret. The new secret is only shown once. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        secret = await client.reset_account_oauth_client_secret(client_id)
        return serialize_api_response(
            {
                "message": "OAuth client secret reset successfully",
                "warning": (
                    "IMPORTANT: The new secret below is shown ONLY ONCE. Save it"
                    " now - it cannot be retrieved later."
                ),
                "client_id": client_id,
                "secret": secret,
            },
            account_pb2.OAuthClientSecretResetWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"reset Linode account OAuth client secret {client_id}", _call
    )


def create_linode_account_oauth_client_thumbnail_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_thumbnail_get tool."""
    return Tool(
        name="linode_account_oauth_client_thumbnail_get",
        description="Gets an OAuth client's thumbnail by client ID.",
        inputSchema=schema("linode.mcp.v1.AccountOAuthClientThumbnailGetInput"),
    ), Capability.Read


async def handle_linode_account_oauth_client_thumbnail_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_thumbnail_get tool request."""
    client_id, error = _validated_oauth_client_id(arguments)
    if error is not None or client_id is None:
        return error_response(error or "client_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_account_oauth_client_thumbnail(client_id)
        return serialize_api_response(
            {"client_id": client_id, **raw},
            oauth_client_thumbnail_pb2.OAuthClientThumbnail(),
        )

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve Linode account OAuth client thumbnail {client_id}",
        _call,
    )


def create_linode_account_invoice_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_invoice_get tool."""
    return Tool(
        name="linode_account_invoice_get",
        description="Gets an invoice on the Linode account by ID.",
        inputSchema=schema("linode.mcp.v1.AccountInvoiceGetInput"),
    ), Capability.Read


async def handle_linode_account_invoice_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_invoice_get tool request."""
    raw_invoice_id = arguments.get("invoice_id")
    if raw_invoice_id is None:
        return error_response("invoice_id is required")
    if not isinstance(raw_invoice_id, int) or isinstance(raw_invoice_id, bool):
        return error_response("invoice_id must be an integer")
    if raw_invoice_id < 1:
        return error_response("invoice_id must be at least 1")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_invoice(raw_invoice_id),
            account_pb2.AccountInvoice(),
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account invoice {raw_invoice_id}", _call
    )


def create_linode_account_payment_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_get tool."""
    return Tool(
        name="linode_account_payment_get",
        description="Gets a payment on the Linode account by ID.",
        inputSchema=schema("linode.mcp.v1.AccountPaymentGetInput"),
    ), Capability.Read


async def handle_linode_account_payment_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_get tool request."""
    raw_payment_id = arguments.get("payment_id")
    if raw_payment_id is None:
        return error_response("payment_id is required")
    if not isinstance(raw_payment_id, int) or isinstance(raw_payment_id, bool):
        return error_response("payment_id must be an integer")
    if raw_payment_id < 1:
        return error_response("payment_id must be at least 1")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_payment(raw_payment_id),
            account_pb2.AccountPayment(),
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account payment {raw_payment_id}", _call
    )


def create_linode_account_payment_method_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_method_get tool."""
    return Tool(
        name="linode_account_payment_method_get",
        description="Gets a payment method on the Linode account by ID.",
        inputSchema=schema("linode.mcp.v1.AccountPaymentMethodGetInput"),
    ), Capability.Read


async def handle_linode_account_payment_method_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_get tool request."""
    raw_payment_method_id, error = required_int_id(arguments, "payment_method_id")
    if raw_payment_method_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_payment_method(raw_payment_method_id),
            account_pb2.AccountPaymentMethod(),
        )

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve Linode account payment method {raw_payment_method_id}",
        _call,
    )


def create_linode_account_payment_method_make_default_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_method_make_default tool."""
    return Tool(
        name="linode_account_payment_method_make_default",
        description=(
            "Sets a payment method as the default for the Linode account. "
            "Pass dry_run=true to preview without changing the default."
        ),
        inputSchema=schema("linode.mcp.v1.AccountPaymentMethodMakeDefaultInput"),
    ), Capability.Admin


async def handle_linode_account_payment_method_make_default(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_make_default tool request."""
    raw_payment_method_id, error = required_int_id(arguments, "payment_method_id")
    if raw_payment_method_id is None:
        return error_response(error)

    encoded_payment_method_id = quote(str(raw_payment_method_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_payment_method_make_default",
            arguments.get("environment", ""),
            "POST",
            f"/account/payment-methods/{encoded_payment_method_id}/make-default",
            None,
            request_body={},
            side_effects=["The selected account payment method becomes the default."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This changes the default payment method. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.make_account_payment_method_default(raw_payment_method_id)
        return serialize_api_response(
            {
                "message": "Payment method set as default successfully",
                "payment_method_id": raw_payment_method_id,
            },
            account_pb2.AccountPaymentMethodIDResponse(),
        )

    return await execute_tool(
        cfg,
        arguments,
        f"set Linode account payment method {raw_payment_method_id} as default",
        _call,
    )


def create_linode_account_login_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_login_get tool."""
    return Tool(
        name="linode_account_login_get",
        description="Gets an account login by login ID.",
        inputSchema=schema("linode.mcp.v1.AccountLoginGetInput"),
    ), Capability.Read


async def handle_linode_account_login_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_login_get tool request."""
    raw_login_id = arguments.get("login_id")
    if raw_login_id is None:
        return error_response("login_id is required")
    if not isinstance(raw_login_id, int) or isinstance(raw_login_id, bool):
        return error_response("login_id must be an integer")
    if raw_login_id < 1:
        return error_response("login_id must be at least 1")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_login(raw_login_id), account_pb2.AccountLogin()
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account login {raw_login_id}", _call
    )


def create_linode_account_user_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_delete tool."""
    return Tool(
        name="linode_account_user_delete",
        description=(
            "Deletes a user from the Linode account by username. "
            "Pass dry_run=true to preview without deleting."
        ),
        inputSchema=schema("linode.mcp.v1.AccountUserDeleteInput"),
    ), Capability.Admin


def _validate_account_user_delete_username(
    value: object,
) -> tuple[str | None, str | None]:
    """Validate an account username supplied by the delete tool."""
    if value is None:
        return None, "username is required"
    if not isinstance(value, str):
        return None, "username must be a string"

    username = value.strip()
    if not username:
        return None, "username is required"
    if username != value or not _ACCOUNT_USERNAME_PATTERN.fullmatch(username):
        return (
            None,
            "username must contain only letters, numbers, underscores, or hyphens",
        )
    return username, None


async def handle_linode_account_user_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_delete tool request."""
    username, message = _validate_account_user_delete_username(
        arguments.get("username")
    )
    if username is None:
        return error_response(message or "username is required")

    encoded_username = quote(username, safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_user_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/account/users/{encoded_username}",
            None,
            side_effects=["The selected account user is deleted."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes an account user. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_account_user(username)
        return serialize_api_response(
            {
                "message": "Account user deleted successfully",
                "username": username,
            },
            account_user_pb2.AccountUserDeleteResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"delete Linode account user {username}", _call
    )


def create_linode_account_user_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_get tool."""
    return Tool(
        name="linode_account_user_get",
        description="Gets an account user by username.",
        inputSchema=schema("linode.mcp.v1.AccountUserGetInput"),
    ), Capability.Read


async def handle_linode_account_user_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_get tool request."""
    raw_username = arguments.get("username")
    if raw_username is None:
        return error_response("username is required")
    if not isinstance(raw_username, str):
        return error_response("username must be a string")

    username = raw_username.strip()
    if not username:
        return error_response("username is required")
    if username != raw_username or not _ACCOUNT_USERNAME_PATTERN.fullmatch(username):
        return error_response(
            "username must contain only letters, numbers, underscores, or hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_user(username), account_user_pb2.AccountUser()
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account user {username}", _call
    )


def create_linode_account_user_grants_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_grants_get tool."""
    return Tool(
        name="linode_account_user_grants_get",
        description="Lists grants for an account user by username.",
        inputSchema=schema("linode.mcp.v1.AccountUserGrantsGetInput"),
    ), Capability.Read


async def handle_linode_account_user_grants_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_grants_get tool request."""
    raw_username = arguments.get("username")
    if raw_username is None:
        return error_response("username is required")
    if not isinstance(raw_username, str):
        return error_response("username must be a string")

    username = raw_username.strip()
    if not username:
        return error_response("username is required")
    if username != raw_username or not _ACCOUNT_USERNAME_PATTERN.fullmatch(username):
        return error_response(
            "username must contain only letters, numbers, underscores, or hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_user_grants(username),
            account_user_pb2.AccountUserGrants(),
        )

    return await execute_tool(
        cfg, arguments, f"list Linode account user grants for {username}", _call
    )


def create_linode_account_availability_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_availability_list tool."""
    return Tool(
        name="linode_account_availability_list",
        description="Lists available Linode services for the account.",
        inputSchema=schema("linode.mcp.v1.AccountAvailabilityListInput"),
    ), Capability.Read


async def handle_linode_account_availability_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_availability_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_availability(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_availabilities",
            account_availability_pb2.AccountAvailabilityListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account availability", _call)


def create_linode_account_availability_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_availability_get tool."""
    return Tool(
        name="linode_account_availability_get",
        description=(
            "Gets available Linode services for the account in a specific region."
        ),
        inputSchema=schema("linode.mcp.v1.AccountAvailabilityGetInput"),
    ), Capability.Read


async def handle_linode_account_availability_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_availability_get tool request."""
    region_id, region_id_error = _required_pathsafe_string(arguments, "region_id")
    if region_id_error is not None or region_id is None:
        return error_response(region_id_error or "region_id is required")
    if not _is_region_id(region_id):
        return error_response(
            "region_id must be a lowercase region slug containing only "
            "letters, numbers, and hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_account_availability(region_id),
            account_availability_pb2.AccountAvailability(),
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account availability for {region_id}", _call
    )


def create_linode_account_beta_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_beta_list tool."""
    return Tool(
        name="linode_account_beta_list",
        description="Lists enrolled Beta programs for the account.",
        inputSchema=schema("linode.mcp.v1.AccountBetaListInput"),
    ), Capability.Read


async def handle_linode_account_beta_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_betas(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_betas",
            account_beta_program_pb2.AccountBetaProgramListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode account betas", _call)


def create_linode_beta_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_beta_list tool."""
    return Tool(
        name="linode_beta_list",
        description="Lists available Beta programs.",
        inputSchema=schema("linode.mcp.v1.BetaListInput"),
    ), Capability.Read


async def handle_linode_beta_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_beta_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_betas(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "betas",
            beta_program_pb2.BetaProgramListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode beta programs", _call)


def create_linode_account_child_account_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_account_list tool."""
    return Tool(
        name="linode_account_child_account_list",
        description="Lists child accounts for the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountChildAccountListInput"),
    ), Capability.Read


async def handle_linode_account_child_account_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_account_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_child_accounts(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "account_child_accounts",
            account_pb2.ChildAccountListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode account child accounts", _call
    )


def create_linode_account_service_transfer_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfer_list tool."""
    return Tool(
        name="linode_account_service_transfer_list",
        description="Lists service transfers for the Linode account.",
        inputSchema=schema("linode.mcp.v1.AccountServiceTransferListInput"),
    ), Capability.Read


async def handle_linode_account_service_transfer_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_account_service_transfers(
            page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "account_service_transfers",
            account_service_transfer_pb2.AccountServiceTransferListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode account service transfers", _call
    )


def create_linode_account_child_account_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_account_token_create tool."""
    return Tool(
        name="linode_account_child_account_token_create",
        description=(
            "Creates a proxy user token for a child account. "
            "Pass dry_run=true to preview without creating a token."
        ),
        inputSchema=schema("linode.mcp.v1.AccountChildAccountTokenCreateInput"),
    ), Capability.Admin


def _validate_child_account_euuid(value: Any) -> str | None:
    """Validate a child account EUUID tool argument."""
    if value is None:
        return None
    if not isinstance(value, str):
        return None

    euuid = value.strip()
    if not euuid:
        return None
    if _CHILD_ACCOUNT_EUUID_PATTERN.fullmatch(euuid) is None:
        return None
    return euuid


async def handle_linode_account_child_account_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_account_token_create tool request."""
    euuid = _validate_child_account_euuid(arguments.get("euuid"))
    if euuid is None:
        return error_response("euuid must match the child account EUUID format")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_child_account_token_create",
            arguments.get("environment", ""),
            "POST",
            f"/account/child-accounts/{euuid}/token",
            None,
            side_effects=[
                "A proxy user token is created for the selected child account."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a proxy user token for a child account. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.create_account_child_account_token(euuid)
        return serialize_api_response(
            {
                "message": "Child account proxy token created successfully",
                "token": token,
            },
            account_pb2.AccountChildAccountTokenWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "create Linode account child account proxy token", _call
    )


def create_linode_tag_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_tag_list tool."""
    return Tool(
        name="linode_tag_list",
        description="Lists tags on the Linode account.",
        inputSchema=schema("linode.mcp.v1.TagListInput"),
    ), Capability.Read


async def handle_linode_tag_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_tag_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_tags(page=page, page_size=page_size)
        return serialize_list_response(raw, "tags", tag_pb2.TagListResponse())

    return await execute_tool(cfg, arguments, "list Linode account tags", _call)


def create_linode_tag_object_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_tag_object_list tool."""
    return Tool(
        name="linode_tag_object_list",
        description="Lists objects assigned to a Linode account tag.",
        inputSchema=schema("linode.mcp.v1.TaggedObjectListInput"),
    ), Capability.Read


def _tag_label_path_error(tag_label: str) -> list[TextContent] | None:
    """Reject path-unsafe tag labels locally (mirrors Go tagLabelArgFromTool)."""
    if "?" in tag_label or "#" in tag_label or ".." in tag_label:
        return error_response("tag_label must not contain '?', '#', or '..'")
    return None


async def handle_linode_tag_object_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_tag_object_list tool request."""
    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")
    path_error = _tag_label_path_error(tag_label)
    if path_error is not None:
        return path_error

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_tagged_objects(
            tag_label, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw, "tagged_objects", tag_pb2.TaggedObjectListResponse()
        )

    return await execute_tool(cfg, arguments, "list tagged objects", _call)


def create_linode_tag_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_tag_create tool."""
    return Tool(
        name="linode_tag_create",
        description="Creates a Linode account tag and optionally assigns resources.",
        inputSchema=schema("linode.mcp.v1.TagCreateInput"),
    ), Capability.Write


def _optional_int_list_argument(
    arguments: dict[str, Any], name: str
) -> list[int] | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, list):
        raise TypeError(f"{name} must be a list of integers")
    values: list[int] = []
    for item in cast("list[object]", value):
        if not isinstance(item, int) or isinstance(item, bool):
            raise TypeError(f"{name} must be a list of integers")
        if item < 1:
            raise ValueError(f"{name} must contain positive integers")
        values.append(item)
    if not values:
        return None
    return values


async def handle_linode_tag_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_tag_create tool request."""
    if is_dry_run(arguments):
        label = arguments.get("label")
        if not isinstance(label, str) or not label.strip():
            return error_response("label is required")
        return build_dry_run_response(
            "linode_tag_create",
            arguments.get("environment", ""),
            "POST",
            "/tags",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response("This creates a Linode tag. Set confirm=true to proceed.")

    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return error_response("label is required")

    try:
        resource_ids = {
            name: _optional_int_list_argument(arguments, name)
            for name in ("domains", "linodes", "nodebalancers", "volumes")
        }
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        tag = await client.create_tag(label.strip(), **resource_ids)
        return serialize_api_response(
            {
                "message": f"Tag '{raw_str(tag, 'label')}' created successfully",
                "tag": tag,
            },
            tag_pb2.TagWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Linode tag", _call)


def create_linode_tag_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_tag_delete tool."""
    return Tool(
        name="linode_tag_delete",
        description="Deletes a Linode account tag by label." + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.TagDeleteInput"),
    ), Capability.Destroy


def _tag_delete_dependency_walk(state: Any) -> DryRunDetails:
    """Phase 2 Tier A walk for tag delete. Deleting a tag removes it from
    every object that carries it; the objects survive, so each is a removed
    dependency. State-only: the tagged objects come from the fetched page,
    no extra API call. The warning count still comes from the page envelope's
    results field (the API's total across all pages), so a truncated first
    page cannot understate the blast radius; a second warning says how many
    were itemized. Mirrors the Go tagDeleteDependencyWalk.
    """
    page = cast("dict[str, Any]", state) if isinstance(state, dict) else {}
    raw_objects = page.get("data", [])
    tagged_items = (
        cast("list[object]", raw_objects) if isinstance(raw_objects, list) else []
    )
    objects = [
        cast("dict[str, Any]", obj) for obj in tagged_items if isinstance(obj, dict)
    ]

    dependencies: list[dict[str, Any]] = []
    for tagged in objects:
        dependency: dict[str, Any] = {
            "kind": str(tagged.get("type") or "resource"),
            "action": "removed",
            "note": "Loses this tag; the resource itself is not deleted.",
        }
        data = tagged.get("data")
        if isinstance(data, dict) and "id" in data:
            dependency["id"] = cast("dict[str, Any]", data)["id"]
        dependencies.append(dependency)

    details: DryRunDetails = {}
    if dependencies:
        total = len(dependencies)
        raw_results = page.get("results")
        if isinstance(raw_results, int) and not isinstance(raw_results, bool):
            total = max(total, raw_results)

        warnings = [
            (
                f"Deleting this tag removes it from {total} tagged "
                "object(s); the objects are not deleted."
            )
        ]
        if total > len(dependencies):
            warnings.append(
                f"Only the first {len(dependencies)} tagged object(s) are "
                "itemized in this preview."
            )

        details["dependencies"] = dependencies
        details["warnings"] = warnings
    return details


async def _account_tag_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, tag_label: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.list_tagged_objects(tag_label)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_tag(tag_label)
        return serialize_api_response(
            {"message": f"Tag '{tag_label}' deleted successfully"},
            common_pb2.MessageResponse(),
        )

    async def _ts_walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _tag_delete_dependency_walk(state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_tag_delete",
        method="DELETE",
        path=f"/tags/{tag_label}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Tag"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_tag_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_tag_delete tool request."""
    # Every branch needs a valid tag_label, so validate it once up front.
    tag_label_raw = arguments.get("tag_label")
    if not isinstance(tag_label_raw, str) or not tag_label_raw.strip():
        return error_response("tag_label is required")
    tag_label = tag_label_raw
    path_error = _tag_label_path_error(tag_label)
    if path_error is not None:
        return path_error

    two_stage = await _account_tag_delete_two_stage(arguments, cfg, tag_label)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.list_tagged_objects(tag_label)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _tag_delete_dependency_walk(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_tag_delete",
            "DELETE",
            f"/tags/{tag_label}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("confirm must be true to delete a tag")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_tag(tag_label)
        return serialize_api_response(
            {"message": f"Tag '{tag_label}' deleted successfully"},
            common_pb2.MessageResponse(),
        )

    return await execute_tool(cfg, arguments, "delete Linode tag", _call)


def create_linode_support_ticket_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_create tool."""
    return Tool(
        name="linode_support_ticket_create",
        description="Opens a Linode support ticket.",
        inputSchema=schema("linode.mcp.v1.SupportTicketCreateInput"),
    ), Capability.Write


def _required_string_argument(arguments: dict[str, Any], name: str) -> str:
    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    return value.strip()


def _optional_string_argument(arguments: dict[str, Any], name: str) -> str | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} must be a non-empty string")
    return value.strip()


def _optional_bool_argument(arguments: dict[str, Any], name: str) -> bool | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, bool):
        raise TypeError(f"{name} must be a boolean")
    return value


def _support_ticket_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive integer ticket_id, or return an error response."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")
    return ticket_id


# The file types the ticket-attachment endpoint documents as accepted.
# Rejecting other extensions BEFORE the file read also keeps arbitrary local
# files (keys, configs) from being uploaded to a ticket. The message matches
# the Go handler exactly so the behavior fixture pins it cross-language.
_ATTACHMENT_EXTENSIONS = frozenset(
    {
        ".gif",
        ".jpg",
        ".jpeg",
        ".pjpg",
        ".pjpeg",
        ".tif",
        ".tiff",
        ".png",
        ".pdf",
        ".txt",
    }
)
_ATTACHMENT_EXTENSION_MESSAGE = (
    "file must have an accepted extension: "
    ".gif, .jpg, .jpeg, .pjpg, .pjpeg, .tif, .tiff, .png, .pdf, or .txt"
)


def _attachment_file(arguments: dict[str, Any]) -> str | list[TextContent]:
    """Parse the attachment file path, or return an error response."""
    file = arguments.get("file")
    if not isinstance(file, str) or not file.strip():
        return error_response("file is required")
    file_path = file.strip()
    if not Path(file_path).is_absolute():
        return error_response("file must be a local, absolute path")
    if Path(file_path).suffix.lower() not in _ATTACHMENT_EXTENSIONS:
        return error_response(_ATTACHMENT_EXTENSION_MESSAGE)
    return file_path


async def handle_linode_support_ticket_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_create tool request."""
    if is_dry_run(arguments):
        try:
            _required_string_argument(arguments, "summary")
            _required_string_argument(arguments, "description")
        except (TypeError, ValueError) as exc:
            return error_response(str(exc))
        summary = arguments.get("summary")
        effect = (
            f"A new support ticket {summary!r} will be opened."
            if summary
            else "A new support ticket will be opened."
        )
        return build_dry_run_response(
            "linode_support_ticket_create",
            arguments.get("environment", ""),
            "POST",
            "/support/tickets",
            None,
            side_effects=[effect],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a support ticket. Set confirm=true to proceed."
        )

    try:
        summary = _required_string_argument(arguments, "summary")
        description = _required_string_argument(arguments, "description")
        ticket_fields: dict[str, Any] = {
            "bucket": _optional_string_argument(arguments, "bucket"),
            "database_id": _optional_int_argument(arguments, "database_id", 1),
            "domain_id": _optional_int_argument(arguments, "domain_id", 1),
            "firewall_id": _optional_int_argument(arguments, "firewall_id", 1),
            "linode_id": _optional_int_argument(arguments, "linode_id", 1),
            "lkecluster_id": _optional_int_argument(arguments, "lkecluster_id", 1),
            "longviewclient_id": _optional_int_argument(
                arguments, "longviewclient_id", 1
            ),
            "managed_issue": _optional_bool_argument(arguments, "managed_issue"),
            "nodebalancer_id": _optional_int_argument(arguments, "nodebalancer_id", 1),
            "region": _optional_string_argument(arguments, "region"),
            "severity": _optional_int_argument(arguments, "severity", 1, 3),
            "vlan": _optional_string_argument(arguments, "vlan"),
            "volume_id": _optional_int_argument(arguments, "volume_id", 1),
            "vpc_id": _optional_int_argument(arguments, "vpc_id", 1),
        }
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ticket = await client.create_support_ticket(
            summary, description, **ticket_fields
        )
        return serialize_api_response(
            {"message": "Support ticket opened successfully", "ticket": ticket},
            support_ticket_pb2.SupportTicketWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "open Linode support ticket", _call)


def create_linode_managed_credential_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credential_get tool."""
    return Tool(
        name="linode_managed_credential_get",
        description=(
            "Gets a Linode Managed credential by ID. This account-level "
            "managed credential metadata requires admin capability. Pass "
            "dry_run=true to preview the request without retrieving it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedCredentialGetInput"),
    ), Capability.Admin


async def handle_linode_managed_credential_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_get tool request."""
    credential_id = arguments.get("credential_id")
    if (
        not isinstance(credential_id, int)
        or isinstance(credential_id, bool)
        or credential_id < 1
    ):
        return error_response("credential_id must be a positive integer")
    validated_credential_id = credential_id

    if is_dry_run(arguments):
        encoded_credential_id = quote(str(validated_credential_id), safe="")
        return build_dry_run_response(
            "linode_managed_credential_get",
            arguments.get("environment", ""),
            "GET",
            f"/managed/credentials/{encoded_credential_id}",
            None,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_managed_credential(validated_credential_id),
            managed_pb2.ManagedCredential(),
        )

    return await execute_tool(cfg, arguments, "get Linode Managed credential", _call)


def create_linode_managed_contact_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_list tool."""
    return Tool(
        name="linode_managed_contact_list",
        description="Lists Managed contacts on the Linode account.",
        inputSchema=schema("linode.mcp.v1.ManagedContactListInput"),
    ), Capability.Read


async def handle_linode_managed_contact_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contact_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_managed_contacts(page=page, page_size=page_size)
        return serialize_list_response(
            raw, "managed_contacts", managed_pb2.ManagedContactListResponse()
        )

    return await execute_tool(cfg, arguments, "list Linode Managed contacts", _call)


def create_linode_managed_issue_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_issue_list tool."""
    return Tool(
        name="linode_managed_issue_list",
        description="Lists open Managed issues on the Linode account.",
        inputSchema=schema("linode.mcp.v1.ManagedIssueListInput"),
    ), Capability.Read


async def handle_linode_managed_issue_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_issue_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_managed_issues(page=page, page_size=page_size)
        return serialize_list_response(
            raw, "managed_issues", managed_issue_pb2.ManagedIssueListResponse()
        )

    return await execute_tool(cfg, arguments, "list Linode Managed issues", _call)


def create_linode_managed_credential_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credential_list tool."""
    return Tool(
        name="linode_managed_credential_list",
        description="Lists Managed credentials on the Linode account.",
        inputSchema=schema("linode.mcp.v1.ManagedCredentialListInput"),
    ), Capability.Read


async def handle_linode_managed_credential_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_managed_credentials(page=page, page_size=page_size)
        return serialize_list_response(
            raw, "managed_credentials", managed_pb2.ManagedCredentialListResponse()
        )

    return await execute_tool(cfg, arguments, "list Linode Managed credentials", _call)


def _managed_credential_username_password_update_body(
    arguments: dict[str, Any],
) -> dict[str, str]:
    """Collect documented Managed credential username/password update fields."""
    password = _optional_string_argument(arguments, "password")
    if password is None:
        raise ValueError("password required")
    body = {"password": password}
    username = _optional_string_argument(arguments, "username")
    if username is not None:
        body["username"] = username
    return body


def create_linode_managed_credential_username_password_update_tool() -> tuple[
    Tool, Capability
]:
    """Create the managed credential username/password update tool."""
    return Tool(
        name="linode_managed_credential_username_password_update",
        description=(
            "Updates a Managed credential username and password. "
            "Requires confirm=true; pass dry_run=true with confirm=true "
            "to preview without changing it."
        ),
        inputSchema=schema(
            "linode.mcp.v1.ManagedCredentialUsernamePasswordUpdateInput"
        ),
    ), Capability.Admin


async def handle_linode_managed_credential_username_password_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_username_password_update tool request."""
    credential_id = _managed_credential_id(arguments)
    if isinstance(credential_id, list):
        return credential_id

    try:
        body = _managed_credential_username_password_update_body(arguments)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    encoded_credential_id = quote(str(credential_id), safe="")
    if is_dry_run(arguments):
        preview_body = {**body, "password": "***"}
        return build_dry_run_response(
            "linode_managed_credential_username_password_update",
            arguments.get("environment", ""),
            "POST",
            f"/managed/credentials/{encoded_credential_id}/update",
            None,
            request_body=preview_body,
            side_effects=[f"Managed credential {credential_id} will be updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a stored Managed credential's username and password. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.update_managed_credential_username_password(
            credential_id,
            password=body["password"],
            username=body.get("username"),
        )
        return serialize_api_response(
            {
                "message": f"Managed credential {credential_id} updated successfully",
                "credential_id": credential_id,
            },
            managed_pb2.ManagedCredentialIDResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode Managed credential", _call)


def create_linode_managed_credential_revoke_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credential_revoke tool."""
    return Tool(
        name="linode_managed_credential_revoke",
        description=(
            "Revokes a Managed credential. Pass confirm=true to revoke it; "
            "pass dry_run=true to preview without revoking it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedCredentialRevokeInput"),
    ), Capability.Admin


async def handle_linode_managed_credential_revoke(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_revoke tool request."""
    try:
        credential_id = _optional_int_argument(arguments, "credential_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if credential_id is None:
        return error_response("credential_id required")

    encoded_credential_id = quote(str(credential_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_credential_revoke",
            arguments.get("environment", ""),
            "POST",
            f"/managed/credentials/{encoded_credential_id}/revoke",
            None,
            side_effects=["A Managed credential is revoked."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes a stored Managed credential. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.revoke_managed_credential(credential_id)
        return serialize_api_response(
            {
                "message": f"Managed credential {credential_id} revoked successfully",
                "credential_id": credential_id,
            },
            managed_pb2.ManagedCredentialIDResponse(),
        )

    return await execute_tool(cfg, arguments, "revoke Managed credential", _call)


def _collect_managed_linode_settings_ssh(value: dict[str, Any]) -> dict[str, Any]:
    """Collect Managed Linode SSH settings from tool arguments."""
    allowed_keys = {"access", "ip", "port", "user"}
    unknown_keys = sorted(set(value) - allowed_keys)
    if unknown_keys:
        msg = "ssh contains unsupported fields: " + ", ".join(unknown_keys)
        raise ValueError(msg)
    if not value:
        msg = "ssh must contain at least one field"
        raise ValueError(msg)

    ssh: dict[str, Any] = {}
    if "access" in value:
        access = value["access"]
        if type(access) is not bool:
            msg = "ssh.access must be a boolean"
            raise ValueError(msg)
        ssh["access"] = access
    if "ip" in value:
        ip = value["ip"]
        if not isinstance(ip, str) or not ip.strip():
            msg = "ssh.ip must be a non-empty string"
            raise ValueError(msg)
        ssh["ip"] = ip.strip()
    if "port" in value:
        port = value["port"]
        if port is not None and (
            type(port) is not int or port < 1 or port > _MANAGED_LINODE_SSH_PORT_MAX
        ):
            msg = "ssh.port must be an integer from 1 to 65535 or null"
            raise ValueError(msg)
        ssh["port"] = port
    if "user" in value:
        user = value["user"]
        if user is not None and (
            not isinstance(user, str) or len(user) > _MANAGED_LINODE_SSH_USER_MAX_LENGTH
        ):
            msg = "ssh.user must be a string up to 32 characters or null"
            raise ValueError(msg)
        ssh["user"] = user
    return ssh


def _validate_managed_linode_settings_update_arguments(
    arguments: dict[str, Any],
) -> tuple[int | None, dict[str, Any] | None, str | None]:
    allowed_keys = {"environment", "linode_id", "ssh", "confirm", PARAM_DRY_RUN}
    unknown_keys = sorted(set(arguments) - allowed_keys)
    if unknown_keys:
        return (
            None,
            None,
            "Unsupported Managed Linode settings field(s): " + ", ".join(unknown_keys),
        )

    linode_id = arguments.get("linode_id")
    if not isinstance(linode_id, int) or isinstance(linode_id, bool) or linode_id < 1:
        return None, None, "linode_id must be a positive integer"

    ssh_raw = arguments.get("ssh")
    if not isinstance(ssh_raw, dict):
        return None, None, "ssh must be a non-empty object"
    typed_ssh = cast("dict[str, Any]", ssh_raw)
    try:
        ssh = _collect_managed_linode_settings_ssh(typed_ssh)
    except ValueError as exc:
        return None, None, str(exc)
    return linode_id, ssh, None


def create_linode_managed_linode_settings_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_linode_settings_update tool."""
    return Tool(
        name="linode_managed_linode_settings_update",
        description=(
            "Updates SSH-related Managed settings for a specific Linode. "
            "Pass dry_run=true to preview without updating settings."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedLinodeSettingsUpdateInput"),
    ), Capability.Admin


def create_linode_managed_sshkey_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_sshkey_get tool."""
    return Tool(
        name="linode_managed_sshkey_get",
        description="Gets the Managed SSH public key for the Linode account.",
        inputSchema=schema("linode.mcp.v1.ManagedSSHKeyGetInput"),
    ), Capability.Read


async def handle_linode_managed_linode_settings_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_linode_settings_update tool request."""
    validated = _validate_managed_linode_settings_update_arguments(arguments)
    linode_id, ssh, validation_error = validated
    if validation_error is not None or linode_id is None or ssh is None:
        return error_response(
            validation_error or "Managed Linode settings input is invalid"
        )

    encoded_linode_id = quote(str(linode_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_linode_settings_update",
            arguments.get("environment", ""),
            "PUT",
            f"/managed/linode-settings/{encoded_linode_id}",
            None,
            request_body={"ssh": ssh},
            side_effects=["Managed SSH settings are updated for the selected Linode."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates Managed Linode settings. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_managed_linode_settings(linode_id, ssh=ssh)
        return serialize_api_response(
            {
                "message": (
                    f"Managed Linode settings for Linode {linode_id} "
                    "updated successfully"
                ),
                "settings": result,
            },
            managed_pb2.ManagedLinodeSettingsWriteResponse(),
        )

    return await execute_tool(
        cfg,
        arguments,
        f"update Managed settings for Linode {linode_id}",
        _call,
    )


async def handle_linode_managed_sshkey_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_sshkey_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_managed_ssh_key(), managed_pb2.ManagedSSHKey()
        )

    return await execute_tool(cfg, arguments, "get Linode Managed SSH key", _call)


_MANAGED_CONTACT_BODY_FIELDS = ("email", "group", "name")
_MANAGED_SERVICE_READ_ONLY_FIELDS = {"created", "id", "status", "updated"}
_MANAGED_CREDENTIAL_REQUIRED_FIELDS = ("label", "password")


def _managed_credential_body(arguments: dict[str, Any]) -> dict[str, str]:
    """Collect documented Managed credential create fields from tool arguments."""
    body = {
        field: _optional_string_argument(arguments, field)
        for field in _MANAGED_CREDENTIAL_REQUIRED_FIELDS
    }
    missing = [field for field, value in body.items() if value is None]
    if missing:
        raise ValueError(f"{', '.join(missing)} required")
    username = _optional_string_argument(arguments, "username")
    if username is not None:
        body["username"] = username
    return cast("dict[str, str]", body)


def create_linode_managed_credential_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credential_create tool."""
    return Tool(
        name="linode_managed_credential_create",
        description=(
            "Creates a Managed credential. Pass confirm=true to create it; "
            "pass dry_run=true to preview without creating it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedCredentialCreateInput"),
    ), Capability.Admin


async def handle_linode_managed_credential_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_create tool request."""
    try:
        body = _managed_credential_body(arguments)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    if is_dry_run(arguments):
        preview_body = {**body, "password": "<redacted>"}
        return build_dry_run_response(
            "linode_managed_credential_create",
            arguments.get("environment", ""),
            "POST",
            "/managed/credentials",
            None,
            request_body=preview_body,
            side_effects=["A Managed credential is created."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a stored Managed credential. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        credential = await client.create_managed_credential(
            label=body["label"],
            password=body["password"],
            username=body.get("username"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Managed credential {credential.get('id', 0)} created successfully"
                ),
                "credential": credential,
            },
            managed_pb2.ManagedCredentialWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Managed credential", _call)


def _managed_credential_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive managed credential ID, or return an error response."""
    credential_id, error = required_int_id(arguments, "credential_id")
    if credential_id is None:
        return error_response(error)
    return credential_id


def _managed_credential_update_body(
    arguments: dict[str, Any],
) -> dict[str, str] | list[TextContent]:
    """Build a Managed credential update body from writable fields."""
    read_only_fields = sorted({"id", "last_decrypted"}.intersection(arguments))
    if read_only_fields:
        return error_response(
            "Read-only fields are not accepted: " + ", ".join(read_only_fields)
        )
    if "label" not in arguments:
        return error_response("label is required")
    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return error_response("label must be a non-empty string")
    return {"label": label.strip()}


def create_linode_managed_credential_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credential_update tool."""
    return Tool(
        name="linode_managed_credential_update",
        description=(
            "Updates a Managed credential label. Requires confirm=true; pass "
            "dry_run=true with confirm=true to preview without changing it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedCredentialUpdateInput"),
    ), Capability.Admin


async def handle_linode_managed_credential_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_update tool request."""
    credential_id = _managed_credential_id(arguments)
    if isinstance(credential_id, list):
        return credential_id

    body = _managed_credential_update_body(arguments)
    if isinstance(body, list):
        return body

    dry_run_path = f"/managed/credentials/{quote(str(credential_id), safe='')}"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_credential_update",
            arguments.get("environment", ""),
            "PUT",
            dry_run_path,
            None,
            side_effects=[f"Managed credential {credential_id} will be updated."],
            request_body=body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Managed credential. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        credential = await client.update_managed_credential(
            credential_id,
            label=body["label"],
        )
        return serialize_api_response(
            {
                "message": f"Managed credential {credential_id} updated successfully",
                "credential": credential,
            },
            managed_pb2.ManagedCredentialWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode Managed credential", _call)


def _managed_contact_body(arguments: dict[str, Any]) -> dict[str, Any]:
    """Collect documented Managed contact body fields from tool arguments.

    The string fields (email, group, name) carry through as-is; the phone
    argument is a nested object with optional primary and secondary numbers.
    """
    # The API assigns id and updated; Go rejects them on create, so mirror that
    # here for identical cross-language rejection.
    if "id" in arguments or "updated" in arguments:
        raise ValueError(
            "id and updated are read-only and cannot be set "
            "when creating a managed contact"
        )
    body: dict[str, Any] = {
        field: value
        for field in _MANAGED_CONTACT_BODY_FIELDS
        if (value := _optional_string_argument(arguments, field)) is not None
    }
    if "phone" in arguments:
        phone_body = _managed_contact_phone_body(arguments.get("phone"))
        if isinstance(phone_body, str):
            raise ValueError(phone_body)
        body["phone"] = phone_body
    if not body:
        raise ValueError("At least one of email, group, name, or phone is required")
    return body


def _managed_service_body(arguments: dict[str, Any]) -> dict[str, Any]:
    """Collect documented Managed service create fields from tool arguments."""
    read_only = sorted(_MANAGED_SERVICE_READ_ONLY_FIELDS.intersection(arguments))
    if read_only:
        raise ValueError("Read-only fields are not accepted: " + ", ".join(read_only))

    label = _required_string_argument(arguments, "label")
    service_type = _required_string_argument(arguments, "service_type")
    error = enum_choice_error(
        service_type, "service_type", managed_pb2.ManagedServiceType.Value
    )
    if error is not None:
        raise ValueError(error)
    address = _required_string_argument(arguments, "address")
    timeout = _optional_int_argument(arguments, "timeout", 1, 255)
    if timeout is None:
        raise ValueError("timeout is required")

    body: dict[str, Any] = {
        "label": label,
        "service_type": service_type,
        "address": address,
        "timeout": timeout,
    }
    for field in ("body", "consultation_group", "notes", "region"):
        if (value := _optional_string_argument(arguments, field)) is not None:
            body[field] = value

    credentials = arguments.get("credentials")
    if credentials is not None:
        if not isinstance(credentials, list):
            raise TypeError("credentials must be an array of positive integers")
        typed_credentials = cast("list[object]", credentials)
        if any(
            not isinstance(item, int) or isinstance(item, bool) or item < 1
            for item in typed_credentials
        ):
            raise ValueError("credentials must be an array of positive integers")
        body["credentials"] = typed_credentials

    return body


def create_linode_managed_service_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_create tool."""
    return Tool(
        name="linode_managed_service_create",
        description=(
            "Creates a Managed service monitor. Pass confirm=true to create it; "
            "pass dry_run=true to preview without creating it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedServiceCreateInput"),
    ), Capability.Admin


async def handle_linode_managed_service_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_create tool request."""
    try:
        request_body = _managed_service_body(arguments)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_service_create",
            arguments.get("environment", ""),
            "POST",
            "/managed/services",
            None,
            request_body=request_body,
            side_effects=["A Managed service monitor is created."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a Managed service monitor. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        service = await client.create_managed_service(
            label=request_body["label"],
            service_type=request_body["service_type"],
            address=request_body["address"],
            timeout=request_body["timeout"],
            body=request_body.get("body"),
            consultation_group=request_body.get("consultation_group"),
            credentials=request_body.get("credentials"),
            notes=request_body.get("notes"),
            region=request_body.get("region"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Managed service monitor {service.get('id', 0)} "
                    "created successfully"
                ),
                "service": service,
            },
            managed_pb2.ManagedServiceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Managed service monitor", _call)


def create_linode_managed_service_enable_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_enable tool."""
    return Tool(
        name="linode_managed_service_enable",
        description=(
            "Enables a Managed service monitor. Requires confirm=true; pass "
            "dry_run=true with confirm=true to preview without enabling it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedServiceEnableInput"),
    ), Capability.Admin


async def handle_linode_managed_service_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_enable tool request."""
    service_id = _managed_service_id(arguments)
    if isinstance(service_id, list):
        return service_id

    dry_run_path = f"/managed/services/{quote(str(service_id), safe='')}/enable"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_service_enable",
            arguments.get("environment", ""),
            "POST",
            dry_run_path,
            None,
            side_effects=[f"Managed service monitor {service_id} will be enabled."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This enables a Managed service monitor. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.enable_managed_service(service_id)
        return serialize_api_response(
            {
                "message": "Managed service enabled successfully",
                "service_id": service_id,
            },
            managed_pb2.ManagedServiceIDResponse(),
        )

    return await execute_tool(cfg, arguments, "enable Managed service monitor", _call)


def create_linode_managed_service_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_delete tool."""
    return Tool(
        name="linode_managed_service_delete",
        description=(
            "Deletes a Managed service monitor. Requires confirm=true; pass "
            "dry_run=true with confirm=true to preview without deleting it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedServiceDeleteInput"),
    ), Capability.Admin


async def handle_linode_managed_service_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_delete tool request."""
    try:
        service_id = _optional_int_argument(arguments, "service_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if service_id is None:
        return error_response("service_id required")

    encoded_service_id = quote(str(service_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_service_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/managed/services/{encoded_service_id}",
            None,
            side_effects=[f"Managed service monitor {service_id} will be deleted."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Managed service monitor. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_managed_service(service_id)
        return serialize_api_response(
            {
                "message": "Managed service deleted successfully",
                "service_id": service_id,
            },
            managed_pb2.ManagedServiceIDResponse(),
        )

    return await execute_tool(cfg, arguments, "delete Managed service monitor", _call)


def create_linode_managed_service_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_get tool."""
    return Tool(
        name="linode_managed_service_get",
        description="Gets a Linode Managed service monitor by ID.",
        inputSchema=schema("linode.mcp.v1.ManagedServiceGetInput"),
    ), Capability.Read


async def handle_linode_managed_service_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_get tool request."""
    service_id = arguments.get("service_id")
    if (
        not isinstance(service_id, int)
        or isinstance(service_id, bool)
        or service_id < 1
    ):
        return error_response("service_id must be a positive integer")
    validated_service_id = service_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_managed_service(validated_service_id),
            managed_pb2.ManagedService(),
        )

    return await execute_tool(
        cfg, arguments, "get Linode Managed service monitor", _call
    )


def create_linode_managed_service_disable_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_disable tool."""
    return Tool(
        name="linode_managed_service_disable",
        description=(
            "Disables a Managed service monitor by service ID. "
            "Pass dry_run=true to preview without disabling."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedServiceDisableInput"),
    ), Capability.Admin


async def handle_linode_managed_service_disable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_disable tool request."""
    try:
        service_id = _optional_int_argument(arguments, "service_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if service_id is None:
        return error_response("service_id is required")

    encoded_service_id = quote(str(service_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_service_disable",
            arguments.get("environment", ""),
            "POST",
            f"/managed/services/{encoded_service_id}/disable",
            None,
            side_effects=["The selected Managed service monitor is disabled."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This disables a Managed service monitor. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.disable_managed_service(service_id)
        return serialize_api_response(
            {
                "message": "Managed service disabled successfully",
                "service_id": service_id,
            },
            managed_pb2.ManagedServiceIDResponse(),
        )

    return await execute_tool(cfg, arguments, "disable Managed service monitor", _call)


def create_linode_managed_contact_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_create tool."""
    return Tool(
        name="linode_managed_contact_create",
        description=(
            "Creates a Managed contact. Pass confirm=true to create it; "
            "pass dry_run=true to preview without creating it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedContactCreateInput"),
    ), Capability.Admin


async def handle_linode_managed_contact_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contact_create tool request."""
    try:
        body = _managed_contact_body(arguments)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_contact_create",
            arguments.get("environment", ""),
            "POST",
            "/managed/contacts",
            None,
            request_body=body,
            side_effects=["A Managed contact is created."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a managed contact. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        contact = await client.create_managed_contact(
            email=body.get("email"),
            group=body.get("group"),
            name=body.get("name"),
            phone=body.get("phone"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Managed contact {contact.get('id', 0)} created successfully"
                ),
                "contact": contact,
            },
            managed_pb2.ManagedContactWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Managed contact", _call)


def create_linode_managed_contact_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_delete tool."""
    return Tool(
        name="linode_managed_contact_delete",
        description=(
            "Deletes a Managed contact by contact ID. "
            "Pass dry_run=true to preview without deleting."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedContactDeleteInput"),
    ), Capability.Admin


async def handle_linode_managed_contact_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contact_delete tool request."""
    try:
        contact_id = _optional_int_argument(arguments, "contact_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if contact_id is None:
        return error_response("contact_id is required")

    encoded_contact_id = quote(str(contact_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_contact_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/managed/contacts/{encoded_contact_id}",
            None,
            side_effects=["The selected Managed contact is deleted."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Managed contact. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_managed_contact(contact_id)
        return serialize_api_response(
            {
                "message": "Managed contact deleted successfully",
                "contact_id": contact_id,
            },
            managed_pb2.ManagedContactIDResponse(),
        )

    return await execute_tool(cfg, arguments, "delete Managed contact", _call)


def create_linode_managed_contact_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_update tool."""
    return Tool(
        name="linode_managed_contact_update",
        description=(
            "Updates a Managed contact. Requires confirm=true; pass "
            "dry_run=true with confirm=true to preview without changing it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedContactUpdateInput"),
    ), Capability.Admin


def _managed_contact_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive managed contact ID, or return an error response."""
    contact_id, error = required_int_id(arguments, "contact_id")
    if contact_id is None:
        return error_response(error)
    return contact_id


def _managed_contact_string_fields(
    arguments: dict[str, Any], body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy non-empty string update fields into the request body."""
    for field in ("email", "name"):
        if field not in arguments:
            continue
        value = arguments.get(field)
        if not isinstance(value, str) or not value.strip():
            return error_response(f"{field} must be a non-empty string")
        body[field] = value.strip()
    return None


def _managed_contact_group_field(
    arguments: dict[str, Any], body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy the nullable group field into the request body."""
    if "group" not in arguments:
        return None
    group = arguments.get("group")
    if group is None:
        body["group"] = None
        return None
    if isinstance(group, str) and group.strip():
        body["group"] = group.strip()
        return None
    return error_response("group must be a non-empty string or null")


def _managed_contact_phone_body(phone: object) -> dict[str, str | None] | str:
    """Build the nested phone object, or return an error string."""
    if not isinstance(phone, dict):
        return "phone must be an object"
    typed_phone = cast("dict[str, Any]", phone)
    unknown_phone_fields = sorted(set(typed_phone) - {"primary", "secondary"})
    if unknown_phone_fields:
        return "phone has unknown fields: " + ", ".join(unknown_phone_fields)

    phone_body: dict[str, str | None] = {}
    for field in ("primary", "secondary"):
        if field not in typed_phone:
            continue
        value = typed_phone.get(field)
        if value is None:
            phone_body[field] = None
        elif isinstance(value, str) and value.strip():
            phone_body[field] = value.strip()
        else:
            return f"phone.{field} must be a non-empty string or null"
    if not phone_body:
        return "phone must include primary or secondary"
    return phone_body


def _managed_contact_phone_field(
    arguments: dict[str, Any], body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy the nested phone field into the request body."""
    if "phone" not in arguments:
        return None
    phone_body = _managed_contact_phone_body(arguments.get("phone"))
    if isinstance(phone_body, str):
        return error_response(phone_body)
    body["phone"] = phone_body
    return None


def _managed_contact_update_body(
    arguments: dict[str, Any],
) -> dict[str, Any] | list[TextContent]:
    """Build a Managed contact update body from writable fields."""
    read_only_fields = sorted({"id", "updated"}.intersection(arguments))
    if read_only_fields:
        return error_response(
            "Read-only fields are not accepted: " + ", ".join(read_only_fields)
        )

    body: dict[str, Any] = {}
    for validator in (
        _managed_contact_string_fields,
        _managed_contact_group_field,
        _managed_contact_phone_field,
    ):
        error = validator(arguments, body)
        if error is not None:
            return error

    if not body:
        return error_response("At least one managed contact field is required")
    return body


async def handle_linode_managed_contact_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contact_update tool request."""
    contact_id = _managed_contact_id(arguments)
    if isinstance(contact_id, list):
        return contact_id

    body = _managed_contact_update_body(arguments)
    if isinstance(body, list):
        return body

    dry_run_path = f"/managed/contacts/{quote(str(contact_id), safe='')}"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_contact_update",
            arguments.get("environment", ""),
            "PUT",
            dry_run_path,
            None,
            side_effects=[f"Managed contact {contact_id} will be updated."],
            request_body=body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Managed contact. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        contact = await client.update_managed_contact(contact_id, **body)
        return serialize_api_response(
            {
                "message": f"Managed contact {contact_id} updated successfully",
                "contact": contact,
            },
            managed_pb2.ManagedContactWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode Managed contact", _call)


def create_linode_managed_service_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_update tool."""
    return Tool(
        name="linode_managed_service_update",
        description=(
            "Updates a Managed service monitor. Requires confirm=true; pass "
            "dry_run=true with confirm=true to preview without changing it."
        ),
        inputSchema=schema("linode.mcp.v1.ManagedServiceUpdateInput"),
    ), Capability.Admin


def _managed_service_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive Managed service ID, or return an error response."""
    service_id = arguments.get("service_id")
    if (
        not isinstance(service_id, int)
        or isinstance(service_id, bool)
        or service_id < 1
    ):
        return error_response("service_id must be a positive integer")
    return service_id


def _managed_service_string_fields(
    arguments: dict[str, Any], request_body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy non-empty string service update fields into the request body."""
    for field in ("address", "consultation_group", "label"):
        if field not in arguments:
            continue
        value = arguments.get(field)
        if not isinstance(value, str) or not value.strip():
            return error_response(f"{field} must be a non-empty string")
        request_body[field] = value.strip()
    return None


def _managed_service_nullable_string_fields(
    arguments: dict[str, Any], request_body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy nullable string service update fields into the request body."""
    for field in ("body", "notes", "region"):
        if field not in arguments:
            continue
        value = arguments.get(field)
        if value is None:
            request_body[field] = None
        elif isinstance(value, str):
            request_body[field] = value
        else:
            return error_response(f"{field} must be a string or null")
    return None


def _managed_service_credentials_field(
    arguments: dict[str, Any], request_body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy Managed credential IDs into the request body."""
    if "credentials" not in arguments:
        return None
    credentials = arguments.get("credentials")
    if not isinstance(credentials, list):
        return error_response("credentials must be an array of positive integers")
    raw_credentials: list[object] = [*credentials]
    typed_credentials: list[int] = []
    for credential_id in raw_credentials:
        if (
            not isinstance(credential_id, int)
            or isinstance(credential_id, bool)
            or credential_id < 1
        ):
            return error_response("credentials must be an array of positive integers")
        typed_credentials.append(credential_id)
    request_body["credentials"] = typed_credentials
    return None


def _managed_service_type_field(
    arguments: dict[str, Any], request_body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy and validate the service_type field."""
    if "service_type" not in arguments:
        return None
    service_type = arguments.get("service_type")
    if service_type not in enum_value_names(managed_pb2.ManagedServiceType.Value):
        return error_response(
            "service_type must be one of: "
            + ", ".join(enum_value_names(managed_pb2.ManagedServiceType.Value))
        )
    request_body["service_type"] = service_type
    return None


def _managed_service_timeout_field(
    arguments: dict[str, Any], request_body: dict[str, Any]
) -> list[TextContent] | None:
    """Copy and validate the timeout field."""
    if "timeout" not in arguments:
        return None
    timeout = arguments.get("timeout")
    if (
        not isinstance(timeout, int)
        or isinstance(timeout, bool)
        or not 1 <= timeout <= _MANAGED_SERVICE_TIMEOUT_MAX
    ):
        return error_response("timeout must be an integer between 1 and 255")
    request_body["timeout"] = timeout
    return None


def _managed_service_update_body(
    arguments: dict[str, Any],
) -> dict[str, Any] | list[TextContent]:
    """Build a Managed service update body from writable documented fields."""
    read_only_fields = sorted(
        {"created", "id", "status", "updated"}.intersection(arguments)
    )
    if read_only_fields:
        return error_response(
            "Read-only fields are not accepted: " + ", ".join(read_only_fields)
        )
    request_body: dict[str, Any] = {}
    for validator in (
        _managed_service_string_fields,
        _managed_service_nullable_string_fields,
        _managed_service_credentials_field,
        _managed_service_type_field,
        _managed_service_timeout_field,
    ):
        error = validator(arguments, request_body)
        if error is not None:
            return error
    if not request_body:
        return error_response("At least one managed service field is required")
    return request_body


async def handle_linode_managed_service_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_update tool request."""
    service_id = _managed_service_id(arguments)
    if isinstance(service_id, list):
        return service_id
    request_body = _managed_service_update_body(arguments)
    if isinstance(request_body, list):
        return request_body
    dry_run_path = f"/managed/services/{quote(str(service_id), safe='')}"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_service_update",
            arguments.get("environment", ""),
            "PUT",
            dry_run_path,
            None,
            side_effects=[f"Managed service monitor {service_id} will be updated."],
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Managed service monitor. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        service = await client.update_managed_service(service_id, **request_body)
        return serialize_api_response(
            {
                "message": (
                    f"Managed service monitor {service_id} updated successfully"
                ),
                "service": service,
            },
            managed_pb2.ManagedServiceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode Managed service", _call)


def create_linode_managed_linode_settings_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_linode_settings_list tool."""
    return Tool(
        name="linode_managed_linode_settings_list",
        description="Lists Managed Linode settings on the account.",
        inputSchema=schema("linode.mcp.v1.ManagedLinodeSettingsListInput"),
    ), Capability.Read


async def handle_linode_managed_linode_settings_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_linode_settings_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_managed_linode_settings(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "managed_linode_settings",
            managed_pb2.ManagedLinodeSettingsListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode Managed Linode settings", _call
    )


def create_linode_managed_stats_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_stats_get tool."""
    return Tool(
        name="linode_managed_stats_get",
        description="Lists Managed statistics from the last 24 hours.",
        inputSchema=schema("linode.mcp.v1.ManagedStatsGetInput"),
    ), Capability.Read


async def handle_linode_managed_stats_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_stats_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_struct_response(await client.get_managed_stats())

    return await execute_tool(cfg, arguments, "list Linode Managed statistics", _call)


def create_linode_managed_linode_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_linode_settings_get tool."""
    return Tool(
        name="linode_managed_linode_settings_get",
        description="Gets Managed settings for a Linode.",
        inputSchema=schema("linode.mcp.v1.ManagedLinodeSettingsGetInput"),
    ), Capability.Read


async def handle_linode_managed_linode_settings_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_linode_settings_get tool request."""
    linode_id = arguments.get("linode_id")
    if isinstance(linode_id, bool) or not isinstance(linode_id, int) or linode_id < 1:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_managed_linode_settings(linode_id),
            managed_pb2.ManagedLinodeSettings(),
        )

    return await execute_tool(cfg, arguments, "get Linode Managed settings", _call)


def create_linode_managed_service_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_service_list tool."""
    return Tool(
        name="linode_managed_service_list",
        description="Lists Managed services on the Linode account.",
        inputSchema=schema("linode.mcp.v1.ManagedServiceListInput"),
    ), Capability.Read


async def handle_linode_managed_service_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_service_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_managed_services(page=page, page_size=page_size)
        return serialize_list_response(
            raw, "managed_services", managed_pb2.ManagedServiceListResponse()
        )

    return await execute_tool(cfg, arguments, "list Linode Managed services", _call)


def create_linode_managed_issue_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_issue_get tool."""
    return Tool(
        name="linode_managed_issue_get",
        description="Gets a Linode Managed issue by ID.",
        inputSchema=schema("linode.mcp.v1.ManagedIssueGetInput"),
    ), Capability.Read


async def handle_linode_managed_issue_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_issue_get tool request."""
    issue_id = arguments.get("issue_id")
    if not isinstance(issue_id, int) or isinstance(issue_id, bool) or issue_id < 1:
        return error_response("issue_id must be a positive integer")
    validated_issue_id = issue_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_managed_issue(validated_issue_id),
            managed_issue_pb2.ManagedIssue(),
        )

    return await execute_tool(cfg, arguments, "get Linode Managed issue", _call)


def create_linode_managed_contact_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_get tool."""
    return Tool(
        name="linode_managed_contact_get",
        description="Gets a Linode Managed contact by ID.",
        inputSchema=schema("linode.mcp.v1.ManagedContactGetInput"),
    ), Capability.Read


async def handle_linode_managed_contact_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contact_get tool request."""
    contact_id = arguments.get("contact_id")
    if (
        not isinstance(contact_id, int)
        or isinstance(contact_id, bool)
        or contact_id < 1
    ):
        return error_response("contact_id must be a positive integer")
    validated_contact_id = contact_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_managed_contact(validated_contact_id),
            managed_pb2.ManagedContact(),
        )

    return await execute_tool(cfg, arguments, "get Linode Managed contact", _call)


def create_linode_support_ticket_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_list tool."""
    return Tool(
        name="linode_support_ticket_list",
        description="Lists Linode support tickets.",
        inputSchema=schema("linode.mcp.v1.SupportTicketListInput"),
    ), Capability.Read


async def handle_linode_support_ticket_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_support_tickets(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "support_tickets",
            support_ticket_pb2.SupportTicketListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Linode support tickets", _call)


def create_linode_support_ticket_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_get tool."""
    return Tool(
        name="linode_support_ticket_get",
        description="Gets a Linode support ticket by ID.",
        inputSchema=schema("linode.mcp.v1.SupportTicketGetInput"),
    ), Capability.Read


async def handle_linode_support_ticket_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_get tool request."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_support_ticket(ticket_id),
            support_ticket_pb2.SupportTicket(),
        )

    return await execute_tool(cfg, arguments, "get Linode support ticket", _call)


def create_linode_support_ticket_reply_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_reply_list tool."""
    return Tool(
        name="linode_support_ticket_reply_list",
        description="Lists replies on a Linode support ticket.",
        inputSchema=schema("linode.mcp.v1.SupportTicketReplyListInput"),
    ), Capability.Read


async def handle_linode_support_ticket_reply_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_reply_list tool request."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_support_ticket_replies(
            ticket_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "support_ticket_replies",
            support_ticket_pb2.SupportTicketReplyListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list Linode support ticket replies", _call
    )


def create_linode_support_ticket_close_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_close tool."""
    return Tool(
        name="linode_support_ticket_close",
        description="Closes a Linode support ticket.",
        inputSchema=schema("linode.mcp.v1.SupportTicketCloseInput"),
    ), Capability.Write


async def handle_linode_support_ticket_close(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_close tool request."""
    if is_dry_run(arguments):
        ticket = _support_ticket_id(arguments)
        if isinstance(ticket, list):
            return ticket
        tid = ticket

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_support_ticket(tid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {"side_effects": [f"Support ticket {tid} will be closed."]}

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_support_ticket_close",
            "POST",
            f"/support/tickets/{tid}/close",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This closes a support ticket. Set confirm=true to proceed."
        )

    ticket = _support_ticket_id(arguments)
    if isinstance(ticket, list):
        return ticket
    ticket_id = ticket

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The close endpoint returns no useful resource body, so the response
        # echoes the affected ticket id.
        await client.close_support_ticket(ticket_id)
        return serialize_api_response(
            {"message": "Support ticket closed successfully", "ticket_id": ticket_id},
            support_ticket_pb2.SupportTicketIDResponse(),
        )

    return await execute_tool(cfg, arguments, "close Linode support ticket", _call)


def create_linode_support_ticket_reply_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_reply_create tool."""
    return Tool(
        name="linode_support_ticket_reply_create",
        description="Creates a reply on a Linode support ticket.",
        inputSchema=schema("linode.mcp.v1.SupportTicketReplyCreateInput"),
    ), Capability.Write


async def handle_linode_support_ticket_reply_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_reply_create tool request."""
    if is_dry_run(arguments):
        ticket = _support_ticket_id(arguments)
        if isinstance(ticket, list):
            return ticket
        tid = ticket

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_support_ticket(tid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [f"A reply will be posted to support ticket {tid}."]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_support_ticket_reply_create",
            "POST",
            f"/support/tickets/{tid}/replies",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a support ticket reply. Set confirm=true to proceed."
        )

    ticket = _support_ticket_id(arguments)
    if isinstance(ticket, list):
        return ticket
    ticket_id = ticket

    description = arguments.get("description")
    if not isinstance(description, str) or not description.strip():
        return error_response("description is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        reply = await client.create_support_ticket_reply(ticket_id, description.strip())
        return serialize_api_response(
            {"message": "Support ticket reply created successfully", "reply": reply},
            support_ticket_pb2.SupportTicketReplyWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "create Linode support ticket reply", _call
    )


def create_linode_support_ticket_attachment_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_support_ticket_attachment_create tool."""
    return Tool(
        name="linode_support_ticket_attachment_create",
        description="Creates an attachment on a Linode support ticket.",
        inputSchema=schema("linode.mcp.v1.SupportTicketAttachmentCreateInput"),
    ), Capability.Write


async def handle_linode_support_ticket_attachment_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_support_ticket_attachment_create tool request."""
    if is_dry_run(arguments):
        ticket = _support_ticket_id(arguments)
        if isinstance(ticket, list):
            return ticket
        tid = ticket

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_support_ticket(tid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"A file attachment will be uploaded to support ticket {tid}."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_support_ticket_attachment_create",
            "POST",
            f"/support/tickets/{tid}/attachments",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a support ticket attachment. Set confirm=true to proceed."
        )

    ticket = _support_ticket_id(arguments)
    if isinstance(ticket, list):
        return ticket
    ticket_id = ticket

    parsed_file = _attachment_file(arguments)
    if isinstance(parsed_file, list):
        return parsed_file
    file_path = parsed_file

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The attachment endpoint returns no useful resource body, so the
        # response echoes the affected ticket id.
        await client.create_support_ticket_attachment(ticket_id, file_path)
        return serialize_api_response(
            {
                "message": "Support ticket attachment created successfully",
                "ticket_id": ticket_id,
            },
            support_ticket_pb2.SupportTicketIDResponse(),
        )

    return await execute_tool(
        cfg, arguments, "create Linode support ticket attachment", _call
    )

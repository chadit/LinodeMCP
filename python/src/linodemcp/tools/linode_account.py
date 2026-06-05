"""Linode account tool - authenticated user account information."""

import re
from pathlib import Path
from typing import Any, cast
from urllib.parse import quote

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
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
    "longview",
    "nodebalancer",
    "stackscript",
    "volume",
    "vpc",
)
_VALID_GRANT_PERMISSIONS = {"read_only", "read_write"}
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
    if permissions is not None and (
        not isinstance(permissions, str) or permissions not in _VALID_GRANT_PERMISSIONS
    ):
        return (
            None,
            f"{field}[{index}].permissions must be 'read_only', 'read_write', or null",
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
            if grant_value is not None and (
                not isinstance(grant_value, str)
                or grant_value not in _VALID_GRANT_PERMISSIONS
            ):
                return (
                    None,
                    f"global.{field} must be 'read_only', 'read_write', or null",
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
    """Validate a service transfer token supplied by an MCP caller."""
    if value is None:
        return None, "token is required"
    if not isinstance(value, str):
        return None, "token must be a string"

    token = value.strip()
    if not token:
        return None, "token is required"
    if token != value or "/" in token or "?" in token or ".." in token:
        return (
            None,
            "token must not contain path separators, "
            "query separators, or traversal segments",
        )
    return token, None


_OAUTH_CLIENT_UPDATE_FIELDS = (
    "label",
    "public",
    "redirect_uri",
    "secret",
    "status",
    "thumbnail_url",
)
_ACCOUNT_OAUTH_CLIENT_ID_PATTERN_TEXT = r"^[A-Za-z0-9][A-Za-z0-9_-]*$"
_ACCOUNT_OAUTH_CLIENT_ID_PATTERN = re.compile(_ACCOUNT_OAUTH_CLIENT_ID_PATTERN_TEXT)
_USD_AMOUNT_PATTERN = re.compile(r"^(?!0+(?:\.0{1,2})?$)\d+(?:\.\d{1,2})?$")


def create_linode_account_tool() -> tuple[Tool, Capability]:
    """Create the linode_account tool."""
    return Tool(
        name="linode_account",
        description=(
            "Retrieves the authenticated user's Linode account information "
            "including billing details and capabilities"
        ),
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


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


def create_linode_account_user_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_create tool."""
    return Tool(
        name="linode_account_user_create",
        description="Creates a user on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "username": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Username for the new account user",
                },
                "email": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Email address for the new account user",
                },
                "restricted": {
                    "type": "boolean",
                    "description": (
                        "Set true to create a restricted user; set false to create "
                        "an unrestricted user."
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["username", "email", "restricted", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_user_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_create tool request."""
    try:
        username = _required_string_argument(arguments, "username")
        email = _required_string_argument(arguments, "email")
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    restricted = arguments.get("restricted")
    if type(restricted) is not bool:
        return error_response("restricted is required and must be a boolean")

    body = {"username": username, "email": email, "restricted": restricted}

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an account user. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_user_create",
            arguments.get("environment", ""),
            "POST",
            "/account/users",
            None,
            side_effects=[
                f"A new account user {username!r} will be created "
                f"with restricted={restricted}."
            ],
            request_body=body,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        user = await client.create_account_user(username, email, restricted)
        return {"message": "Account user created successfully", "user": user}

    return await execute_tool(cfg, arguments, "create Linode account user", _call)


def create_linode_account_user_grants_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_grants_update tool."""
    resource_grant_schema = {
        "type": "array",
        "items": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "id": {
                    "type": "integer",
                    "description": "Resource ID this grant applies to",
                },
                "permissions": {
                    "type": ["string", "null"],
                    "enum": ["read_only", "read_write", None],
                    "description": "Grant level; use null to remove access",
                },
            },
            "required": ["id", "permissions"],
        },
    }
    grant_props: dict[str, dict[str, Any]] = {
        field: resource_grant_schema.copy() for field in _RESOURCE_GRANT_FIELDS
    }
    grant_props["global"] = {
        "type": "object",
        "additionalProperties": False,
        "properties": {
            "account_access": {
                "type": ["string", "null"],
                "enum": ["read_only", "read_write", None],
            },
            **{field: {"type": "boolean"} for field in _GLOBAL_BOOLEAN_FIELDS},
            "child_account_access": {"type": ["boolean", "null"]},
        },
    }
    return Tool(
        name="linode_account_user_grants_update",
        description="Updates grants for an account user.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "username": {
                    "type": "string",
                    "pattern": _ACCOUNT_USERNAME_PATTERN_TEXT,
                    "description": "Username whose grants will be updated",
                },
                **grant_props,
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["username", "confirm"],
        },
    ), Capability.Write


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

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates account user grants. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_account_user_grants(username, grants)
        return {"message": "Account user grants updated successfully", "grants": result}

    return await execute_tool(
        cfg, arguments, f"update Linode account user grants for {username}", _call
    )


def create_linode_account_agreements_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_agreements_list tool."""
    return Tool(
        name="linode_account_agreements_list",
        description="Lists agreements on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_account_agreements_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_agreements_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_agreements()

    return await execute_tool(cfg, arguments, "list Linode account agreements", _call)


def create_linode_account_logins_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_logins_list tool."""
    return Tool(
        name="linode_account_logins_list",
        description="Lists user logins on the Linode account.",
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


async def handle_linode_account_logins_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_logins_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_logins(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account logins", _call)


def create_linode_account_users_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_users_list tool."""
    return Tool(
        name="linode_account_users_list",
        description="Lists users on the Linode account.",
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


async def handle_linode_account_users_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_users_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_users(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account users", _call)


def create_linode_account_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_settings_get tool."""
    return Tool(
        name="linode_account_settings_get",
        description="Gets settings for the Linode account.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_account_settings_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_settings_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_settings()

    return await execute_tool(cfg, arguments, "get Linode account settings", _call)


def create_linode_account_settings_managed_enable_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_settings_managed_enable tool."""
    return Tool(
        name="linode_account_settings_managed_enable",
        description=(
            "Enables Linode Managed for the account. Pass dry_run=true to "
            "preview without enabling it."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm enabling or previewing Linode Managed."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_account_settings_managed_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_settings_managed_enable tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This enables Linode Managed. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_settings_managed_enable",
            arguments.get("environment", ""),
            "POST",
            "/account/settings/managed-enable",
            None,
            side_effects=["Linode Managed is enabled for this account."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.enable_account_managed()

    return await execute_tool(cfg, arguments, "enable Linode Managed", _call)


def create_linode_account_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_transfer_get tool."""
    return Tool(
        name="linode_account_transfer_get",
        description="Gets network transfer usage for the Linode account.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_account_transfer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_transfer_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_transfer()

    return await execute_tool(cfg, arguments, "get Linode account transfer", _call)


def create_linode_account_maintenance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_maintenance_list tool."""
    return Tool(
        name="linode_account_maintenance_list",
        description="Lists maintenances on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_account_maintenance_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_maintenance_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_maintenance()

    return await execute_tool(cfg, arguments, "list Linode account maintenance", _call)


def create_linode_maintenance_policies_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_maintenance_policies_list tool."""
    return Tool(
        name="linode_maintenance_policies_list",
        description="Lists available maintenance policies.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_maintenance_policies_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_maintenance_policies_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_maintenance_policies()

    return await execute_tool(cfg, arguments, "list Linode maintenance policies", _call)


def create_linode_account_oauth_clients_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_clients_list tool."""
    return Tool(
        name="linode_account_oauth_clients_list",
        description="Lists OAuth clients on the Linode account.",
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


async def handle_linode_account_oauth_clients_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_clients_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_oauth_clients(page=page, page_size=page_size)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "string",
                    "minLength": 1,
                    "description": "OAuth client ID",
                },
                "label": {"type": "string", "description": "OAuth client label"},
                "public": {
                    "type": "boolean",
                    "description": "Whether the client is public",
                },
                "redirect_uri": {"type": "string", "description": "OAuth redirect URI"},
                "secret": {"type": "string", "description": "OAuth client secret"},
                "status": {"type": "string", "description": "OAuth client status"},
                "thumbnail_url": {
                    "type": "string",
                    "description": "OAuth client thumbnail URL",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["client_id", "confirm"],
        },
    ), Capability.Write


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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "string",
                    "minLength": 1,
                    "pattern": _ACCOUNT_OAUTH_CLIENT_ID_PATTERN_TEXT,
                    "description": "OAuth client ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["client_id", "confirm"],
        },
    ), Capability.Write


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
        return {
            "message": "OAuth client updated successfully",
            "client": oauth_client,
        }

    return await execute_tool(
        cfg, arguments, "update Linode account OAuth client", _call
    )


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
        oauth_client = await client.update_account_oauth_client_thumbnail(client_id)
        return {
            "message": "OAuth client thumbnail updated successfully",
            "client": oauth_client,
        }

    return await execute_tool(
        cfg, arguments, "update Linode account OAuth client thumbnail", _call
    )


def create_linode_account_events_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_events_list tool."""
    return Tool(
        name="linode_account_events_list",
        description="Lists events on the Linode account.",
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


async def handle_linode_account_events_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_events_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_events(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account events", _call)


def create_linode_account_invoices_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_invoices_list tool."""
    return Tool(
        name="linode_account_invoices_list",
        description="Lists invoices on the Linode account.",
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


async def handle_linode_account_invoices_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_invoices_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_invoices(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account invoices", _call)


def create_linode_account_payments_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payments_list tool."""
    return Tool(
        name="linode_account_payments_list",
        description="Lists payments on the Linode account.",
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


async def handle_linode_account_payments_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payments_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_payments(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account payments", _call)


def create_linode_account_payment_methods_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_methods_list tool."""
    return Tool(
        name="linode_account_payment_methods_list",
        description="Lists payment methods on the Linode account.",
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


async def handle_linode_account_payment_methods_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_methods_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_payment_methods(page=page, page_size=page_size)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "payment_method_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Payment method ID to charge",
                },
                "usd": {
                    "type": "string",
                    "minLength": 1,
                    "pattern": r"^(?!0+(?:\.0{1,2})?$)\d+(?:\.\d{1,2})?$",
                    "description": "Payment amount in USD",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm live payment creation. "
                        "When dry_run=true, the request is previewed without "
                        "creating the payment."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["payment_method_id", "usd", "confirm"],
        },
    ), Capability.Write


def _account_payment_create_body(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    payment_method_id = arguments.get("payment_method_id")
    if payment_method_id is None:
        return None, "payment_method_id is required"
    if (
        isinstance(payment_method_id, bool)
        or not isinstance(payment_method_id, int)
        or payment_method_id < 1
    ):
        return None, "payment_method_id must be a positive integer"

    usd, usd_error = _required_nonempty_string_argument(arguments, "usd")
    if usd_error is not None or usd is None:
        return None, usd_error or "usd is required"
    if _USD_AMOUNT_PATTERN.fullmatch(usd) is None:
        return None, "usd must be a positive dollar amount with up to two decimals"

    return {"payment_method_id": payment_method_id, "usd": usd}, None


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
            "This creates an account payment. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_account_payment(
            request_body["payment_method_id"], str(request_body["usd"])
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_ids": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "minItems": 1,
                    "description": "Linode IDs to include in the service transfer",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm live service transfer creation. "
                        "When dry_run=true, the request is previewed without "
                        "creating the service transfer."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_ids", "confirm"],
        },
    ), Capability.Write


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
        return await client.create_account_service_transfer(linode_ids)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "payment_method_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Payment method ID to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this destructive operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["payment_method_id", "confirm"],
        },
    ), Capability.Destroy


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
        result = await client.delete_account_payment_method(payment_method_id)
        return {
            "message": "Payment method deleted successfully",
            "result": result,
        }

    return await execute_tool(
        cfg,
        arguments,
        f"delete Linode account payment method {payment_method_id}",
        _call,
    )


def create_linode_account_notifications_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_notifications_list tool."""
    return Tool(
        name="linode_account_notifications_list",
        description="Lists notifications on the Linode account.",
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


async def handle_linode_account_notifications_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_notifications_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_notifications(page=page, page_size=page_size)

    return await execute_tool(
        cfg, arguments, "list Linode account notifications", _call
    )


def create_linode_account_invoice_items_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_invoice_items_list tool."""
    return Tool(
        name="linode_account_invoice_items_list",
        description="Lists items on a Linode account invoice.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "invoice_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Invoice ID whose items to list",
                },
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
            "required": ["invoice_id"],
        },
    ), Capability.Read


async def handle_linode_account_invoice_items_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_invoice_items_list tool request."""
    invoice_id = arguments.get("invoice_id")
    if (
        not isinstance(invoice_id, int)
        or isinstance(invoice_id, bool)
        or invoice_id < 1
    ):
        return error_response("invoice_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_invoice_items(
            invoice_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "list Linode account invoice items", _call
    )


def create_linode_account_event_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_get tool."""
    return Tool(
        name="linode_account_event_get",
        description="Gets a Linode account event by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "event_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Account event ID to retrieve",
                },
            },
            "required": ["event_id"],
        },
    ), Capability.Read


async def handle_linode_account_event_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_event_get tool request."""
    event_id = arguments.get("event_id")
    if not isinstance(event_id, int) or isinstance(event_id, bool) or event_id < 1:
        return error_response("event_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_event(event_id)

    return await execute_tool(cfg, arguments, "get Linode account event", _call)


def create_linode_account_event_seen_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_seen tool."""
    return Tool(
        name="linode_account_event_seen",
        description=(
            "Marks a Linode account event as seen. "
            "Pass dry_run=true to preview without marking the event seen."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "event_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Account event ID to mark as seen",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["event_id", "confirm"],
        },
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
                    "The specified account event and all earlier events are "
                    "marked as seen."
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
            "This marks an account event seen. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.mark_account_event_seen(event_id)

    return await execute_tool(cfg, arguments, "mark Linode account event seen", _call)


_ACCOUNT_AGREEMENT_FIELDS = (
    "billing_agreement",
    "eu_model",
    "master_service_agreement",
    "privacy_policy",
)


def create_linode_account_agreements_acknowledge_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_agreements_acknowledge tool."""
    return Tool(
        name="linode_account_agreements_acknowledge",
        description="Acknowledges agreements on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "billing_agreement": {
                    "type": "boolean",
                    "description": "Acknowledge the billing agreement",
                },
                "eu_model": {
                    "type": "boolean",
                    "description": "Acknowledge the EU model agreement",
                },
                "master_service_agreement": {
                    "type": "boolean",
                    "description": "Acknowledge the master service agreement",
                },
                "privacy_policy": {
                    "type": "boolean",
                    "description": "Acknowledge the privacy policy",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_account_agreements_acknowledge(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_agreements_acknowledge tool request."""
    agreements: dict[str, bool] = {}
    for field in _ACCOUNT_AGREEMENT_FIELDS:
        value = arguments.get(field)
        if value is None:
            continue
        if not isinstance(value, bool):
            return error_response(f"{field} must be a boolean")
        agreements[field] = value

    if not agreements:
        return error_response("At least one account agreement field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_agreements_acknowledge",
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
        return await client.acknowledge_account_agreements(agreements)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "id": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Beta program ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to acknowledge this mutating operation. "
                        "Required even when dry_run=true; dry_run still "
                        "avoids the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_beta_enroll(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_enroll tool request."""
    raw_beta_id = arguments.get("id")
    if raw_beta_id is None:
        return error_response("id is required")
    if not isinstance(raw_beta_id, str):
        return error_response("id must be a string")

    beta_id = raw_beta_id.strip()
    if not beta_id:
        return error_response("id is required")

    if arguments.get("confirm") is not True:
        return error_response(
            "This enrolls the account in a beta program. Set confirm=true to proceed."
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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.enroll_account_beta(beta_id)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Label for the OAuth client",
                },
                "redirect_uri": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Redirect URI for the OAuth client",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm live OAuth client creation. "
                        "When dry_run=true, the request is previewed without "
                        "creating the client."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "redirect_uri", "confirm"],
        },
    ), Capability.Write


def create_linode_account_oauth_client_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_delete tool."""
    return Tool(
        name="linode_account_oauth_client_delete",
        description=(
            "Deletes an account OAuth client by client ID. "
            "Pass dry_run=true to preview without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "string",
                    "minLength": 1,
                    "pattern": _ACCOUNT_OAUTH_CLIENT_ID_PATTERN_TEXT,
                    "description": "OAuth client ID to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm OAuth client deletion.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["client_id", "confirm"],
        },
    ), Capability.Destroy


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
        return await client.delete_account_oauth_client(client_id)

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
                "A new account OAuth client is created. The returned client "
                "secret is shown once and cannot be retrieved later."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an OAuth client and returns a one-time secret. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_account_oauth_client(label, redirect_uri)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "type": {
                    "type": "string",
                    "enum": ["credit_card"],
                    "description": "Payment method type",
                },
                "data": {
                    "type": "object",
                    "description": "Payment method provider data",
                },
                "is_default": {
                    "type": "boolean",
                    "description": "Whether to make this the default payment method",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm live payment method creation. "
                        "When dry_run=true, the request is previewed without "
                        "creating the payment method."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["type", "data", "is_default", "confirm"],
        },
    ), Capability.Write


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
                "A new account payment method is created and may become the "
                "default payment method."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an account payment method. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_account_payment_method(
            str(request_body["type"]),
            cast("dict[str, Any]", request_body["data"]),
            bool(request_body["is_default"]),
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "promo_code": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Promo code to apply to the account",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm applying this promo code. "
                        "Required even when dry_run=true; dry_run still "
                        "avoids the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["promo_code", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_promo_credit_add(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_promo_credit_add tool request."""
    promo_code, promo_code_error = _required_nonempty_string_argument(
        arguments, "promo_code"
    )
    if promo_code_error is not None or promo_code is None:
        return error_response(promo_code_error or "promo_code is required")

    if arguments.get("confirm") is not True:
        return error_response(
            "This applies a promo credit to the account. Set confirm=true to proceed."
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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.add_account_promo_credit(promo_code)

    return await execute_tool(cfg, arguments, "add Linode account promo credit", _call)


def create_linode_account_cancel_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_cancel tool."""
    return Tool(
        name="linode_account_cancel",
        description=(
            "Cancels the Linode account. "
            "Pass dry_run=true to preview without canceling."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "comments": {
                    "type": "string",
                    "description": "Optional cancellation comments",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this destructive operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Destroy


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
                "Account cancellation is permanent and irreversible; every "
                "resource on the account is destroyed and access is lost."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This cancels the Linode account. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.cancel_account(comments=comments)

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
_ACCOUNT_USER_UPDATE_STRING_FIELDS = frozenset({"email", "username"})
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
        "current_username",
        *_ACCOUNT_USER_UPDATE_FIELDS,
    }
)
_ACCOUNT_USER_USERNAME_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")


def _validate_account_user_update_username(
    value: object,
) -> tuple[str | None, str | None]:
    if value is None:
        return None, "current_username is required"
    if not isinstance(value, str):
        return None, "current_username must be a string"
    username = value.strip()
    if not username:
        return None, "current_username is required"
    if username != value or not _ACCOUNT_USER_USERNAME_PATTERN.fullmatch(username):
        return (
            None,
            "current_username must contain only letters, numbers, underscores, "
            "and hyphens",
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
    return {
        key: arguments[key]
        for key in _ACCOUNT_USER_UPDATE_FIELDS
        if arguments.get(key) is not None
    }


def create_linode_account_settings_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_settings_update tool."""
    return Tool(
        name="linode_account_settings_update",
        description=(
            "Updates Linode account-wide settings. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "backups_enabled": {
                    "type": "boolean",
                    "description": "Whether backups are enabled by default",
                },
                "interfaces_for_new_linodes": {
                    "type": "string",
                    "description": "Default interface setting for new Linodes",
                },
                "longview_subscription": {
                    "type": "string",
                    "description": "Longview subscription tier",
                },
                "maintenance_policy": {
                    "type": "string",
                    "description": "Account maintenance policy",
                },
                "managed": {
                    "type": "boolean",
                    "description": "Whether Linode Managed is enabled",
                },
                "network_helper": {
                    "type": "boolean",
                    "description": "Whether Network Helper is enabled",
                },
                "object_storage": {
                    "type": "string",
                    "description": "Object Storage account setting",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


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
            "This updates account settings. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_account_settings(**update_fields)
        return {
            "message": "Account settings updated successfully",
            "settings": result,
        }

    return await execute_tool(cfg, arguments, "update Linode account settings", _call)


def create_linode_account_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_update tool."""
    return Tool(
        name="linode_account_update",
        description=(
            "Updates Linode account contact and billing-address information. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "first_name": {"type": "string", "description": "First name"},
                "last_name": {"type": "string", "description": "Last name"},
                "email": {"type": "string", "description": "Contact email"},
                "company": {"type": "string", "description": "Company name"},
                "address_1": {"type": "string", "description": "Address line 1"},
                "address_2": {"type": "string", "description": "Address line 2"},
                "city": {"type": "string", "description": "City"},
                "state": {"type": "string", "description": "State or province"},
                "zip": {"type": "string", "description": "Postal code"},
                "country": {"type": "string", "description": "Country code"},
                "phone": {"type": "string", "description": "Phone number"},
                "tax_id": {"type": "string", "description": "Tax ID"},
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


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
            "This updates account information. Set confirm=true to proceed."
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
        account = await client.update_account(**update_fields)
        return {
            "message": "Account updated successfully",
            "account": {
                "first_name": account.first_name,
                "last_name": account.last_name,
                "email": account.email,
                "company": account.company,
                "address_1": account.address_1,
                "address_2": account.address_2,
                "city": account.city,
                "state": account.state,
                "zip": account.zip,
                "country": account.country,
                "phone": account.phone,
            },
        }

    return await execute_tool(cfg, arguments, "update Linode account", _call)


def create_linode_account_user_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_update tool."""
    return Tool(
        name="linode_account_user_update",
        description=(
            "Updates an account user by username. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "current_username": {
                    "type": "string",
                    "description": "Current username to update",
                },
                "email": {"type": "string", "description": "User email address"},
                "restricted": {
                    "type": "boolean",
                    "description": "Whether the user is restricted",
                },
                "ssh_keys": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "SSH keys for the user",
                },
                "username": {"type": "string", "description": "New username"},
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. "
                        "When dry_run=true, the request is previewed without "
                        "updating the user."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["current_username", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_user_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_user_update tool request."""
    current_username, username_error = _validate_account_user_update_username(
        arguments.get("current_username")
    )
    if username_error is not None or current_username is None:
        return error_response(username_error or "current_username is required")

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
        return {
            "message": "Account user updated successfully",
            "user": result,
        }

    return await execute_tool(cfg, arguments, "update Linode account user", _call)


_MIN_REGION_ID_PARTS = 2


def _is_region_id(value: str) -> bool:
    """Return True when value looks like a Linode region ID slug."""
    parts = value.split("-")
    return len(parts) >= _MIN_REGION_ID_PARTS and all(
        part and all("0" <= c <= "9" or "a" <= c <= "z" for c in part) for part in parts
    )


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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "beta_id": {
                    "type": "string",
                    "description": "Beta program ID to retrieve",
                },
            },
            "required": ["beta_id"],
        },
    ), Capability.Read


async def handle_linode_account_beta_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_get tool request."""
    raw_beta_id = arguments.get("beta_id")
    if raw_beta_id is None:
        return error_response("beta_id is required")
    if not isinstance(raw_beta_id, str):
        return error_response("beta_id must be a string")

    beta_id = raw_beta_id.strip()
    if not beta_id:
        return error_response("beta_id is required")
    if "/" in beta_id or "?" in beta_id or ".." in beta_id:
        return error_response("beta_id must not contain '/', '?', or '..'")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_beta(beta_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account beta {beta_id}", _call
    )


def create_linode_account_child_account_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_account_get tool."""
    return Tool(
        name="linode_account_child_account_get",
        description="Gets a child account by EUUID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "euuid": {
                    "type": "string",
                    "description": "Child account EUUID to retrieve",
                },
            },
            "required": ["euuid"],
        },
    ), Capability.Read


async def handle_linode_account_child_account_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_account_get tool request."""
    raw_euuid = arguments.get("euuid")
    if raw_euuid is None:
        return error_response("euuid is required")
    if not isinstance(raw_euuid, str):
        return error_response("euuid must be a string")

    euuid = raw_euuid.strip()
    if not euuid:
        return error_response("euuid is required")
    if "/" in euuid or "?" in euuid or ".." in euuid:
        return error_response("euuid must not contain '/', '?', or '..'")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_child_account(euuid)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "token": {
                    "type": "string",
                    "description": "Service transfer token to accept",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. "
                        "Required even when dry_run=true; dry_run still avoids "
                        "the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["token", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_service_transfer_accept(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_accept tool request."""
    token, validation_error = _validate_service_transfer_token(arguments.get("token"))
    if validation_error is not None:
        return error_response(validation_error)
    token = cast("str", token)

    if arguments.get("confirm") is not True:
        return error_response(
            "This accepts an account service transfer. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.accept_account_service_transfer(token)

    return await execute_tool(
        cfg, arguments, f"accept Linode account service transfer {token}", _call
    )


def create_linode_account_service_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfer_get tool."""
    return Tool(
        name="linode_account_service_transfer_get",
        description="Gets an account service transfer request by token.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "token": {
                    "type": "string",
                    "description": "Service transfer token to retrieve",
                },
            },
            "required": ["token"],
        },
    ), Capability.Read


async def handle_linode_account_service_transfer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfer_get tool request."""
    token, message = _validate_service_transfer_token(arguments.get("token"))
    if token is None:
        return error_response(message or "token is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_service_transfer(token)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "token": {
                    "type": "string",
                    "description": "Service transfer token to cancel",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this destructive operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["token", "confirm"],
        },
    ), Capability.Destroy


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
            "This cancels a service transfer. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.delete_account_service_transfer(token)
        return {
            "message": "Service transfer canceled successfully",
            "result": result,
        }

    return await execute_tool(
        cfg, arguments, f"cancel Linode account service transfer {token}", _call
    )


def create_linode_account_oauth_client_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_get tool."""
    return Tool(
        name="linode_account_oauth_client_get",
        description="Gets an OAuth client by client ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "string",
                    "description": "OAuth client ID to retrieve",
                },
            },
            "required": ["client_id"],
        },
    ), Capability.Read


async def handle_linode_account_oauth_client_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_get tool request."""
    client_id, error = _validated_oauth_client_id(arguments)
    if error is not None or client_id is None:
        return error_response(error or "client_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_oauth_client(client_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account OAuth client {client_id}", _call
    )


def create_linode_account_oauth_client_reset_secret_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_reset_secret tool."""
    return Tool(
        name="linode_account_oauth_client_reset_secret",
        description=(
            "Resets an OAuth client secret. The new secret is only shown once "
            "in the response. Pass dry_run=true to preview without resetting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "string",
                    "description": "OAuth client ID whose secret will be reset",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm live OAuth client secret reset. "
                        "When dry_run=true, the request is previewed without "
                        "resetting the secret."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["client_id", "confirm"],
        },
    ), Capability.Write


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


async def handle_linode_account_oauth_client_reset_secret(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_reset_secret tool request."""
    client_id, error = _validated_oauth_client_id(arguments)
    if error is not None or client_id is None:
        return error_response(error or "client_id is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_oauth_client_reset_secret",
            arguments.get("environment", ""),
            "POST",
            f"/account/oauth-clients/{quote(client_id, safe='')}/reset-secret",
            None,
            side_effects=[
                "The OAuth client secret is reset. The replacement secret is "
                "shown once and cannot be retrieved later."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This resets the OAuth client secret and returns a one-time secret. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.reset_account_oauth_client_secret(client_id)

    return await execute_tool(
        cfg, arguments, f"reset Linode account OAuth client secret {client_id}", _call
    )


def create_linode_account_oauth_client_thumbnail_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_oauth_client_thumbnail_get tool."""
    return Tool(
        name="linode_account_oauth_client_thumbnail_get",
        description="Gets an OAuth client's thumbnail by client ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "string",
                    "description": "OAuth client ID whose thumbnail to retrieve",
                },
            },
            "required": ["client_id"],
        },
    ), Capability.Read


async def handle_linode_account_oauth_client_thumbnail_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_oauth_client_thumbnail_get tool request."""
    client_id, error = _validated_oauth_client_id(arguments)
    if error is not None or client_id is None:
        return error_response(error or "client_id is required")

    async def _call(client: RetryableClient) -> dict[str, str]:
        return await client.get_account_oauth_client_thumbnail(client_id)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "invoice_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Invoice ID to retrieve",
                },
            },
            "required": ["invoice_id"],
        },
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
        return await client.get_account_invoice(raw_invoice_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account invoice {raw_invoice_id}", _call
    )


def create_linode_account_payment_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_get tool."""
    return Tool(
        name="linode_account_payment_get",
        description="Gets a payment on the Linode account by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "payment_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Payment ID to retrieve",
                },
            },
            "required": ["payment_id"],
        },
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
        return await client.get_account_payment(raw_payment_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account payment {raw_payment_id}", _call
    )


def create_linode_account_payment_method_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_payment_method_get tool."""
    return Tool(
        name="linode_account_payment_method_get",
        description="Gets a payment method on the Linode account by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "payment_method_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Payment method ID to retrieve",
                },
            },
            "required": ["payment_method_id"],
        },
    ), Capability.Read


async def handle_linode_account_payment_method_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_get tool request."""
    raw_payment_method_id = arguments.get("payment_method_id")
    if raw_payment_method_id is None:
        return error_response("payment_method_id is required")
    if not isinstance(raw_payment_method_id, int) or isinstance(
        raw_payment_method_id, bool
    ):
        return error_response("payment_method_id must be an integer")
    if raw_payment_method_id < 1:
        return error_response("payment_method_id must be at least 1")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_payment_method(raw_payment_method_id)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "payment_method_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Payment method ID to set as default",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["payment_method_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_payment_method_make_default(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_payment_method_make_default tool request."""
    raw_payment_method_id = arguments.get("payment_method_id")
    if raw_payment_method_id is None:
        return error_response("payment_method_id is required")
    if not isinstance(raw_payment_method_id, int) or isinstance(
        raw_payment_method_id, bool
    ):
        return error_response("payment_method_id must be an integer")
    if raw_payment_method_id < 1:
        return error_response("payment_method_id must be at least 1")

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
        result = await client.make_account_payment_method_default(raw_payment_method_id)
        return {
            "message": "Default payment method updated successfully",
            "payment_method": result,
        }

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "login_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Login ID to retrieve",
                },
            },
            "required": ["login_id"],
        },
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
        return await client.get_account_login(raw_login_id)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "username": {
                    "type": "string",
                    "minLength": 1,
                    "pattern": _ACCOUNT_USERNAME_PATTERN_TEXT,
                    "description": "Username to delete from the account",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm account user deletion.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["username", "confirm"],
        },
    ), Capability.Destroy


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

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes an account user. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.delete_account_user(username)
        return {
            "message": "Account user deleted successfully",
            "result": result,
        }

    return await execute_tool(
        cfg, arguments, f"delete Linode account user {username}", _call
    )


def create_linode_account_user_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_get tool."""
    return Tool(
        name="linode_account_user_get",
        description="Gets an account user by username.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "username": {
                    "type": "string",
                    "pattern": _ACCOUNT_USERNAME_PATTERN_TEXT,
                    "description": "Username to retrieve",
                },
            },
            "required": ["username"],
        },
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
        return await client.get_account_user(username)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account user {username}", _call
    )


def create_linode_account_user_grants_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_user_grants_get tool."""
    return Tool(
        name="linode_account_user_grants_get",
        description="Lists grants for an account user by username.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "username": {
                    "type": "string",
                    "pattern": _ACCOUNT_USERNAME_PATTERN_TEXT,
                    "description": "Username whose grants to list",
                },
            },
            "required": ["username"],
        },
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
        return await client.get_account_user_grants(username)

    return await execute_tool(
        cfg, arguments, f"list Linode account user grants for {username}", _call
    )


def create_linode_account_availability_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_availability_list tool."""
    return Tool(
        name="linode_account_availability_list",
        description="Lists available Linode services for the account.",
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


async def handle_linode_account_availability_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_availability_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_availability(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account availability", _call)


def create_linode_account_availability_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_availability_get tool."""
    return Tool(
        name="linode_account_availability_get",
        description=(
            "Gets available Linode services for the account in a specific region."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "region_id": {
                    "type": "string",
                    "description": "Region ID to check (for example, 'us-east')",
                },
            },
            "required": ["region_id"],
        },
    ), Capability.Read


async def handle_linode_account_availability_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_availability_get tool request."""
    raw_region_id = arguments.get("region_id")
    if raw_region_id is None:
        return error_response("region_id is required")
    if not isinstance(raw_region_id, str):
        return error_response("region_id must be a string")

    region_id = raw_region_id.strip()
    if not region_id:
        return error_response("region_id is required")
    if not _is_region_id(region_id):
        return error_response(
            "region_id must be a lowercase region slug with letters, "
            "numbers, and hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_availability(region_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account availability for {region_id}", _call
    )


def create_linode_account_betas_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_betas_list tool."""
    return Tool(
        name="linode_account_betas_list",
        description="Lists enrolled Beta programs for the account.",
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


async def handle_linode_account_betas_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_betas_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_betas(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account betas", _call)


def create_linode_betas_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_betas_list tool."""
    return Tool(
        name="linode_betas_list",
        description="Lists available Beta programs.",
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


async def handle_linode_betas_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_betas_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_betas(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode beta programs", _call)


def create_linode_account_child_accounts_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_accounts_list tool."""
    return Tool(
        name="linode_account_child_accounts_list",
        description="Lists child accounts for the Linode account.",
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


async def handle_linode_account_child_accounts_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_accounts_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_child_accounts(page=page, page_size=page_size)

    return await execute_tool(
        cfg, arguments, "list Linode account child accounts", _call
    )


def create_linode_account_service_transfers_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_service_transfers_list tool."""
    return Tool(
        name="linode_account_service_transfers_list",
        description="Lists service transfers for the Linode account.",
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


async def handle_linode_account_service_transfers_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_service_transfers_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_service_transfers(
            page=page, page_size=page_size
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "euuid": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Child account EUUID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this credential-creating operation. "
                        "Required even when dry_run=true; dry_run still avoids "
                        "the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["euuid", "confirm"],
        },
    ), Capability.Write


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

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a child account proxy token. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_account_child_account_token(euuid)

    return await execute_tool(
        cfg, arguments, "create Linode account child account proxy token", _call
    )


def create_linode_account_tags_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tags_list tool."""
    return Tool(
        name="linode_account_tags_list",
        description="Lists tags on the Linode account.",
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


async def handle_linode_account_tags_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tags_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_tags(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account tags", _call)


def create_linode_account_tag_objects_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_objects_list tool."""
    return Tool(
        name="linode_account_tag_objects_list",
        description="Lists objects assigned to a Linode account tag.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tag_label": {
                    "type": "string",
                    "description": "Label of the tag to inspect",
                },
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
            "required": ["tag_label"],
        },
    ), Capability.Read


async def handle_linode_account_tag_objects_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_objects_list tool request."""
    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_tagged_objects(
            tag_label, page=page, page_size=page_size
        )

    return await execute_tool(cfg, arguments, "list tagged objects", _call)


def create_linode_account_tag_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_create tool."""
    return Tool(
        name="linode_account_tag_create",
        description="Creates a Linode account tag and optionally assigns resources.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {"type": "string", "description": "Tag label to create"},
                "domains": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Domain IDs to assign to the tag",
                },
                "linodes": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Linode IDs to assign to the tag",
                },
                "nodebalancers": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "NodeBalancer IDs to assign to the tag",
                },
                "volumes": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Volume IDs to assign to the tag",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "confirm"],
        },
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


async def handle_linode_account_tag_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_create tool request."""
    if is_dry_run(arguments):
        label = arguments.get("label")
        if not isinstance(label, str) or not label.strip():
            return error_response("label is required")
        return build_dry_run_response(
            "linode_account_tag_create",
            arguments.get("environment", ""),
            "POST",
            "/tags",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response("This creates a tag. Set confirm=true to proceed.")

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
        return {"message": f"Tag '{label.strip()}' created successfully", "tag": tag}

    return await execute_tool(cfg, arguments, "create Linode tag", _call)


def create_linode_account_tag_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_delete tool."""
    return Tool(
        name="linode_account_tag_delete",
        description="Deletes a Linode account tag by label.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tag_label": {
                    "type": "string",
                    "description": "Label of the tag to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["tag_label", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_account_tag_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_delete tool request."""
    if is_dry_run(arguments):
        tag_label = arguments.get("tag_label")
        if not isinstance(tag_label, str) or not tag_label.strip():
            return error_response("tag_label is required")
        return build_dry_run_response(
            "linode_account_tag_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/tags/{tag_label}",
            None,
        )

    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")
    if not arguments.get("confirm"):
        return error_response("This deletes a tag. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_tag(tag_label)
        return {"message": f"Tag '{tag_label}' deleted successfully"}

    return await execute_tool(cfg, arguments, "delete Linode tag", _call)


def create_linode_account_support_ticket_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_create tool."""
    return Tool(
        name="linode_account_support_ticket_create",
        description="Opens a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "summary": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 64,
                    "description": "Support ticket summary or title",
                },
                "description": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 65000,
                    "description": "Full details of the issue or question",
                },
                "bucket": {
                    "type": "string",
                    "description": "Object Storage bucket name",
                },
                "database_id": {"type": "integer", "minimum": 1},
                "domain_id": {"type": "integer", "minimum": 1},
                "firewall_id": {"type": "integer", "minimum": 1},
                "linode_id": {"type": "integer", "minimum": 1},
                "lkecluster_id": {"type": "integer", "minimum": 1},
                "longviewclient_id": {"type": "integer", "minimum": 1},
                "managed_issue": {"type": "boolean"},
                "nodebalancer_id": {"type": "integer", "minimum": 1},
                "region": {"type": "string", "description": "Region ID"},
                "severity": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 3,
                    "description": "Ticket severity: 1 major, 2 moderate, 3 low",
                },
                "vlan": {"type": "string", "description": "VLAN label"},
                "volume_id": {"type": "integer", "minimum": 1},
                "vpc_id": {"type": "integer", "minimum": 1},
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["summary", "description", "confirm"],
        },
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


def _attachment_file(arguments: dict[str, Any]) -> str | list[TextContent]:
    """Parse the attachment file path, or return an error response."""
    file = arguments.get("file")
    if not isinstance(file, str) or not file.strip():
        return error_response("file is required")
    file_path = file.strip()
    if not Path(file_path).is_absolute():
        return error_response("file must be a local, absolute path")
    return file_path


async def handle_linode_account_support_ticket_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_create tool request."""
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
            "linode_account_support_ticket_create",
            arguments.get("environment", ""),
            "POST",
            "/support/tickets",
            None,
            side_effects=[effect],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This opens a support ticket. Set confirm=true to proceed."
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
        return {"message": "Support ticket opened successfully", "ticket": ticket}

    return await execute_tool(cfg, arguments, "open Linode support ticket", _call)


def create_linode_managed_credential_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credential_get tool."""
    return Tool(
        name="linode_managed_credential_get",
        description="Gets a Linode Managed credential by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "credential_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed credential ID to retrieve",
                },
            },
            "required": ["credential_id"],
        },
    ), Capability.Read


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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_managed_credential(validated_credential_id)

    return await execute_tool(cfg, arguments, "get Linode Managed credential", _call)


def create_linode_managed_contacts_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contacts_list tool."""
    return Tool(
        name="linode_managed_contacts_list",
        description="Lists Managed contacts on the Linode account.",
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


async def handle_linode_managed_contacts_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contacts_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_managed_contacts(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode Managed contacts", _call)


def create_linode_managed_issues_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_issues_list tool."""
    return Tool(
        name="linode_managed_issues_list",
        description="Lists open Managed issues on the Linode account.",
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


async def handle_linode_managed_issues_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_issues_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_managed_issues(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode Managed issues", _call)


def create_linode_managed_credentials_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_credentials_list tool."""
    return Tool(
        name="linode_managed_credentials_list",
        description="Lists Managed credentials on the Linode account.",
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


async def handle_linode_managed_credentials_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credentials_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_managed_credentials(page=page, page_size=page_size)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "credential_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed credential ID to update",
                },
                "password": {"type": "string"},
                "username": {"type": "string"},
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm credential update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["credential_id", "password", "confirm"],
        },
    ), Capability.Write


async def handle_linode_managed_credential_username_password_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_username_password_update tool request."""
    credential_id = _managed_credential_id(arguments)
    if isinstance(credential_id, list):
        return credential_id

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Managed credential. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_managed_credential_username_password(
            credential_id,
            password=body["password"],
            username=body.get("username"),
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "credential_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed credential ID to revoke",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm revoking or previewing "
                        "the Managed credential."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["credential_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_managed_credential_revoke(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_revoke tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes a Managed credential. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.revoke_managed_credential(credential_id)

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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Linode ID whose Managed settings to update",
                },
                "ssh": {
                    "type": "object",
                    "description": "SSH Managed settings to update for this Linode",
                    "additionalProperties": False,
                    "minProperties": 1,
                    "properties": {
                        "access": {"type": "boolean"},
                        "ip": {"type": "string", "minLength": 1},
                        "port": {
                            "anyOf": [
                                {"type": "integer", "minimum": 1, "maximum": 65535},
                                {"type": "null"},
                            ],
                        },
                        "user": {
                            "anyOf": [
                                {"type": "string", "maxLength": 32},
                                {"type": "null"},
                            ],
                        },
                    },
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating Managed settings "
                        "operation. Required even when dry_run=true; dry_run "
                        "still avoids the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "ssh", "confirm"],
        },
    ), Capability.Write


def create_linode_managed_ssh_key_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_ssh_key_get tool."""
    return Tool(
        name="linode_managed_ssh_key_get",
        description="Gets the Managed SSH public key for the Linode account.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
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

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates Managed settings for a Linode. Set confirm=true to proceed."
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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_managed_linode_settings(linode_id, ssh=ssh)
        return {
            "message": "Managed Linode settings updated successfully",
            "settings": result,
        }

    return await execute_tool(
        cfg,
        arguments,
        f"update Managed settings for Linode {linode_id}",
        _call,
    )


async def handle_linode_managed_ssh_key_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_ssh_key_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_managed_ssh_key()

    return await execute_tool(cfg, arguments, "get Linode Managed SSH key", _call)


_MANAGED_CONTACT_BODY_FIELDS = ("email", "group", "name", "phone")
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {"type": "string"},
                "password": {"type": "string"},
                "username": {"type": "string"},
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm creating or previewing "
                        "the Managed credential."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "password", "confirm"],
        },
    ), Capability.Write


async def handle_linode_managed_credential_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_credential_create tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a Managed credential. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_managed_credential(
            label=body["label"],
            password=body["password"],
            username=body.get("username"),
        )

    return await execute_tool(cfg, arguments, "create Managed credential", _call)


def _managed_credential_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive managed credential ID, or return an error response."""
    credential_id = arguments.get("credential_id")
    if (
        not isinstance(credential_id, int)
        or isinstance(credential_id, bool)
        or credential_id < 1
    ):
        return error_response("credential_id must be a positive integer")
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "credential_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed credential ID to update",
                },
                "label": {
                    "type": "string",
                    "description": "Managed credential label",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["credential_id", "label", "confirm"],
        },
    ), Capability.Write


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

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Managed credential. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_managed_credential(
            credential_id,
            label=body["label"],
        )

    return await execute_tool(cfg, arguments, "update Linode Managed credential", _call)


def _managed_contact_body(arguments: dict[str, Any]) -> dict[str, str]:
    """Collect documented Managed contact body fields from tool arguments."""
    body = {
        field: value
        for field in _MANAGED_CONTACT_BODY_FIELDS
        if (value := _optional_string_argument(arguments, field)) is not None
    }
    if not body:
        raise ValueError("At least one of email, group, name, or phone is required")
    return body


def create_linode_managed_contact_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_create tool."""
    body_properties = {
        field: {"type": "string"} for field in _MANAGED_CONTACT_BODY_FIELDS
    }
    return Tool(
        name="linode_managed_contact_create",
        description=(
            "Creates a Managed contact. Pass confirm=true to create it; "
            "pass dry_run=true to preview without creating it."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                **body_properties,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm creating or previewing "
                        "the Managed contact."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_managed_contact_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contact_create tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a Managed contact. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_managed_contact(
            email=body.get("email"),
            group=body.get("group"),
            name=body.get("name"),
            phone=body.get("phone"),
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
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "contact_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed contact ID to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm Managed contact deletion.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["contact_id", "confirm"],
        },
    ), Capability.Destroy


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

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Managed contact. Set confirm=true to proceed."
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.delete_managed_contact(contact_id)
        return {"message": "Managed contact deleted successfully", "result": result}

    return await execute_tool(cfg, arguments, "delete Managed contact", _call)


def create_linode_managed_contacts_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contacts_update tool."""
    return Tool(
        name="linode_managed_contacts_update",
        description=(
            "Updates a Managed contact. Requires confirm=true; pass "
            "dry_run=true with confirm=true to preview without changing it."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "contact_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed contact ID to update",
                },
                "email": {
                    "type": "string",
                    "description": "Email address for issue alerts",
                },
                "group": {
                    "type": ["string", "null"],
                    "description": "Display grouping for this contact",
                },
                "name": {
                    "type": "string",
                    "description": "Managed contact name",
                },
                "phone": {
                    "type": "object",
                    "description": "Phone contact details",
                    "properties": {
                        "primary": {"type": ["string", "null"]},
                        "secondary": {"type": ["string", "null"]},
                    },
                    "additionalProperties": False,
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["contact_id", "confirm"],
        },
    ), Capability.Write


def _managed_contact_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive managed contact ID, or return an error response."""
    contact_id = arguments.get("contact_id")
    if (
        not isinstance(contact_id, int)
        or isinstance(contact_id, bool)
        or contact_id < 1
    ):
        return error_response("contact_id must be a positive integer")
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


async def handle_linode_managed_contacts_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_contacts_update tool request."""
    contact_id = _managed_contact_id(arguments)
    if isinstance(contact_id, list):
        return contact_id

    body = _managed_contact_update_body(arguments)
    if isinstance(body, list):
        return body

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Managed contact. Set confirm=true to proceed."
        )

    dry_run_path = f"/managed/contacts/{quote(str(contact_id), safe='')}"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_managed_contacts_update",
            arguments.get("environment", ""),
            "PUT",
            dry_run_path,
            None,
            side_effects=[f"Managed contact {contact_id} will be updated."],
            request_body=body,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_managed_contact(contact_id, **body)

    return await execute_tool(cfg, arguments, "update Linode Managed contact", _call)


def create_linode_managed_linode_settings_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_linode_settings_list tool."""
    return Tool(
        name="linode_managed_linode_settings_list",
        description="Lists Managed Linode settings on the account.",
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


async def handle_linode_managed_linode_settings_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_linode_settings_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_managed_linode_settings(page=page, page_size=page_size)

    return await execute_tool(
        cfg, arguments, "list Linode Managed Linode settings", _call
    )


def create_linode_managed_stats_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_stats tool."""
    return Tool(
        name="linode_managed_stats",
        description="Lists Managed statistics from the last 24 hours.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_managed_stats(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_stats tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_managed_stats()

    return await execute_tool(cfg, arguments, "list Linode Managed statistics", _call)


def create_linode_managed_linode_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_linode_settings_get tool."""
    return Tool(
        name="linode_managed_linode_settings_get",
        description="Gets Managed settings for a Linode.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Linode ID whose Managed settings to retrieve",
                },
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_managed_linode_settings_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_linode_settings_get tool request."""
    linode_id = arguments.get("linode_id")
    if isinstance(linode_id, bool) or not isinstance(linode_id, int) or linode_id < 1:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_managed_linode_settings(linode_id)

    return await execute_tool(cfg, arguments, "get Linode Managed settings", _call)


def create_linode_managed_issue_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_issue_get tool."""
    return Tool(
        name="linode_managed_issue_get",
        description="Gets a Linode Managed issue by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "issue_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed issue ID to retrieve",
                },
            },
            "required": ["issue_id"],
        },
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
        return await client.get_managed_issue(validated_issue_id)

    return await execute_tool(cfg, arguments, "get Linode Managed issue", _call)


def create_linode_managed_contact_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_contact_get tool."""
    return Tool(
        name="linode_managed_contact_get",
        description="Gets a Linode Managed contact by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "contact_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Managed contact ID to retrieve",
                },
            },
            "required": ["contact_id"],
        },
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
        return await client.get_managed_contact(validated_contact_id)

    return await execute_tool(cfg, arguments, "get Linode Managed contact", _call)


def create_linode_account_support_tickets_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_tickets_list tool."""
    return Tool(
        name="linode_account_support_tickets_list",
        description="Lists Linode support tickets.",
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


async def handle_linode_account_support_tickets_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_tickets_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_support_tickets(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode support tickets", _call)


def create_linode_account_support_ticket_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_get tool."""
    return Tool(
        name="linode_account_support_ticket_get",
        description="Gets a Linode support ticket by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to retrieve",
                },
            },
            "required": ["ticket_id"],
        },
    ), Capability.Read


async def handle_linode_account_support_ticket_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_get tool request."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_support_ticket(ticket_id)

    return await execute_tool(cfg, arguments, "get Linode support ticket", _call)


def create_linode_account_support_ticket_replies_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_replies_list tool."""
    return Tool(
        name="linode_account_support_ticket_replies_list",
        description="Lists replies on a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID whose replies to list",
                },
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
            "required": ["ticket_id"],
        },
    ), Capability.Read


async def handle_linode_account_support_ticket_replies_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_replies_list tool request."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_support_ticket_replies(
            ticket_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "list Linode support ticket replies", _call
    )


def create_linode_account_support_ticket_close_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_close tool."""
    return Tool(
        name="linode_account_support_ticket_close",
        description="Closes a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to close",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["ticket_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_support_ticket_close(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_close tool request."""
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
            "linode_account_support_ticket_close",
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
        closed_ticket = await client.close_support_ticket(ticket_id)
        return {
            "message": "Support ticket closed successfully",
            "ticket": closed_ticket,
        }

    return await execute_tool(cfg, arguments, "close Linode support ticket", _call)


def create_linode_account_support_ticket_reply_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_reply_create tool."""
    return Tool(
        name="linode_account_support_ticket_reply_create",
        description="Creates a reply on a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to reply to",
                },
                "description": {
                    "type": "string",
                    "description": "Reply body to add to the support ticket",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["ticket_id", "description", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_support_ticket_reply_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_reply_create tool request."""
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
            "linode_account_support_ticket_reply_create",
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
        return {"message": "Support ticket reply created successfully", "reply": reply}

    return await execute_tool(
        cfg, arguments, "create Linode support ticket reply", _call
    )


def create_linode_account_support_ticket_attachment_create_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_account_support_ticket_attachment_create tool."""
    return Tool(
        name="linode_account_support_ticket_attachment_create",
        description="Creates an attachment on a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to attach the file to",
                },
                "file": {
                    "type": "string",
                    "description": ("Local, absolute path to the file to attach"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["ticket_id", "file", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_support_ticket_attachment_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_attachment_create tool request."""
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
            "linode_account_support_ticket_attachment_create",
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
        attachment = await client.create_support_ticket_attachment(ticket_id, file_path)
        return {
            "message": "Support ticket attachment created successfully",
            "attachment": attachment,
        }

    return await execute_tool(
        cfg, arguments, "create Linode support ticket attachment", _call
    )

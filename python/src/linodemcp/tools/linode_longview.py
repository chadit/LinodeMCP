"""Longview tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}
_CLIENT_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "Longview client ID to update (required)",
}
_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Set true to confirm this mutating operation.",
}


def create_linode_longview_client_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_update tool."""
    return Tool(
        name="linode_longview_client_update",
        description=(
            "Updates a Longview client label. Pass dry_run=true to preview "
            "without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "client_id": _CLIENT_ID_PROP,
                "label": {"type": "string", "description": "Longview client label"},
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["client_id", "label", "confirm"],
        },
    ), Capability.Write


def create_linode_longview_subscription_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_subscription_get tool."""
    return Tool(
        name="linode_longview_subscription_get",
        description="Gets details for a single Longview subscription by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "subscription_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Longview subscription ID to retrieve",
                },
            },
            "required": ["subscription_id"],
        },
    ), Capability.Read


def create_linode_longview_plan_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_plan_get tool."""
    return Tool(
        name="linode_longview_plan_get",
        description="Gets the account Longview plan.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


def create_linode_longview_clients_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_clients_list tool."""
    return Tool(
        name="linode_longview_clients_list",
        description="Lists Longview clients on the account.",
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


def create_linode_longview_plan_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_plan_update tool."""
    return Tool(
        name="linode_longview_plan_update",
        description=(
            "Updates the account Longview plan."
            " Requires confirm=true and supports dry_run=true to preview."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "longview_subscription": {
                    "type": "string",
                    "pattern": r"^longview-[1-9][0-9]*$",
                    "description": "Longview subscription plan identifier",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to update the Longview plan",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["longview_subscription", "confirm"],
        },
    ), Capability.Write


def create_linode_longview_client_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_delete tool."""
    return Tool(
        name="linode_longview_client_delete",
        description=(
            "Deletes a Longview client."
            " Requires confirm=true and supports dry_run=true to preview."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "client_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Longview client ID to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to delete the Longview client",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["client_id", "confirm"],
        },
    ), Capability.Destroy


def _required_positive_int_argument(arguments: dict[str, Any], name: str) -> int:
    value = arguments.get(name)
    if value is None:
        msg = f"{name} is required"
        raise ValueError(msg)
    if type(value) is not int or value < 1:
        msg = f"{name} must be a positive integer"
        raise ValueError(msg)
    return value


def _longview_client_id_error(value: object) -> str | None:
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return "client_id must be a positive integer"
    return None


def _longview_client_update_body(arguments: dict[str, Any]) -> dict[str, str]:
    label = arguments.get("label")
    if not isinstance(label, str):
        return {}
    return {"label": label}


def _longview_client_update_error(
    arguments: dict[str, Any],
) -> list[TextContent] | None:
    id_error = _longview_client_id_error(arguments.get("client_id"))
    if id_error is not None:
        return error_response(id_error)
    label = arguments.get("label")
    if not isinstance(label, str) or not label:
        return error_response("label must be a non-empty string")
    return None


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if type(value) is not int:
        msg = f"{name} must be an integer"
        raise TypeError(msg)
    if value < minimum:
        msg = f"{name} must be at least {minimum}"
        raise ValueError(msg)
    if maximum is not None and value > maximum:
        msg = f"{name} must be at most {maximum}"
        raise ValueError(msg)
    return value


async def handle_linode_longview_client_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_update tool request."""
    validation = _longview_client_update_error(arguments)
    if validation is not None:
        return validation

    client_id = cast("int", arguments["client_id"])
    body = _longview_client_update_body(arguments)

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Longview client. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_longview_client_update",
            arguments.get("environment", ""),
            "PUT",
            f"/longview/clients/{client_id}",
            None,
            request_body=body,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_longview_client(client_id, label=body["label"])
        return {
            "message": f"Longview client {client_id} updated successfully",
            "longview_client": result,
        }

    return await execute_tool(cfg, arguments, "update Longview client", _call)


async def handle_linode_longview_clients_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_clients_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_longview_clients(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Longview clients", _call)


LABEL_MIN_LENGTH = 3
LABEL_MAX_LENGTH = 32
LABEL_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")


def create_linode_longview_client_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_create tool."""
    return Tool(
        name="linode_longview_client_create",
        description=(
            "Creates a Longview client. Pass dry_run=true to preview without "
            "creating the client."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {
                    "type": "string",
                    "minLength": 3,
                    "maxLength": 32,
                    "pattern": "^[A-Za-z0-9_-]{3,32}$",
                    "description": "Unique Longview client label.",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm Longview client creation "
                        "or preview it with dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


def _validate_longview_client_label(value: object) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError("label is required")
    label = value.strip()
    if len(label) < LABEL_MIN_LENGTH or len(label) > LABEL_MAX_LENGTH:
        raise ValueError("label must be 3 to 32 characters")
    if LABEL_PATTERN.fullmatch(label) is None:
        raise ValueError(
            "label may only contain letters, numbers, hyphens, and underscores"
        )
    return label


async def handle_linode_longview_client_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_create tool request."""
    try:
        label = _validate_longview_client_label(arguments.get("label"))
    except ValueError as exc:
        return error_response(str(exc))

    confirm = arguments.get("confirm")
    if not isinstance(confirm, bool) or confirm is not True:
        return error_response(
            "This creates a Longview client. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        environment = arguments.get("environment")
        if not isinstance(environment, str):
            environment = ""
        return build_dry_run_response(
            "linode_longview_client_create",
            environment,
            "POST",
            "/longview/clients",
            None,
            side_effects=[f"A Longview client labeled {label!r} will be created."],
            request_body={"label": label},
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        client_data = await client.create_longview_client(label)
        return {
            "message": f"Longview client {label!r} created successfully",
            "longview_client": client_data,
        }

    return await execute_tool(cfg, arguments, "create Longview client", _call)


async def handle_linode_longview_plan_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_plan_update tool request."""
    longview_subscription_value = arguments.get("longview_subscription")
    if not isinstance(longview_subscription_value, str):
        return error_response("longview_subscription is required and must be a string")
    longview_subscription = longview_subscription_value
    if not re.fullmatch(r"longview-[1-9][0-9]*", longview_subscription):
        return error_response(
            "longview_subscription must be a Longview plan ID like longview-10"
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates the account Longview plan. Set confirm=true to proceed."
        )

    request_body: dict[str, Any] = {"longview_subscription": longview_subscription}

    if is_dry_run(arguments):
        environment = arguments.get("environment")
        if not isinstance(environment, str):
            environment = ""
        return build_dry_run_response(
            "linode_longview_plan_update",
            environment,
            "PUT",
            "/longview/plan",
            None,
            request_body=request_body,
            side_effects=["The account Longview plan would be updated."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_longview_plan(longview_subscription)

    return await execute_tool(cfg, arguments, "update Longview plan", _call)


async def handle_linode_longview_client_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_delete tool request."""
    try:
        client_id = _required_positive_int_argument(arguments, "client_id")
    except ValueError as exc:
        return error_response(str(exc))

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Longview client. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        environment = arguments.get("environment")
        if not isinstance(environment, str):
            environment = ""
        return build_dry_run_response(
            "linode_longview_client_delete",
            environment,
            "DELETE",
            f"/longview/clients/{client_id}",
            None,
            side_effects=[f"Longview client {client_id} would be deleted."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_longview_client(client_id)
        return {
            "message": f"Longview client {client_id} deleted",
            "client_id": client_id,
        }

    return await execute_tool(cfg, arguments, "delete Longview client", _call)


async def handle_linode_longview_subscription_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_subscription_get tool request."""
    try:
        subscription_id = _required_positive_int_argument(arguments, "subscription_id")
    except ValueError as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        subscription = await client.get_longview_subscription(subscription_id)
        return {"subscription": subscription}

    return await execute_tool(
        cfg,
        arguments,
        f"get Longview subscription {subscription_id}",
        _call,
    )


async def handle_linode_longview_plan_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_plan_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_longview_plan()

    return await execute_tool(cfg, arguments, "get Longview plan", _call)

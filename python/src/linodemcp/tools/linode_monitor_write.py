from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


_SERVICE_TYPE_RE = re.compile(r"^[A-Za-z0-9_-]+$")


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_monitor_service_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_list tool."""
    return Tool(
        name="linode_monitor_service_list",
        description="Lists supported Linode Metrics service types.",
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


def monitor_service_to_response_dict(service: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw Monitor service API dict to proto-canonical form."""
    return {
        "label": service.get("label", ""),
        "service_type": service.get("service_type", ""),
    }


def create_linode_monitor_service_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_get tool."""
    return Tool(
        name="linode_monitor_service_get",
        description="Gets details for a supported Linode Metrics service type.",
        inputSchema=schema("linode.mcp.v1.MonitorServiceGetInput"),
    ), Capability.Read


def create_linode_monitor_service_metric_query_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_metric_query tool."""
    return Tool(
        name="linode_monitor_service_metric_query",
        description=("Reads metrics for a Linode Metrics service entity type."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
            },
            "required": ["service_type"],
        },
    ), Capability.Read


def create_linode_monitor_dashboard_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_dashboard_list tool."""
    return Tool(
        name="linode_monitor_dashboard_list",
        description="Lists Linode Metrics dashboards.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
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
        },
    ), Capability.Read


def create_linode_monitor_alert_definition_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_alert_definition_list tool."""
    return Tool(
        name="linode_monitor_alert_definition_list",
        description="Lists Linode Metrics alert definitions.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
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
        },
    ), Capability.Read


def create_linode_monitor_alert_channel_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_alert_channel_list tool."""
    return Tool(
        name="linode_monitor_alert_channel_list",
        description="Lists Linode Metrics alert channels.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
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
        },
    ), Capability.Read


def create_linode_monitor_service_dashboard_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_dashboard_list tool."""
    return Tool(
        name="linode_monitor_service_dashboard_list",
        description=("Lists dashboards for a Linode Metrics service type."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
            },
            "required": ["service_type"],
        },
    ), Capability.Read


def create_linode_monitor_dashboard_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_dashboard_get tool."""
    return Tool(
        name="linode_monitor_dashboard_get",
        description="Gets a Linode Metrics dashboard by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "dashboard_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Dashboard ID to get (required)",
                },
            },
            "required": ["dashboard_id"],
        },
    ), Capability.Read


def create_linode_monitor_service_metric_definition_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_metric_definition_list tool."""
    return Tool(
        name="linode_monitor_service_metric_definition_list",
        description=("Lists metric definitions for a Linode Metrics service type."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
            },
            "required": ["service_type"],
        },
    ), Capability.Read


def create_linode_monitor_service_alert_definition_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_list tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_list",
        description=("Lists alert definitions for a Linode Metrics service type."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
            },
            "required": ["service_type"],
        },
    ), Capability.Read


def create_linode_monitor_service_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_token_create tool."""
    return Tool(
        name="linode_monitor_service_token_create",
        description=(
            "Creates a JWT for the Linode Metrics service scoped to the given "
            "entities. The token is returned only once and cannot be retrieved "
            "later; capture both `token` and `expiry` from the response."
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
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                },
                "entity_ids": {
                    "type": "array",
                    "items": {"type": "integer"},
                    "minItems": 1,
                    "description": (
                        "Non-empty list of entity IDs the token will grant access "
                        "to (required)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["service_type", "entity_ids", "confirm"],
        },
    ), Capability.Write


def create_linode_monitor_service_alert_definition_create_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_create tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_create",
        description="Creates an alert definition for a Linode Metrics service type.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
                "label": {"type": "string", "description": "Alert label (required)"},
                "severity": {
                    "type": "integer",
                    "description": "Alert severity value (required)",
                },
                "rule_criteria": {
                    "type": "object",
                    "description": "Alert rule criteria (required)",
                },
                "trigger_conditions": {
                    "type": "object",
                    "description": "Alert trigger conditions (required)",
                },
                "channel_ids": {
                    "type": "array",
                    "items": {"type": "integer"},
                    "minItems": 1,
                    "description": "Notification channel IDs (required)",
                },
                "description": {"type": "string", "description": "Alert description"},
                "entity_ids": {
                    "type": "array",
                    "items": {"type": "integer"},
                    "minItems": 1,
                    "description": "Optional service entity IDs",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "service_type",
                "label",
                "severity",
                "rule_criteria",
                "trigger_conditions",
                "channel_ids",
                "confirm",
            ],
        },
    ), Capability.Write


def create_linode_monitor_service_alert_definition_get_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_get tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_get",
        description="Gets an alert definition for a Linode Metrics service type.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
                "alert_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Alert definition ID to get (required)",
                },
            },
            "required": ["service_type", "alert_id"],
        },
    ), Capability.Read


def create_linode_monitor_service_alert_definition_delete_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_delete tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_delete",
        description=(
            "Deletes an alert definition for a Linode Metrics service type."
            " Pass dry_run=true to preview without deleting."
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
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                    "pattern": "^[A-Za-z0-9_-]+$",
                },
                "alert_id": {
                    "type": "integer",
                    "description": "Alert definition ID to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["service_type", "alert_id", "confirm"],
        },
    ), Capability.Destroy


def _validate_service_type(raw: object) -> str | None:
    """Return a safe service_type slug or None for invalid input."""
    if not isinstance(raw, str) or not raw:
        return None
    if not _SERVICE_TYPE_RE.fullmatch(raw):
        return None
    return raw


async def handle_linode_monitor_service_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_services()
        services = data.get("data", [])
        return {
            "message": "Monitor services listed",
            "count": len(services),
            "services": services,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list monitor services", _call)


async def handle_linode_monitor_service_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_get tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.get_monitor_service(service_type)
        return monitor_service_to_response_dict(data)

    return await execute_tool(cfg, arguments, "get monitor service", _call)


async def handle_linode_monitor_service_metric_query(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_metric_query tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.read_monitor_service_metrics(service_type)
        return {
            "message": f"Monitor service metrics read for '{service_type}'",
            "service_type": service_type,
            "metrics": data,
        }

    return await execute_tool(cfg, arguments, "read monitor service metrics", _call)


async def handle_linode_monitor_dashboard_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_dashboard_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_dashboards(page=page, page_size=page_size)
        dashboards = data.get("data", [])
        return {
            "message": "Monitor dashboards listed",
            "count": len(dashboards),
            "dashboards": dashboards,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list monitor dashboards", _call)


async def handle_linode_monitor_alert_definition_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_alert_definition_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_alert_definitions(
            page=page, page_size=page_size
        )
        alert_definitions = data.get("data", [])
        return {
            "message": "Monitor alert definitions listed",
            "count": len(alert_definitions),
            "alert_definitions": alert_definitions,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list monitor alert definitions", _call)


async def handle_linode_monitor_alert_channel_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_alert_channel_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_alert_channels(page=page, page_size=page_size)
        alert_channels = data.get("data", [])
        return {
            "message": "Monitor alert channels listed",
            "count": len(alert_channels),
            "alert_channels": alert_channels,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list monitor alert channels", _call)


async def handle_linode_monitor_service_dashboard_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_dashboard_list tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_service_dashboards(service_type)
        return {
            "message": f"Monitor service dashboards listed for '{service_type}'",
            "service_type": service_type,
            "dashboards": data,
        }

    return await execute_tool(cfg, arguments, "list monitor service dashboards", _call)


async def handle_linode_monitor_dashboard_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_dashboard_get tool request."""
    raw_dashboard_id = arguments.get("dashboard_id")
    if type(raw_dashboard_id) is not int:
        return error_response("dashboard_id must be a valid integer")
    if raw_dashboard_id <= 0:
        return error_response("dashboard_id must be a positive integer")
    dashboard_id = raw_dashboard_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.get_monitor_dashboard(dashboard_id)
        return {
            "message": f"Monitor dashboard {dashboard_id} retrieved",
            "dashboard_id": dashboard_id,
            "dashboard": data,
        }

    return await execute_tool(cfg, arguments, "get monitor dashboard", _call)


async def handle_linode_monitor_service_metric_definition_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_metric_definition_list tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_service_metric_definitions(service_type)
        return {
            "message": (
                f"Monitor service metric definitions listed for '{service_type}'"
            ),
            "service_type": service_type,
            "metric_definitions": data,
        }

    return await execute_tool(
        cfg, arguments, "list monitor service metric definitions", _call
    )


async def handle_linode_monitor_service_alert_definition_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_alert_definition_list tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_monitor_service_alert_definitions(service_type)
        return {
            "message": (
                f"Monitor service alert definitions listed for '{service_type}'"
            ),
            "service_type": service_type,
            "alert_definitions": data,
        }

    return await execute_tool(
        cfg, arguments, "list monitor service alert definitions", _call
    )


def _coerce_entity_ids(raw: object) -> list[int] | None:
    """Return raw as a list of ints, or None if any element is not an int.

    Typed `object` rather than `Any`; the inner cast satisfies pyright strict
    mode by giving each iterated item a known type to narrow from.
    """
    if not isinstance(raw, list) or not raw:
        return None
    # isinstance narrows raw to list[Unknown]; cast to list[object] gives
    # each iterated item a known type for pyright to narrow further.
    items = cast("list[object]", raw)
    result: list[int] = []
    for item in items:
        # bool is a subclass of int; reject it explicitly to avoid `True` -> 1.
        if isinstance(item, bool) or not isinstance(item, int):
            return None
        result.append(item)
    return result


def _build_alert_definition_create_args(
    arguments: dict[str, Any],
    *,
    require_confirm: bool = True,
) -> tuple[dict[str, Any] | None, str | None]:
    """Validate tool args for alert-definition create.

    When require_confirm is False (the dry-run path) the confirm gate is
    skipped so the preview still validates the create body without forcing
    confirm=true.
    """
    args: dict[str, Any] | None = None
    error: str | None = None

    service_type = _validate_service_type(arguments.get("service_type"))
    label = arguments.get("label")
    raw_severity = arguments.get("severity")
    rule_criteria = arguments.get("rule_criteria")
    trigger_conditions = arguments.get("trigger_conditions")
    channel_ids = _coerce_entity_ids(arguments.get("channel_ids"))
    description = arguments.get("description")
    entity_ids = None
    if "entity_ids" in arguments:
        entity_ids = _coerce_entity_ids(arguments.get("entity_ids"))

    if require_confirm and arguments.get("confirm") is not True:
        error = (
            "This creates a Linode Metrics alert definition. "
            "Set confirm=true to proceed."
        )
    elif service_type is None:
        error = (
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )
    elif not isinstance(label, str) or not label:
        error = "label is required"
    elif type(raw_severity) is not int:
        error = "severity must be a valid integer"
    elif raw_severity not in {0, 1, 2, 3}:
        error = "severity must be one of 0, 1, 2, or 3"
    elif not isinstance(rule_criteria, dict) or not rule_criteria:
        error = "rule_criteria must be a non-empty object"
    elif not isinstance(trigger_conditions, dict) or not trigger_conditions:
        error = "trigger_conditions must be a non-empty object"
    elif channel_ids is None:
        error = "channel_ids must be a non-empty list of integers"
    elif "entity_ids" in arguments and entity_ids is None:
        error = "entity_ids must be a non-empty list of integers"
    elif description is not None and not isinstance(description, str):
        error = "description must be a string"
    else:
        args = {
            "service_type": service_type,
            "label": label,
            "severity": raw_severity,
            "rule_criteria": rule_criteria,
            "trigger_conditions": trigger_conditions,
            "channel_ids": channel_ids,
            "description": description,
            "entity_ids": entity_ids,
        }

    return args, error


async def handle_linode_monitor_service_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_token_create tool request."""
    service_type = arguments.get("service_type", "")
    if not service_type or not isinstance(service_type, str):
        return error_response("service_type is required")

    entity_ids = _coerce_entity_ids(arguments.get("entity_ids"))
    if entity_ids is None:
        return error_response("entity_ids must be a non-empty list of integers")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_monitor_service_token_create",
            arguments.get("environment", ""),
            "POST",
            f"/monitor/services/{service_type}/token",
            None,
        )

    if not arguments.get("confirm"):
        return error_response(
            "This creates a Linode Metrics service token. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.create_monitor_service_token(service_type, entity_ids)
        return {
            "message": f"Monitor service token created for '{service_type}'",
            "service_type": service_type,
            "token": data.get("token"),
            "expiry": data.get("expiry"),
        }

    return await execute_tool(cfg, arguments, "create monitor service token", _call)


async def handle_linode_monitor_service_alert_definition_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_alert_definition_create tool request."""
    if is_dry_run(arguments):
        preview, preview_error = _build_alert_definition_create_args(
            arguments, require_confirm=False
        )
        if preview_error is not None or preview is None:
            return error_response(
                preview_error or "invalid alert definition create arguments"
            )
        return build_dry_run_response(
            "linode_monitor_service_alert_definition_create",
            arguments.get("environment", ""),
            "POST",
            f"/monitor/services/{preview['service_type']}/alert-definitions",
            None,
        )

    parsed, error = _build_alert_definition_create_args(arguments)
    if error is not None or parsed is None:
        return error_response(error or "invalid alert definition create arguments")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.create_monitor_service_alert_definition(
            parsed["service_type"],
            label=parsed["label"],
            severity=parsed["severity"],
            rule_criteria=parsed["rule_criteria"],
            trigger_conditions=parsed["trigger_conditions"],
            channel_ids=parsed["channel_ids"],
            description=parsed["description"],
            entity_ids=parsed["entity_ids"],
        )
        return {
            "message": (
                "Monitor service alert definition created for "
                f"'{parsed['service_type']}'"
            ),
            "service_type": parsed["service_type"],
            "alert_definition": data,
        }

    return await execute_tool(
        cfg, arguments, "create monitor service alert definition", _call
    )


async def handle_linode_monitor_service_alert_definition_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_alert_definition_get tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    raw_alert_id = arguments.get("alert_id")
    if type(raw_alert_id) is not int:
        return error_response("alert_id must be a valid integer")
    if raw_alert_id <= 0:
        return error_response("alert_id must be a positive integer")
    alert_id = raw_alert_id

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.get_monitor_service_alert_definition(service_type, alert_id)
        return {
            "message": (
                f"Monitor service alert definition {alert_id} "
                f"retrieved for '{service_type}'"
            ),
            "service_type": service_type,
            "alert_id": alert_id,
            "alert_definition": data,
        }

    return await execute_tool(
        cfg, arguments, "get monitor service alert definition", _call
    )


async def handle_linode_monitor_service_alert_definition_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_alert_definition_delete tool request."""
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return error_response(
            "service_type is required and must contain only letters, "
            "numbers, '_' or '-'"
        )

    raw_alert_id = arguments.get("alert_id")
    if type(raw_alert_id) is not int:
        return error_response("alert_id must be a valid integer")
    if raw_alert_id <= 0:
        return error_response("alert_id must be a positive integer")
    alert_id = raw_alert_id

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_monitor_service_alert_definition(
                service_type, alert_id
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_monitor_service_alert_definition_delete",
            "DELETE",
            f"/monitor/services/{service_type}/alert-definitions/{alert_id}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Linode Metrics alert definition. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_monitor_service_alert_definition(service_type, alert_id)
        return {
            "message": (
                f"Monitor service alert definition {alert_id} "
                f"deleted for '{service_type}'"
            ),
            "service_type": service_type,
            "alert_id": alert_id,
        }

    return await execute_tool(
        cfg, arguments, "delete monitor service alert definition", _call
    )


def create_linode_monitor_service_alert_definition_update_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_update tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_update",
        description=(
            "Updates a Linode Metrics alert definition for a service type."
            " Pass dry_run=true to preview without modifying."
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
                "service_type": {
                    "type": "string",
                    "description": "Monitor service type.",
                },
                "alert_id": {
                    "type": "integer",
                    "description": "Alert definition ID.",
                },
                "channel_ids": {"type": "array", "items": {"type": "integer"}},
                "description": {"type": "string"},
                "entity_ids": {"type": "array", "items": {"type": "integer"}},
                "label": {"type": "string"},
                "rule_criteria": {"type": "object"},
                "severity": {"type": "integer", "enum": [0, 1, 2, 3]},
                "status": {"type": "string", "enum": ["enabled", "disabled"]},
                "trigger_conditions": {"type": "object"},
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["service_type", "alert_id", "confirm"],
        },
    ), Capability.Write


def _validate_alert_target(
    arguments: dict[str, Any],
) -> tuple[str | None, int | None, str | None]:
    """Validate service_type + alert_id, the path components shared by the
    alert update dry-run and real paths. Returns (service_type, alert_id,
    error); extracted to keep the update handler under PLR0911 once the
    dry-run branch lands.
    """
    service_type = _validate_service_type(arguments.get("service_type"))
    if service_type is None:
        return (
            None,
            None,
            (
                "service_type is required and must contain only letters, "
                "numbers, '_' or '-'"
            ),
        )
    raw_alert_id = arguments.get("alert_id")
    if type(raw_alert_id) is not int:
        return None, None, "alert_id must be a valid integer"
    if raw_alert_id <= 0:
        return None, None, "alert_id must be a positive integer"
    return service_type, raw_alert_id, None


async def handle_linode_monitor_service_alert_definition_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_alert_definition_update tool request."""
    service_type, alert_id, target_error = _validate_alert_target(arguments)
    if target_error is not None or service_type is None or alert_id is None:
        return error_response(target_error or "invalid alert target")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_monitor_service_alert_definition(
                service_type, alert_id
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_monitor_service_alert_definition_update",
            "PUT",
            f"/monitor/services/{service_type}/alert-definitions/{alert_id}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Linode Metrics alert definition. "
            "Set confirm=true to proceed."
        )
    payload_keys = (
        "channel_ids",
        "description",
        "entity_ids",
        "label",
        "rule_criteria",
        "severity",
        "status",
        "trigger_conditions",
    )
    fields = {
        key: arguments[key]
        for key in payload_keys
        if key in arguments and arguments[key] is not None
    }
    if not fields:
        return error_response("at least one update field is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.update_monitor_alert_definition(
            service_type, alert_id, **fields
        )
        return {
            "message": f"Monitor alert definition {alert_id} updated",
            "service_type": service_type,
            "alert_id": alert_id,
            "alert_definition": data,
        }

    return await execute_tool(cfg, arguments, "update monitor alert definition", _call)

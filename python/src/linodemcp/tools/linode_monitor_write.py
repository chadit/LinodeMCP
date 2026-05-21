from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


_SERVICE_TYPE_RE = re.compile(r"^[A-Za-z0-9_-]+$")


def create_linode_monitor_service_metrics_read_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_metrics_read tool."""
    return Tool(
        name="linode_monitor_service_metrics_read",
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


def create_linode_monitor_service_dashboards_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_dashboards_list tool."""
    return Tool(
        name="linode_monitor_service_dashboards_list",
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


def create_linode_monitor_service_metric_definitions_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_metric_definitions_list tool."""
    return Tool(
        name="linode_monitor_service_metric_definitions_list",
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
            },
            "required": ["service_type", "entity_ids", "confirm"],
        },
    ), Capability.Write


def create_linode_monitor_service_alert_definition_delete_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_delete tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_delete",
        description="Deletes an alert definition for a Linode Metrics service type.",
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


async def handle_linode_monitor_service_metrics_read(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_metrics_read tool request."""
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


async def handle_linode_monitor_service_dashboards_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_dashboards_list tool request."""
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


async def handle_linode_monitor_service_metric_definitions_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_metric_definitions_list tool request."""
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


async def handle_linode_monitor_service_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_token_create tool request."""
    if not arguments.get("confirm"):
        return error_response(
            "This creates a Linode Metrics service token. Set confirm=true to proceed."
        )

    service_type = arguments.get("service_type", "")
    if not service_type or not isinstance(service_type, str):
        return error_response("service_type is required")

    entity_ids = _coerce_entity_ids(arguments.get("entity_ids"))
    if entity_ids is None:
        return error_response("entity_ids must be a non-empty list of integers")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.create_monitor_service_token(service_type, entity_ids)
        return {
            "message": f"Monitor service token created for '{service_type}'",
            "service_type": service_type,
            "token": data.get("token"),
            "expiry": data.get("expiry"),
        }

    return await execute_tool(cfg, arguments, "create monitor service token", _call)


async def handle_linode_monitor_service_alert_definition_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_alert_definition_delete tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Linode Metrics alert definition. "
            "Set confirm=true to proceed."
        )

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

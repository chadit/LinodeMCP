from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import monitor_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


_SERVICE_TYPE_RE = re.compile(r"^[A-Za-z0-9_-]+$")


def create_linode_monitor_service_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_list tool."""
    return Tool(
        name="linode_monitor_service_list",
        description="Lists supported Linode Metrics service types.",
        inputSchema=schema("linode.mcp.v1.MonitorServiceListInput"),
    ), Capability.Read


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
        inputSchema=schema("linode.mcp.v1.MonitorServiceMetricQueryInput"),
    ), Capability.Read


def create_linode_monitor_dashboard_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_dashboard_list tool."""
    return Tool(
        name="linode_monitor_dashboard_list",
        description="Lists Linode Metrics dashboards.",
        inputSchema=schema("linode.mcp.v1.MonitorDashboardListInput"),
    ), Capability.Read


def create_linode_monitor_alert_definition_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_alert_definition_list tool."""
    return Tool(
        name="linode_monitor_alert_definition_list",
        description="Lists Linode Metrics alert definitions.",
        inputSchema=schema("linode.mcp.v1.MonitorAlertDefinitionListInput"),
    ), Capability.Read


def create_linode_monitor_alert_channel_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_alert_channel_list tool."""
    return Tool(
        name="linode_monitor_alert_channel_list",
        description="Lists Linode Metrics alert channels.",
        inputSchema=schema("linode.mcp.v1.MonitorAlertChannelListInput"),
    ), Capability.Read


def create_linode_monitor_service_dashboard_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_service_dashboard_list tool."""
    return Tool(
        name="linode_monitor_service_dashboard_list",
        description=("Lists dashboards for a Linode Metrics service type."),
        inputSchema=schema("linode.mcp.v1.MonitorServiceDashboardListInput"),
    ), Capability.Read


def create_linode_monitor_dashboard_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_monitor_dashboard_get tool."""
    return Tool(
        name="linode_monitor_dashboard_get",
        description="Gets a Linode Metrics dashboard by ID.",
        inputSchema=schema("linode.mcp.v1.MonitorDashboardGetInput"),
    ), Capability.Read


def create_linode_monitor_service_metric_definition_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_metric_definition_list tool."""
    return Tool(
        name="linode_monitor_service_metric_definition_list",
        description=("Lists metric definitions for a Linode Metrics service type."),
        inputSchema=schema("linode.mcp.v1.MonitorServiceMetricDefinitionListInput"),
    ), Capability.Read


def create_linode_monitor_service_alert_definition_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_list tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_list",
        description=("Lists alert definitions for a Linode Metrics service type."),
        inputSchema=schema("linode.mcp.v1.MonitorServiceAlertDefinitionListInput"),
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
        inputSchema=schema("linode.mcp.v1.MonitorServiceTokenCreateInput"),
    ), Capability.Write


def create_linode_monitor_service_alert_definition_create_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_create tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_create",
        description="Creates an alert definition for a Linode Metrics service type.",
        inputSchema=schema("linode.mcp.v1.MonitorServiceAlertDefinitionCreateInput"),
    ), Capability.Write


def create_linode_monitor_service_alert_definition_get_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_monitor_service_alert_definition_get tool."""
    return Tool(
        name="linode_monitor_service_alert_definition_get",
        description="Gets an alert definition for a Linode Metrics service type.",
        inputSchema=schema("linode.mcp.v1.MonitorAlertDefinitionGetInput"),
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
        inputSchema=schema("linode.mcp.v1.MonitorAlertDefinitionDeleteInput"),
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
        raw = await client.list_monitor_services()
        return serialize_list_response(
            raw,
            "services",
            monitor_pb2.MonitorServiceListResponse(),
        )

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
        return serialize_api_response(data, monitor_pb2.MonitorService())

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
        return serialize_api_response(
            {
                "message": f"Monitor service metrics read for '{service_type}'",
                "service_type": service_type,
                "metrics": data,
            },
            monitor_pb2.MonitorServiceMetricQueryResponse(),
        )

    return await execute_tool(cfg, arguments, "read monitor service metrics", _call)


async def handle_linode_monitor_dashboard_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_dashboard_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_monitor_dashboards(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "dashboards",
            monitor_pb2.MonitorDashboardListResponse(),
        )

    return await execute_tool(cfg, arguments, "list monitor dashboards", _call)


async def handle_linode_monitor_alert_definition_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_alert_definition_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_monitor_alert_definitions(
            page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "alert_definitions",
            monitor_pb2.MonitorAlertDefinitionListResponse(),
        )

    return await execute_tool(cfg, arguments, "list monitor alert definitions", _call)


async def handle_linode_monitor_alert_channel_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_alert_channel_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_monitor_alert_channels(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "alert_channels",
            monitor_pb2.MonitorAlertChannelListResponse(),
        )

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
        return serialize_list_response(
            data,
            "dashboards",
            monitor_pb2.MonitorServiceDashboardListResponse(),
        )

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
        return serialize_api_response(
            await client.get_monitor_dashboard(dashboard_id),
            monitor_pb2.MonitorDashboard(),
        )

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
        return serialize_list_response(
            data,
            "metric_definitions",
            monitor_pb2.MonitorServiceMetricDefinitionListResponse(),
        )

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
        return serialize_list_response(
            data,
            "alert_definitions",
            monitor_pb2.MonitorServiceAlertDefinitionListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list monitor service alert definitions", _call
    )


def _coerce_positive_ids(raw: object) -> list[int] | None:
    """Return raw as a list of positive ints, or None on any invalid element.

    Mirrors Go's intArrayArgument / monitorServiceTokenEntityIDsFromTool:
    the list must be non-empty and every element a positive integer. Typed
    `object` rather than `Any`; the inner cast satisfies pyright strict
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
        if isinstance(item, bool) or not isinstance(item, int) or item <= 0:
            return None
        result.append(item)
    return result


def _coerce_entity_id_strings(raw: object) -> list[str] | None:
    """Return raw as a list of non-blank strings, or None on any invalid element.

    Alert-definition entity_ids are strings in the Linode Metrics API (and in
    the shared proto contract); this mirrors Go's optionalStringArrayArgument,
    which accepts an empty array but rejects non-string or blank elements.
    """
    if not isinstance(raw, list):
        return None
    items = cast("list[object]", raw)
    result: list[str] = []
    for item in items:
        if not isinstance(item, str) or not item.strip():
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
    channel_ids = _coerce_positive_ids(arguments.get("channel_ids"))
    description = arguments.get("description")
    entity_ids: list[str] | None = None
    entity_ids_invalid = False
    if "entity_ids" in arguments:
        coerced = _coerce_entity_id_strings(arguments.get("entity_ids"))
        entity_ids_invalid = coerced is None
        # An empty entity_ids array is accepted but omitted from the request
        # body, matching Go's omitempty encoding of the same field.
        entity_ids = coerced or None

    if require_confirm and arguments.get("confirm") is not True:
        error = "This creates a monitor alert definition. Set confirm=true to proceed."
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
        error = "channel_ids must be a non-empty array of positive integers"
    elif entity_ids_invalid:
        error = "entity_ids must be an array of non-empty strings"
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

    entity_ids = _coerce_positive_ids(arguments.get("entity_ids"))
    if entity_ids is None:
        return error_response(
            "entity_ids must be a non-empty array of positive integers"
        )

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
            "This creates a monitor service token. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.create_monitor_service_token(service_type, entity_ids)
        return serialize_api_response(
            data, monitor_pb2.MonitorServiceTokenCreateResponse()
        )

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
        return serialize_api_response(
            {
                "message": (
                    "Monitor service alert definition created for "
                    f"'{parsed['service_type']}'"
                ),
                "alert_definition": data,
            },
            monitor_pb2.MonitorAlertDefinitionWriteResponse(),
        )

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
        return serialize_api_response(
            await client.get_monitor_service_alert_definition(service_type, alert_id),
            monitor_pb2.MonitorAlertDefinition(),
        )

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
            "This deletes a monitor alert definition. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_monitor_service_alert_definition(service_type, alert_id)
        return serialize_api_response(
            {
                "message": (
                    f"Monitor service alert definition {alert_id} "
                    f"deleted for '{service_type}'"
                ),
                "service_type": service_type,
                "alert_id": alert_id,
            },
            monitor_pb2.MonitorAlertDefinitionDeleteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.MonitorServiceAlertDefinitionUpdateInput"),
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


def _validate_alert_update_fields(fields: dict[str, Any]) -> str | None:
    """Validate the present alert-definition update fields, trimming label.

    Mirrors Go's monitorServiceAlertDefinitionUpdateRequestFromTool so both
    languages reject the same malformed update payloads locally instead of
    passing them through to the API. Only fields present in the payload are
    checked; entity_ids must be a non-empty array of non-blank strings on
    update (unlike create, where an empty array is accepted and omitted).
    """
    error: str | None = None
    label = fields.get("label")
    if "label" in fields and (not isinstance(label, str) or not label.strip()):
        error = "label must be a non-empty string"
    elif "severity" in fields and (
        type(fields["severity"]) is not int or fields["severity"] not in {0, 1, 2, 3}
    ):
        error = "severity must be an integer from 0 through 3"
    elif "status" in fields and fields["status"] not in {"enabled", "disabled"}:
        error = "status must be enabled or disabled"
    elif "rule_criteria" in fields and (
        not isinstance(fields["rule_criteria"], dict) or not fields["rule_criteria"]
    ):
        error = "rule_criteria must be a non-empty object"
    elif "trigger_conditions" in fields and (
        not isinstance(fields["trigger_conditions"], dict)
        or not fields["trigger_conditions"]
    ):
        error = "trigger_conditions must be a non-empty object"
    elif (
        "channel_ids" in fields and _coerce_positive_ids(fields["channel_ids"]) is None
    ):
        error = "channel_ids must be a non-empty array of positive integers"
    elif "entity_ids" in fields and not _coerce_entity_id_strings(fields["entity_ids"]):
        error = "entity_ids must be an array of non-empty strings"
    elif "description" in fields and not isinstance(fields["description"], str):
        error = "description must be a string"
    elif isinstance(label, str):
        fields["label"] = label.strip()
    return error


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
            "This updates a monitor alert definition. Set confirm=true to proceed."
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

    validation_error = _validate_alert_update_fields(fields)
    if validation_error is not None:
        return error_response(validation_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.update_monitor_alert_definition(
            service_type, alert_id, **fields
        )
        return serialize_api_response(
            {
                "message": f"Monitor alert definition {alert_id} updated",
                "alert_definition": data,
            },
            monitor_pb2.MonitorAlertDefinitionWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update monitor alert definition", _call)

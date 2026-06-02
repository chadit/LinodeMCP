"""Linode Managed Databases tools."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast
from urllib.parse import quote

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
    from collections.abc import Callable

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_DATABASE_ENGINE_ID_PATTERN = re.compile(r"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$")
_CREATE_MYSQL_REQUIRED_FIELDS = ("label", "type", "engine", "region")
_CREATE_MYSQL_OPTIONAL_FIELDS = (
    "allow_list",
    "cluster_size",
    "engine_config",
    "fork",
    "private_network",
    "ssl_connection",
)
_CREATE_MYSQL_ALLOWED_FIELDS = (
    _CREATE_MYSQL_REQUIRED_FIELDS + _CREATE_MYSQL_OPTIONAL_FIELDS
)


def _validate_non_empty_string(
    arguments: dict[str, Any], name: str
) -> tuple[str | None, str | None]:
    value = arguments.get(name)
    if value is None:
        return None, f"{name} is required"
    if not isinstance(value, str):
        return None, f"{name} must be a string"
    normalized = value.strip()
    if not normalized:
        return None, f"{name} is required"
    if normalized != value:
        return None, f"{name} must not include leading or trailing whitespace"
    return normalized, None


def _validate_allowed_mysql_database_fields(arguments: dict[str, Any]) -> str | None:
    for field in arguments:
        if field in ("environment", "confirm", PARAM_DRY_RUN):
            continue
        if field not in _CREATE_MYSQL_ALLOWED_FIELDS:
            return f"unsupported argument: {field}"
    return None


def _copy_mysql_required_fields(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    for field in _CREATE_MYSQL_REQUIRED_FIELDS:
        value, error = _validate_non_empty_string(arguments, field)
        if error is not None or value is None:
            return error or f"{field} is required"
        payload[field] = value
    return None


def _copy_mysql_allow_list(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    if "allow_list" not in arguments:
        return None
    allow_list = arguments["allow_list"]
    if not isinstance(allow_list, list):
        return "allow_list must be an array of non-empty strings"
    allow_list_items = cast("list[object]", allow_list)
    if any(not isinstance(item, str) or not item.strip() for item in allow_list_items):
        return "allow_list must be an array of non-empty strings"
    payload["allow_list"] = allow_list
    return None


def _copy_mysql_cluster_size(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    if "cluster_size" not in arguments:
        return None
    cluster_size = arguments["cluster_size"]
    if not isinstance(cluster_size, int) or isinstance(cluster_size, bool):
        return "cluster_size must be an integer"
    if cluster_size < 1:
        return "cluster_size must be at least 1"
    payload["cluster_size"] = cluster_size
    return None


def _copy_mysql_object_field(
    arguments: dict[str, Any], payload: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if not isinstance(value, dict):
        return f"{name} must be an object"
    payload[name] = value
    return None


def _copy_mysql_private_network(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    if "private_network" not in arguments:
        return None
    private_network = arguments["private_network"]
    if not isinstance(private_network, str) or not private_network.strip():
        return "private_network must be a non-empty string"
    payload["private_network"] = private_network
    return None


def _copy_mysql_ssl_connection(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    if "ssl_connection" not in arguments:
        return None
    ssl_connection = arguments["ssl_connection"]
    if not isinstance(ssl_connection, bool):
        return "ssl_connection must be a boolean"
    payload["ssl_connection"] = ssl_connection
    return None


def _copy_mysql_engine_config(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    return _copy_mysql_object_field(arguments, payload, "engine_config")


def _copy_mysql_fork(arguments: dict[str, Any], payload: dict[str, Any]) -> str | None:
    return _copy_mysql_object_field(arguments, payload, "fork")


def _copy_mysql_optional_fields(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    validators: tuple[Callable[[dict[str, Any], dict[str, Any]], str | None], ...] = (
        _copy_mysql_allow_list,
        _copy_mysql_cluster_size,
        _copy_mysql_engine_config,
        _copy_mysql_fork,
        _copy_mysql_private_network,
        _copy_mysql_ssl_connection,
    )
    for validator in validators:
        error = validator(arguments, payload)
        if error is not None:
            return error
    return None


def _build_mysql_database_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    payload: dict[str, Any] = {}
    error = _validate_allowed_mysql_database_fields(arguments)
    if error is not None:
        return None, error
    error = _copy_mysql_required_fields(arguments, payload)
    if error is not None:
        return None, error
    error = _copy_mysql_optional_fields(arguments, payload)
    if error is not None:
        return None, error
    return payload, None


def _validate_engine_id(value: object) -> tuple[str | None, str | None]:
    """Validate a Managed Databases engine ID path parameter."""
    engine_id: str | None = None
    error: str | None = None

    if value is None:
        error = "engine_id is required"
    elif not isinstance(value, str):
        error = "engine_id must be a string"
    else:
        engine_id = value.strip()
        if not engine_id:
            error = "engine_id is required"
        elif engine_id != value:
            error = "engine_id must not include leading or trailing whitespace"
        elif "?" in engine_id or "#" in engine_id or ".." in engine_id:
            error = "engine_id must not contain query, fragment, or traversal segments"
        elif not _DATABASE_ENGINE_ID_PATTERN.fullmatch(engine_id):
            error = (
                "engine_id must use the documented engine/version shape with "
                "letters, numbers, dots, underscores, and hyphens"
            )

    if error is not None:
        return None, error
    return engine_id, None


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    """Parse an optional integer argument with range checks."""
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        msg = f"{name} must be an integer"
        raise TypeError(msg)
    if value < minimum:
        msg = f"{name} must be at least {minimum}"
        raise ValueError(msg)
    if maximum is not None and value > maximum:
        msg = f"{name} must be at most {maximum}"
        raise ValueError(msg)
    return value


def _required_positive_int_argument(arguments: dict[str, Any], name: str) -> int:
    """Parse a required positive integer path parameter."""
    value = arguments.get(name)
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        msg = f"{name} must be a positive integer"
        raise ValueError(msg)
    return value


def create_linode_database_engine_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_engine_get tool."""
    return Tool(
        name="linode_database_engine_get",
        description="Gets details for a Managed Databases engine.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "engine_id": {
                    "type": "string",
                    "description": (
                        "Managed Databases engine ID, for example mysql/8.0.26"
                    ),
                },
                # The OpenAPI contract lists page/page_size on this
                # single-engine route even though the 200 response is one object.
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "Page of results to return when the API includes paginated data"
                    ),
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["engine_id"],
        },
    ), Capability.Read


def create_linode_database_cluster_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_cluster_create tool."""
    return Tool(
        name="linode_database_cluster_create",
        description=(
            "Creates or restores a MySQL Managed Database. WARNING: this can "
            "create a billable resource. Pass dry_run=true to preview without "
            "creating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {"type": "string", "description": "Database label"},
                "type": {"type": "string", "description": "Linode database plan type"},
                "engine": {
                    "type": "string",
                    "description": "MySQL engine ID, for example mysql/8.0",
                },
                "region": {"type": "string", "description": "Target region"},
                "allow_list": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "IPv4/IPv6 addresses or ranges allowed to connect",
                },
                "cluster_size": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Number of nodes in the database cluster",
                },
                "engine_config": {
                    "type": "object",
                    "description": "Engine-specific configuration",
                },
                "fork": {
                    "type": "object",
                    "description": "Restore/fork source configuration",
                },
                "private_network": {
                    "type": "string",
                    "description": "Private network identifier",
                },
                "ssl_connection": {
                    "type": "boolean",
                    "description": "Whether to enable SSL connection requirements",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm database creation or restore."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "type", "engine", "region", "confirm"],
        },
    ), Capability.Write


def create_linode_database_mysql_instance_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_delete tool."""
    return Tool(
        name="linode_database_mysql_instance_delete",
        description=(
            "Deletes a MySQL Managed Database. Pass dry_run=true to preview "
            "without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "MySQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database deletion.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_database_mysql_instances_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instances_list tool."""
    return Tool(
        name="linode_database_mysql_instances_list",
        description="Lists MySQL Managed Database instances.",
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


def create_linode_database_instances_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_instances_list tool."""
    return Tool(
        name="linode_database_instances_list",
        description="Lists Managed Database instances.",
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


def create_linode_databases_engines_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_databases_engines_list tool."""
    return Tool(
        name="linode_databases_engines_list",
        description="Lists available Linode Managed Databases engines.",
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


async def handle_linode_database_engine_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_engine_get tool request."""
    engine_id, error = _validate_engine_id(arguments.get("engine_id"))
    if error is not None or engine_id is None:
        return error_response(error or "engine_id is required")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_engine(
            engine_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Managed Databases engine {engine_id}", _call
    )


def create_linode_database_mysql_instance_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_get tool."""
    return Tool(
        name="linode_database_mysql_instance_get",
        description="Gets a MySQL Managed Database instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "MySQL Managed Database instance ID",
                },
            },
            "required": ["instance_id"],
        },
    ), Capability.Read


def create_linode_database_mysql_config_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_config_get tool."""
    return Tool(
        name="linode_database_mysql_config_get",
        description="Lists MySQL Managed Database advanced parameters.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


async def handle_linode_database_cluster_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_cluster_create tool request."""
    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    payload, error = _build_mysql_database_payload(arguments)
    if error is not None or payload is None:
        return error_response(error or "invalid database create arguments")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_cluster_create",
            arguments.get("environment", ""),
            "POST",
            "/databases/mysql/instances",
            None,
            side_effects=[
                f"A MySQL Managed Database {payload['label']!r} will be "
                "created or restored."
            ],
            warnings=["Creating a Managed Database can incur billing."],
            request_body=payload,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_mysql_database_instance(payload)

    return await execute_tool(cfg, arguments, "create MySQL Managed Database", _call)


async def handle_linode_database_mysql_instance_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_delete tool request."""
    try:
        instance_id = _required_positive_int_argument(arguments, "instance_id")
    except ValueError as exc:
        return error_response(str(exc))

    encoded_instance_id = quote(str(instance_id), safe="")
    delete_path = f"/databases/mysql/instances/{encoded_instance_id}"

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_delete",
            arguments.get("environment", ""),
            "DELETE",
            delete_path,
            None,
            side_effects=[
                f"MySQL Managed Database instance {instance_id} will be deleted."
            ],
            warnings=["Deleting a Managed Database is destructive."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.delete_mysql_database_instance(instance_id)

    return await execute_tool(cfg, arguments, "delete MySQL Managed Database", _call)


async def handle_linode_database_instances_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_instances_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_database_instances(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode database instances", _call)


async def handle_linode_database_mysql_instance_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_get tool request."""
    try:
        instance_id = _optional_int_argument(arguments, "instance_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if instance_id is None:
        return error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_mysql_instance(instance_id)

    return await execute_tool(
        cfg, arguments, f"retrieve MySQL Managed Database instance {instance_id}", _call
    )


async def handle_linode_database_mysql_config_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_config_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_mysql_config()

    return await execute_tool(
        cfg, arguments, "retrieve MySQL Managed Database advanced parameters", _call
    )


async def handle_linode_database_mysql_instances_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instances_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_mysql_database_instances(
            page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "list Linode MySQL database instances", _call
    )


async def handle_linode_databases_engines_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_databases_engines_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_database_engines(page=page, page_size=page_size)
        engines = data.get("data", [])
        return {
            "message": "Database engines listed",
            "count": len(engines),
            "engines": engines,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list database engines", _call)

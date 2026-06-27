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
    MODE_PROP,
    PARAM_DRY_RUN,
    PARAM_MODE,
    PARAM_PLAN_ID,
    PLAN_ID_PROP,
    TWO_STAGE_NOTE,
    build_dry_run_response,
    error_response,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_DATABASE_ENGINE_ID_PATTERN = re.compile(r"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$")
_CREATE_DATABASE_REQUIRED_FIELDS = ("label", "type", "engine", "region")
_CREATE_DATABASE_OPTIONAL_FIELDS = (
    "allow_list",
    "cluster_size",
    "engine_config",
    "fork",
    "private_network",
    "ssl_connection",
)
_CREATE_DATABASE_ALLOWED_FIELDS = (
    _CREATE_DATABASE_REQUIRED_FIELDS + _CREATE_DATABASE_OPTIONAL_FIELDS
)
_UPDATE_DATABASE_OPTIONAL_FIELDS = (
    "allow_list",
    "engine_config",
    "label",
    "private_network",
    "type",
    "updates",
    "version",
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


def _validate_allowed_database_create_fields(arguments: dict[str, Any]) -> str | None:
    for field in arguments:
        if field in ("environment", "confirm", PARAM_DRY_RUN):
            continue
        if field not in _CREATE_DATABASE_ALLOWED_FIELDS:
            return f"unsupported argument: {field}"
    return None


def _copy_database_required_fields(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    for field in _CREATE_DATABASE_REQUIRED_FIELDS:
        value, error = _validate_non_empty_string(arguments, field)
        if error is not None or value is None:
            return error or f"{field} is required"
        payload[field] = value
    return None


def _copy_database_allow_list(
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


def _copy_database_cluster_size(
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


def _copy_database_object_field(
    arguments: dict[str, Any], payload: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if not isinstance(value, dict):
        return f"{name} must be an object"
    payload[name] = value
    return None


def _copy_database_private_network(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    if "private_network" not in arguments:
        return None
    private_network = arguments["private_network"]
    if private_network is not None and not isinstance(private_network, dict):
        return "private_network must be an object or null"
    payload["private_network"] = private_network
    return None


def _copy_database_ssl_connection(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    if "ssl_connection" not in arguments:
        return None
    ssl_connection = arguments["ssl_connection"]
    if not isinstance(ssl_connection, bool):
        return "ssl_connection must be a boolean"
    payload["ssl_connection"] = ssl_connection
    return None


def _copy_database_engine_config(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    return _copy_database_object_field(arguments, payload, "engine_config")


def _copy_database_fork(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    return _copy_database_object_field(arguments, payload, "fork")


def _copy_database_create_optional_fields(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    validators: tuple[Callable[[dict[str, Any], dict[str, Any]], str | None], ...] = (
        _copy_database_allow_list,
        _copy_database_cluster_size,
        _copy_database_engine_config,
        _copy_database_fork,
        _copy_database_private_network,
        _copy_database_ssl_connection,
    )
    for validator in validators:
        error = validator(arguments, payload)
        if error is not None:
            return error
    return None


def _build_database_create_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    payload: dict[str, Any] = {}
    error = _validate_allowed_database_create_fields(arguments)
    if error is not None:
        return None, error
    error = _copy_database_required_fields(arguments, payload)
    if error is not None:
        return None, error
    error = _copy_database_create_optional_fields(arguments, payload)
    if error is not None:
        return None, error
    return payload, None


def _build_mysql_database_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    return _build_database_create_payload(arguments)


def _build_postgresql_database_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    payload, error = _build_database_create_payload(arguments)
    if error is not None or payload is None:
        return None, error
    engine = payload["engine"]
    if not engine.lower().startswith(
        "postgresql/"
    ) or not _DATABASE_ENGINE_ID_PATTERN.fullmatch(engine):
        return None, "engine must be a PostgreSQL engine ID"
    return payload, None


def _validate_instance_id(value: object) -> tuple[int | None, str | None]:
    """Validate a MySQL Managed Database instance ID path parameter."""
    if value is None:
        return None, "instance_id is required"
    if not isinstance(value, int) or isinstance(value, bool):
        return None, "instance_id must be an integer"
    if value < 1:
        return None, "instance_id must be at least 1"
    return value, None


def _copy_optional_update_string(
    arguments: dict[str, Any], payload: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if not isinstance(value, str):
        return f"{name} must be a string"
    normalized = value.strip()
    if not normalized:
        return f"{name} must be a non-empty string"
    if normalized != value:
        return f"{name} must not include leading or trailing whitespace"
    payload[name] = value
    return None


def _copy_mysql_update_allow_list(
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


def _copy_mysql_update_object(
    arguments: dict[str, Any], payload: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if not isinstance(value, dict):
        return f"{name} must be an object"
    payload[name] = value
    return None


def _copy_mysql_update_object_or_null(
    arguments: dict[str, Any], payload: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if value is not None and not isinstance(value, dict):
        return f"{name} must be an object or null"
    payload[name] = value
    return None


def _copy_mysql_update_objects(
    arguments: dict[str, Any], payload: dict[str, Any]
) -> str | None:
    for field in ("engine_config", "updates"):
        error = _copy_mysql_update_object(arguments, payload, field)
        if error is not None:
            return error
    return _copy_mysql_update_object_or_null(arguments, payload, "private_network")


def _build_database_update_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    payload: dict[str, Any] = {}
    for field in arguments:
        if field in ("environment", "confirm", PARAM_DRY_RUN, "instance_id"):
            continue
        if field not in _UPDATE_DATABASE_OPTIONAL_FIELDS:
            return None, f"unsupported argument: {field}"

    for field in ("label", "type", "version"):
        error = _copy_optional_update_string(arguments, payload, field)
        if error is not None:
            return None, error
    error = _copy_mysql_update_allow_list(arguments, payload)
    if error is not None:
        return None, error
    error = _copy_mysql_update_objects(arguments, payload)
    if error is not None:
        return None, error
    if not payload:
        return None, "at least one update field is required"
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


def _validate_database_type_id(value: Any) -> tuple[str | None, str | None]:
    """Validate a database type ID path parameter."""
    if not isinstance(value, str):
        return None, "type_id must be a string"
    type_id = value.strip()
    if not type_id:
        return None, "type_id is required"
    if type_id != value:
        return None, "type_id must not include leading or trailing whitespace"
    if ".." in type_id or not re.fullmatch(r"[A-Za-z0-9._-]+", type_id):
        return (
            None,
            "type_id must use letters, numbers, dots, underscores, and hyphens",
        )
    return type_id, None


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


def database_engine_to_response_dict(engine: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw Managed Database engine API dict to proto-canonical form."""
    return {
        "id": engine.get("id") or "",
        "engine": engine.get("engine") or "",
        "version": engine.get("version") or "",
    }


def create_linode_database_engine_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_engine_get tool."""
    return Tool(
        name="linode_database_engine_get",
        description="Gets details for a Managed Databases engine.",
        inputSchema=schema("linode.mcp.v1.DatabaseEngineGetInput"),
    ), Capability.Read


def database_type_engine_to_response_dict(engine: dict[str, Any]) -> dict[str, Any]:
    """Shape one database type engine entry to proto-canonical form."""
    price: dict[str, Any] = engine.get("price") or {}
    return {
        "quantity": engine.get("quantity", 0),
        "price": {
            "hourly": price.get("hourly", 0.0),
            "monthly": price.get("monthly", 0.0),
        },
    }


def database_type_to_response_dict(db_type: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw Managed Database type API dict to proto-canonical form."""
    engines: dict[str, Any] = db_type.get("engines") or {}
    return {
        "id": db_type.get("id", ""),
        "label": db_type.get("label", ""),
        "class": db_type.get("class", ""),
        "disk": db_type.get("disk", 0),
        "memory": db_type.get("memory", 0),
        "vcpus": db_type.get("vcpus", 0),
        "deprecated": db_type.get("deprecated", False),
        "engines": {
            "mysql": [
                database_type_engine_to_response_dict(entry)
                for entry in cast("list[dict[str, Any]]", engines.get("mysql") or [])
            ],
            "postgresql": [
                database_type_engine_to_response_dict(entry)
                for entry in cast(
                    "list[dict[str, Any]]", engines.get("postgresql") or []
                )
            ],
        },
    }


def create_linode_database_type_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_type_get tool."""
    return Tool(
        name="linode_database_type_get",
        description="Gets details for a Managed Databases type.",
        inputSchema=schema("linode.mcp.v1.DatabaseTypeGetInput"),
    ), Capability.Read


def create_linode_database_mysql_instance_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_create tool."""
    return Tool(
        name="linode_database_mysql_instance_create",
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
                    "type": "object",
                    "description": (
                        "Object placing the database in a VPC (vpc_id, "
                        "subnet_id, public_access). Omit for no private networking."
                    ),
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


def create_linode_database_postgresql_instance_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_create tool."""
    return Tool(
        name="linode_database_postgresql_instance_create",
        description=(
            "Creates or restores a PostgreSQL Managed Database. WARNING: this can "
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
                    "description": "PostgreSQL engine ID, for example postgresql/17",
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
                    "type": "object",
                    "description": (
                        "Object placing the database in a VPC (vpc_id, "
                        "subnet_id, public_access). Omit for no private networking."
                    ),
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
        )
        + TWO_STAGE_NOTE,
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
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_database_mysql_instance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_list tool."""
    return Tool(
        name="linode_database_mysql_instance_list",
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


def create_linode_database_postgresql_instance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_list tool."""
    return Tool(
        name="linode_database_postgresql_instance_list",
        description="Lists PostgreSQL Managed Database instances.",
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


def create_linode_database_postgresql_instance_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_delete tool."""
    return Tool(
        name="linode_database_postgresql_instance_delete",
        description=(
            "Deletes a PostgreSQL Managed Database. Pass dry_run=true to "
            "preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database deletion.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_database_mysql_instance_patch_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_patch tool."""
    return Tool(
        name="linode_database_mysql_instance_patch",
        description=(
            "Applies pending patches to a MySQL Managed Database. Pass "
            "dry_run=true to preview without patching."
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
                    "description": "Must be true to confirm database patching.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_postgresql_instance_patch_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_patch tool."""
    return Tool(
        name="linode_database_postgresql_instance_patch",
        description=(
            "Applies pending patches to a PostgreSQL Managed Database. Pass "
            "dry_run=true to preview without patching."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database patching.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_mysql_instance_suspend_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_suspend tool."""
    return Tool(
        name="linode_database_mysql_instance_suspend",
        description=(
            "Suspends a MySQL Managed Database. Pass dry_run=true to preview "
            "without suspending."
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
                    "description": "Must be true to confirm database suspension.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_mysql_instance_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_update tool."""
    return Tool(
        name="linode_database_mysql_instance_update",
        description=(
            "Updates a MySQL Managed Database. Requires confirm=true; pass "
            "dry_run=true to preview without updating."
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
                "allow_list": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "IPv4/IPv6 addresses or ranges allowed to connect",
                },
                "engine_config": {
                    "type": "object",
                    "description": "Engine-specific configuration",
                },
                "label": {"type": "string", "description": "Database label"},
                "private_network": {
                    "type": "object",
                    "description": (
                        "Object placing the database in a VPC (vpc_id, "
                        "subnet_id, public_access). Pass null to detach."
                    ),
                },
                "type": {"type": "string", "description": "Linode database plan type"},
                "updates": {
                    "type": "object",
                    "description": "Maintenance update settings",
                },
                "version": {"type": "string", "description": "MySQL version"},
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_postgresql_instance_suspend_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_database_postgresql_instance_suspend tool."""
    return Tool(
        name="linode_database_postgresql_instance_suspend",
        description=(
            "Suspends a PostgreSQL Managed Database. Pass dry_run=true to "
            "preview without suspending."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database suspension.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_postgresql_instance_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_update tool."""
    return Tool(
        name="linode_database_postgresql_instance_update",
        description=(
            "Updates a PostgreSQL Managed Database. Requires confirm=true; pass "
            "dry_run=true to preview without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "allow_list": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "IPv4/IPv6 addresses or ranges allowed to connect",
                },
                "engine_config": {
                    "type": "object",
                    "description": "Engine-specific configuration",
                },
                "label": {"type": "string", "description": "Database label"},
                "private_network": {
                    "type": "object",
                    "description": (
                        "Object placing the database in a VPC (vpc_id, "
                        "subnet_id, public_access). Pass null to detach."
                    ),
                },
                "type": {"type": "string", "description": "Linode database plan type"},
                "updates": {
                    "type": "object",
                    "description": "Maintenance update settings",
                },
                "version": {"type": "string", "description": "PostgreSQL version"},
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_instance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_instance_list tool."""
    return Tool(
        name="linode_database_instance_list",
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


def create_linode_database_engine_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_engine_list tool."""
    return Tool(
        name="linode_database_engine_list",
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


def create_linode_database_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_type_list tool."""
    return Tool(
        name="linode_database_type_list",
        description="Lists available Linode Managed Databases types.",
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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return database_engine_to_response_dict(
            await client.get_database_engine(engine_id)
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Managed Databases engine {engine_id}", _call
    )


async def handle_linode_database_type_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_type_get tool request."""
    type_id, error = _validate_database_type_id(arguments.get("type_id"))
    if error is not None or type_id is None:
        return error_response(error or "type_id is required")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return database_type_to_response_dict(
            await client.get_database_type(type_id, page=page, page_size=page_size)
        )

    return await execute_tool(
        cfg, arguments, f"retrieve Managed Databases type {type_id}", _call
    )


def create_linode_database_mysql_instance_credentials_reset_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_database_mysql_instance_credentials_reset tool."""
    return Tool(
        name="linode_database_mysql_instance_credentials_reset",
        description=(
            "Resets credentials for a MySQL Managed Database. Pass dry_run=true "
            "to preview without resetting credentials."
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
                    "description": (
                        "Must be true to confirm database credential reset."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_postgresql_instance_credentials_reset_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_database_postgresql_instance_credentials_reset tool."""
    return Tool(
        name="linode_database_postgresql_instance_credentials_reset",
        description=(
            "Resets credentials for a PostgreSQL Managed Database. Pass "
            "dry_run=true to preview without resetting credentials."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm database credential reset."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


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


def database_ssl_to_response_dict(ssl: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw Managed Database SSL API dict to proto-canonical form."""
    return {"ca_certificate": ssl.get("ca_certificate") or ""}


def create_linode_database_mysql_instance_ssl_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_ssl_get tool."""
    return Tool(
        name="linode_database_mysql_instance_ssl_get",
        description="Gets a MySQL Managed Database SSL certificate.",
        inputSchema=schema("linode.mcp.v1.DatabaseMySQLInstanceSSLGetInput"),
    ), Capability.Read


async def handle_linode_database_mysql_instance_ssl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_ssl_get tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return database_ssl_to_response_dict(
            await client.get_database_mysql_instance_ssl(instance_id)
        )

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve MySQL Managed Database SSL certificate for {instance_id}",
        _call,
    )


def create_linode_database_mysql_instance_credentials_get_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_database_mysql_instance_credentials_get tool."""
    return Tool(
        name="linode_database_mysql_instance_credentials_get",
        description=(
            "Gets credentials for a MySQL Managed Database instance. "
            "This returns sensitive password material, requires confirm=true, "
            "and requires a database write-capable profile. Pass dry_run=true "
            "to preview without retrieving credentials."
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
                    "description": "Must be true to retrieve database credentials.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_postgresql_instance_credentials_get_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_database_postgresql_instance_credentials_get tool."""
    return Tool(
        name="linode_database_postgresql_instance_credentials_get",
        description=(
            "Gets credentials for a PostgreSQL Managed Database instance. "
            "This returns sensitive password material, requires confirm=true, "
            "and requires a database write-capable profile. Pass dry_run=true "
            "to preview without retrieving credentials."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to retrieve database credentials.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_mysql_instance_resume_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_mysql_instance_resume tool."""
    return Tool(
        name="linode_database_mysql_instance_resume",
        description=(
            "Resumes a MySQL Managed Database. Requires confirm=true; pass "
            "dry_run=true to preview without resuming."
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
                    "description": "Must be true to confirm database resume.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def create_linode_database_postgresql_instance_resume_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_resume tool."""
    return Tool(
        name="linode_database_postgresql_instance_resume",
        description=(
            "Resumes a PostgreSQL Managed Database. Requires confirm=true; pass "
            "dry_run=true to preview without resuming."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm database resume.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


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


def create_linode_database_postgresql_instance_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_instance_get tool."""
    return Tool(
        name="linode_database_postgresql_instance_get",
        description="Gets a PostgreSQL Managed Database instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "instance_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "PostgreSQL Managed Database instance ID",
                },
            },
            "required": ["instance_id"],
        },
    ), Capability.Read


def create_linode_database_postgresql_instance_ssl_get_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_database_postgresql_instance_ssl_get tool."""
    return Tool(
        name="linode_database_postgresql_instance_ssl_get",
        description="Gets a PostgreSQL Managed Database SSL certificate.",
        inputSchema=schema("linode.mcp.v1.DatabasePostgreSQLInstanceSSLGetInput"),
    ), Capability.Read


def create_linode_database_postgresql_config_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_database_postgresql_config_get tool."""
    return Tool(
        name="linode_database_postgresql_config_get",
        description="Lists PostgreSQL Managed Database advanced parameters.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


async def handle_linode_database_mysql_instance_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_create tool request."""
    payload, error = _build_mysql_database_payload(arguments)
    if error is not None or payload is None:
        return error_response(error or "invalid database create arguments")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_create",
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

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_mysql_database_instance(payload)

    return await execute_tool(cfg, arguments, "create MySQL Managed Database", _call)


async def handle_linode_database_postgresql_instance_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_create tool request."""
    payload, error = _build_postgresql_database_payload(arguments)
    if error is not None or payload is None:
        return error_response(error or "invalid database create arguments")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_create",
            arguments.get("environment", ""),
            "POST",
            "/databases/postgresql/instances",
            None,
            side_effects=[
                f"A PostgreSQL Managed Database {payload['label']!r} will be "
                "created or restored."
            ],
            warnings=["Creating a Managed Database can incur billing."],
            request_body=payload,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_postgresql_database_instance(payload)

    return await execute_tool(
        cfg, arguments, "create PostgreSQL Managed Database", _call
    )


async def _mysql_instance_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, instance_id: int, delete_path: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_database_mysql_instance(instance_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        return await client.delete_mysql_database_instance(instance_id)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_database_mysql_instance_delete",
        method="DELETE",
        path=delete_path,
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("DatabaseInstance"),
    )


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

    two_stage = await _mysql_instance_delete_two_stage(
        arguments, cfg, instance_id, delete_path
    )
    if two_stage is not None:
        return two_stage

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

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.delete_mysql_database_instance(instance_id)

    return await execute_tool(cfg, arguments, "delete MySQL Managed Database", _call)


async def handle_linode_database_mysql_instance_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_list tool request."""
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


async def handle_linode_database_mysql_instance_suspend(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_suspend tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    encoded_instance_id = quote(str(instance_id), safe="")
    suspend_path = f"/databases/mysql/instances/{encoded_instance_id}/suspend"

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_suspend",
            arguments.get("environment", ""),
            "POST",
            suspend_path,
            None,
            side_effects=[
                f"MySQL Managed Database instance {instance_id} will be suspended."
            ],
            warnings=["Suspending a Managed Database can interrupt service."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.suspend_mysql_database_instance(instance_id)

    return await execute_tool(cfg, arguments, "suspend MySQL Managed Database", _call)


async def handle_linode_database_mysql_instance_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_update tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    payload, error = _build_database_update_payload(arguments)
    if error is not None or payload is None:
        return error_response(error or "invalid database update arguments")

    encoded_instance_id = quote(str(instance_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_update",
            arguments.get("environment", ""),
            "PUT",
            f"/databases/mysql/instances/{encoded_instance_id}",
            None,
            side_effects=[f"MySQL Managed Database {instance_id} will be updated."],
            warnings=["Updating a Managed Database can change service behavior."],
            request_body=payload,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_mysql_database_instance(instance_id, payload)

    return await execute_tool(
        cfg, arguments, f"update MySQL Managed Database {instance_id}", _call
    )


async def handle_linode_database_postgresql_instance_suspend(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_suspend tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    encoded_instance_id = quote(str(instance_id), safe="")
    suspend_path = f"/databases/postgresql/instances/{encoded_instance_id}/suspend"

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_suspend",
            arguments.get("environment", ""),
            "POST",
            suspend_path,
            None,
            side_effects=[
                f"PostgreSQL Managed Database instance {instance_id} will be suspended."
            ],
            warnings=["Suspending a Managed Database can interrupt service."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.suspend_postgresql_database_instance(instance_id)

    return await execute_tool(
        cfg, arguments, "suspend PostgreSQL Managed Database", _call
    )


async def handle_linode_database_postgresql_instance_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_update tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    payload, error = _build_database_update_payload(arguments)
    if error is not None or payload is None:
        return error_response(error or "invalid database update arguments")

    encoded_instance_id = quote(str(instance_id), safe="")
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_update",
            arguments.get("environment", ""),
            "PUT",
            f"/databases/postgresql/instances/{encoded_instance_id}",
            None,
            side_effects=[
                f"PostgreSQL Managed Database {instance_id} will be updated."
            ],
            warnings=["Updating a Managed Database can change service behavior."],
            request_body=payload,
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_postgresql_database_instance(instance_id, payload)

    return await execute_tool(
        cfg, arguments, f"update PostgreSQL Managed Database {instance_id}", _call
    )


async def handle_linode_database_postgresql_instance_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_postgresql_database_instances(
            page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "list Linode PostgreSQL database instances", _call
    )


async def _postgresql_instance_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, instance_id: int, delete_path: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_database_postgresql_instance(instance_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        return await client.delete_postgresql_database_instance(instance_id)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_database_postgresql_instance_delete",
        method="DELETE",
        path=delete_path,
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("DatabaseInstance"),
    )


async def handle_linode_database_postgresql_instance_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_delete tool request."""
    try:
        instance_id = _required_positive_int_argument(arguments, "instance_id")
    except ValueError as exc:
        return error_response(str(exc))

    encoded_instance_id = quote(str(instance_id), safe="")
    delete_path = f"/databases/postgresql/instances/{encoded_instance_id}"

    two_stage = await _postgresql_instance_delete_two_stage(
        arguments, cfg, instance_id, delete_path
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_delete",
            arguments.get("environment", ""),
            "DELETE",
            delete_path,
            None,
            side_effects=[
                f"PostgreSQL Managed Database instance {instance_id} will be deleted."
            ],
            warnings=["Deleting a Managed Database is destructive."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.delete_postgresql_database_instance(instance_id)

    return await execute_tool(
        cfg, arguments, "delete PostgreSQL Managed Database", _call
    )


async def handle_linode_database_mysql_instance_patch(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_patch tool request."""
    try:
        instance_id = _required_positive_int_argument(arguments, "instance_id")
    except ValueError as exc:
        return error_response(str(exc))

    encoded_instance_id = quote(str(instance_id), safe="")
    patch_path = f"/databases/mysql/instances/{encoded_instance_id}/patch"

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_patch",
            arguments.get("environment", ""),
            "POST",
            patch_path,
            None,
            side_effects=[
                f"Pending patches will be applied to MySQL Managed Database "
                f"instance {instance_id}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.patch_mysql_database_instance(instance_id)

    return await execute_tool(cfg, arguments, "patch MySQL Managed Database", _call)


async def handle_linode_database_postgresql_instance_patch(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_patch tool request."""
    try:
        instance_id = _required_positive_int_argument(arguments, "instance_id")
    except ValueError as exc:
        return error_response(str(exc))

    encoded_instance_id = quote(str(instance_id), safe="")
    patch_path = f"/databases/postgresql/instances/{encoded_instance_id}/patch"

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_patch",
            arguments.get("environment", ""),
            "POST",
            patch_path,
            None,
            side_effects=[
                f"Pending patches will be applied to PostgreSQL Managed Database "
                f"instance {instance_id}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.patch_postgresql_database_instance(instance_id)

    return await execute_tool(
        cfg, arguments, "patch PostgreSQL Managed Database", _call
    )


async def handle_linode_database_instance_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_instance_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_database_instances(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode database instances", _call)


async def handle_linode_database_mysql_instance_credentials_reset(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_credentials_reset tool request."""
    try:
        instance_id = _required_positive_int_argument(arguments, "instance_id")
    except ValueError as exc:
        return error_response(str(exc))

    encoded_instance_id = quote(str(instance_id), safe="")
    reset_path = f"/databases/mysql/instances/{encoded_instance_id}/credentials/reset"

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_credentials_reset",
            arguments.get("environment", ""),
            "POST",
            reset_path,
            None,
            side_effects=[
                (
                    f"MySQL Managed Database instance {instance_id} credentials "
                    "will be reset."
                )
            ],
            warnings=[
                "Resetting credentials can disrupt clients using the old credentials."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.reset_mysql_database_credentials(instance_id)

    return await execute_tool(
        cfg, arguments, "reset MySQL Managed Database credentials", _call
    )


async def handle_linode_database_postgresql_instance_credentials_reset(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_credentials_reset tool request."""
    try:
        instance_id = _required_positive_int_argument(arguments, "instance_id")
    except ValueError as exc:
        return error_response(str(exc))

    encoded_instance_id = quote(str(instance_id), safe="")
    reset_path = (
        f"/databases/postgresql/instances/{encoded_instance_id}/credentials/reset"
    )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_credentials_reset",
            arguments.get("environment", ""),
            "POST",
            reset_path,
            None,
            side_effects=[
                (
                    f"PostgreSQL Managed Database instance {instance_id} credentials "
                    "will be reset."
                )
            ],
            warnings=[
                "Resetting credentials can disrupt clients using the old credentials."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.reset_postgresql_database_credentials(instance_id)

    return await execute_tool(
        cfg, arguments, "reset PostgreSQL Managed Database credentials", _call
    )


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


async def handle_linode_database_mysql_instance_credentials_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_credentials_get tool request."""
    try:
        instance_id = _optional_int_argument(arguments, "instance_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if instance_id is None:
        return error_response("instance_id is required")

    encoded_instance_id = quote(str(instance_id), safe="")
    credentials_path = f"/databases/mysql/instances/{encoded_instance_id}/credentials"

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_credentials_get",
            arguments.get("environment", ""),
            "GET",
            credentials_path,
            None,
            side_effects=[
                "MySQL Managed Database credentials will be retrieved and exposed."
            ],
            warnings=["The response contains sensitive password material."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_mysql_instance_credentials(instance_id)

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve MySQL Managed Database credentials for instance {instance_id}",
        _call,
    )


async def handle_linode_database_postgresql_instance_credentials_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_credentials_get tool request."""
    try:
        instance_id = _optional_int_argument(arguments, "instance_id", 1)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if instance_id is None:
        return error_response("instance_id is required")

    encoded_instance_id = quote(str(instance_id), safe="")
    credentials_path = (
        f"/databases/postgresql/instances/{encoded_instance_id}/credentials"
    )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_credentials_get",
            arguments.get("environment", ""),
            "GET",
            credentials_path,
            None,
            side_effects=[
                "PostgreSQL Managed Database credentials will be retrieved and exposed."
            ],
            warnings=["The response contains sensitive password material."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_postgresql_instance_credentials(instance_id)

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve PostgreSQL Managed Database credentials for instance {instance_id}",
        _call,
    )


async def handle_linode_database_mysql_instance_resume(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_mysql_instance_resume tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    encoded_instance_id = quote(str(instance_id), safe="")
    resume_path = f"/databases/mysql/instances/{encoded_instance_id}/resume"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_mysql_instance_resume",
            arguments.get("environment", ""),
            "POST",
            resume_path,
            None,
            side_effects=[f"MySQL Managed Database {instance_id} will be resumed."],
            warnings=["Resuming a Managed Database changes service state."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.resume_mysql_database_instance(instance_id)

    return await execute_tool(
        cfg, arguments, f"resume MySQL Managed Database {instance_id}", _call
    )


async def handle_linode_database_postgresql_instance_resume(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_resume tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    encoded_instance_id = quote(str(instance_id), safe="")
    resume_path = f"/databases/postgresql/instances/{encoded_instance_id}/resume"
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_database_postgresql_instance_resume",
            arguments.get("environment", ""),
            "POST",
            resume_path,
            None,
            side_effects=[
                f"PostgreSQL Managed Database {instance_id} will be resumed."
            ],
            warnings=["Resuming a Managed Database changes service state."],
        )

    if arguments.get("confirm") is not True:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.resume_postgresql_database_instance(instance_id)

    return await execute_tool(
        cfg, arguments, f"resume PostgreSQL Managed Database {instance_id}", _call
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


async def handle_linode_database_postgresql_instance_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_get tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_postgresql_instance(instance_id)

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve PostgreSQL Managed Database instance {instance_id}",
        _call,
    )


async def handle_linode_database_postgresql_instance_ssl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_instance_ssl_get tool request."""
    instance_id, error = _validate_instance_id(arguments.get("instance_id"))
    if error is not None or instance_id is None:
        return error_response(error or "instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return database_ssl_to_response_dict(
            await client.get_database_postgresql_instance_ssl(instance_id)
        )

    return await execute_tool(
        cfg,
        arguments,
        f"retrieve PostgreSQL Managed Database SSL certificate for {instance_id}",
        _call,
    )


async def handle_linode_database_postgresql_config_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_postgresql_config_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_database_postgresql_config()

    return await execute_tool(
        cfg,
        arguments,
        "retrieve PostgreSQL Managed Database advanced parameters",
        _call,
    )


async def handle_linode_database_engine_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_engine_list tool request."""
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


async def handle_linode_database_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_database_type_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_database_types(page=page, page_size=page_size)
        database_types = data.get("data", [])
        return {
            "message": "Database types listed",
            "count": len(database_types),
            "types": database_types,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list database types", _call)

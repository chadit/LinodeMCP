"""Linode Managed Databases tools."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_DATABASE_ENGINE_ID_PATTERN = re.compile(r"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$")


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

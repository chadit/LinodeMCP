"""Longview tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

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

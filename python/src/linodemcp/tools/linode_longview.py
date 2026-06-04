"""Longview READ tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

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

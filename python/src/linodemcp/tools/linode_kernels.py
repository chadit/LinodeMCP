"""Linode kernels list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def _optional_int_argument(
    arguments: dict[str, Any],
    name: str,
    minimum: int,
    maximum: int | None = None,
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


def create_linode_kernels_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_kernels_list tool."""
    return Tool(
        name="linode_kernels_list",
        description="Lists available Linode kernels.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Page size for results",
                },
            },
        },
    ), Capability.Read


async def handle_linode_kernels_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_kernels_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_kernels(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode kernels", _call)

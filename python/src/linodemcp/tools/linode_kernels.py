"""Linode kernel tools."""

from __future__ import annotations

import re
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


def create_linode_kernel_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_kernel_list tool."""
    return Tool(
        name="linode_kernel_list",
        description="Lists available Linode kernels.",
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
                    "description": "Page size for results",
                },
            },
        },
    ), Capability.Read


async def handle_linode_kernel_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_kernel_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_kernels(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode kernels", _call)


_KERNEL_ID_PATTERN = re.compile(r"^(?!.*\.\.)linode/[A-Za-z0-9._-]+$")


def _validated_kernel_id(value: object) -> tuple[str | None, str | None]:
    """Validate and normalize a Linode kernel ID path parameter."""
    if not isinstance(value, str) or not value.strip():
        return None, "kernel_id is required"
    kernel_id = value.strip()
    if not _KERNEL_ID_PATTERN.fullmatch(kernel_id):
        return None, "kernel_id must look like linode/latest-64bit"
    return kernel_id, None


def create_linode_kernel_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_kernel_get tool."""
    return Tool(
        name="linode_kernel_get",
        description="Gets a single Linode kernel by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "kernel_id": {
                    "type": "string",
                    "pattern": _KERNEL_ID_PATTERN.pattern,
                    "description": ("Kernel ID such as linode/latest-64bit (required)"),
                },
            },
            "required": ["kernel_id"],
        },
    ), Capability.Read


async def handle_linode_kernel_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_kernel_get tool request."""
    kernel_id, kernel_id_err = _validated_kernel_id(arguments.get("kernel_id"))
    if kernel_id_err is not None or kernel_id is None:
        return error_response(kernel_id_err or "kernel_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        kernel = await client.get_kernel(kernel_id)
        return {"kernel": kernel}

    return await execute_tool(cfg, arguments, "retrieve Linode kernel", _call)

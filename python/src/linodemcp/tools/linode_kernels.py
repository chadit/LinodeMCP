"""Linode kernel tools."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import kernel_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    error_response,
    execute_tool,
    pagination_int_argument,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_kernel_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_kernel_list tool."""
    return Tool(
        name="linode_kernel_list",
        description="Lists available Linode kernels.",
        inputSchema=schema("linode.mcp.v1.KernelListInput"),
    ), Capability.Read


async def handle_linode_kernel_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_kernel_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_kernels(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "kernels",
            kernel_pb2.KernelListResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.KernelGetInput"),
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
        return serialize_api_response(kernel, kernel_pb2.Kernel())

    return await execute_tool(cfg, arguments, "retrieve Linode kernel", _call)

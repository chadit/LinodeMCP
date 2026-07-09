"""MCP tools for Linode network transfer pricing."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import type_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool
from linodemcp.tools.proto_response import serialize_list_response
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_network_transfer_price_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_network_transfer_price_list tool."""
    return Tool(
        name="linode_network_transfer_price_list",
        description="Gets network transfer prices.",
        inputSchema=schema("linode.mcp.v1.NetworkTransferPriceListInput"),
    ), Capability.Read


async def handle_linode_network_transfer_price_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_network_transfer_price_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_network_transfer_prices()
        return serialize_list_response(
            raw,
            "network_transfer_prices",
            type_pb2.NetworkTransferPriceListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve network transfer prices", _call)

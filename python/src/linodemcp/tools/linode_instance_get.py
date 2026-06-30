"""Linode instance get tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import instance_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_instance_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_get tool."""
    return Tool(
        name="linode_instance_get",
        description="Retrieves details of a single Linode instance by its ID",
        inputSchema=schema("linode.mcp.v1.InstanceGetInput"),
    ), Capability.Read


async def handle_linode_instance_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_get tool request.

    Args:
        arguments: InstanceIDArgs - instance_id, environment (optional)
        cfg: Configuration object
    """
    instance_id_str = arguments.get("instance_id", "")

    if not instance_id_str:
        return error_response("instance_id is required")

    try:
        instance_id = int(instance_id_str)
    except ValueError:
        return error_response("instance_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw(f"/linode/instances/{instance_id}")
        return serialize_api_response(raw, instance_pb2.Instance())

    return await execute_tool(cfg, arguments, "retrieve Linode instance", _call)

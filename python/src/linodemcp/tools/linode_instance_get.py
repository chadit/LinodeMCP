"""Linode instance get tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import instance_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool, required_int_id
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_instance_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_get tool."""
    return Tool(
        name="linode_instance_get",
        description="Retrieves details of a single Linode instance by its ID",
        input_schema=schema("linode.mcp.v1.InstanceGetInput"),
    ), Capability.Read


async def handle_linode_instance_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_get tool request.

    Args:
        arguments: InstanceIDArgs - instance_id, environment (optional)
        cfg: Configuration object
    """
    instance_id, instance_id_error = required_int_id(arguments, "instance_id")
    if instance_id is None:
        return error_response(instance_id_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.route_raw("linode_instance_get", instance_id)
        return serialize_api_response(raw, instance_pb2.Instance())

    return await execute_tool(cfg, arguments, "retrieve Linode instance", _call)

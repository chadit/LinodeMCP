"""Hello tool - friendly greeting."""

import json
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import version_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.proto_response import proto_to_canonical_dict
from linodemcp.tools.toolschemas import schema


def create_hello_tool() -> tuple[Tool, Capability]:
    """Create the hello tool."""
    return Tool(
        name="hello",
        description="Responds with a friendly greeting from LinodeMCP",
        inputSchema=schema("linode.mcp.v1.HelloInput"),
    ), Capability.Meta


async def handle_hello(arguments: dict[str, Any]) -> list[TextContent]:
    """Handle hello tool request.

    Args:
        arguments: HelloArgs - name (optional)
    """
    name = arguments.get("name", "World")
    message = version_pb2.HelloResponse(
        message=f"Hello, {name}! LinodeMCP server is running and ready."
    )
    return [
        TextContent(
            type="text", text=json.dumps(proto_to_canonical_dict(message), indent=2)
        )
    ]

"""Version tool - server version and build information."""

import json
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.version import get_version_info


def create_version_tool() -> Tool:
    """Create the version tool."""
    return Tool(
        name="version",
        description="Returns LINodeMCP server version and build information",
        inputSchema={
            "type": "object",
            "properties": {},
        },
    )


async def handle_version(_arguments: dict[str, Any]) -> list[TextContent]:
    """Handle version tool request."""
    version_info = get_version_info()
    json_response = json.dumps(version_info.to_dict(), indent=2)
    return [TextContent(type="text", text=json_response)]

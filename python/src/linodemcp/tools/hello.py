"""Hello tool - friendly greeting."""

from typing import Any

from mcp.types import TextContent, Tool


def create_hello_tool() -> Tool:
    """Create the hello tool."""
    return Tool(
        name="hello",
        description="Responds with a friendly greeting from LinodeMCP",
        inputSchema={
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Name to include in the greeting (optional)",
                },
            },
        },
    )


async def handle_hello(arguments: dict[str, Any]) -> list[TextContent]:
    """Handle hello tool request.

    Args:
        arguments: HelloArgs - name (optional)
    """
    name = arguments.get("name", "World")
    message = f"Hello, {name}! LinodeMCP server is running and ready."
    return [TextContent(type="text", text=message)]

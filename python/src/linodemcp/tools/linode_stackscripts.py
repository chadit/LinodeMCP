from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import (
    DESCRIPTION_TRUNCATE_LIMIT,
    execute_tool,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_stackscripts_list_tool() -> Tool:
    """Create the linode_stackscripts_list tool."""
    return Tool(
        name="linode_stackscripts_list",
        description=(
            "Lists StackScripts. By default returns your own StackScripts. "
            "Can filter by public status, ownership, or label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "is_public": {
                    "type": "string",
                    "description": "Filter by public status (true, false)",
                },
                "mine": {
                    "type": "string",
                    "description": (
                        "Filter by ownership - only your own StackScripts (true, false)"
                    ),
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter StackScripts by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_stackscripts_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscripts_list tool request."""
    is_public_filter = arguments.get("is_public", "")
    mine_filter = arguments.get("mine", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        scripts = await client.list_stackscripts()

        if is_public_filter:
            want_public = is_public_filter.lower() == "true"
            scripts = [s for s in scripts if s.is_public == want_public]

        if mine_filter:
            want_mine = mine_filter.lower() == "true"
            scripts = [s for s in scripts if s.mine == want_mine]

        if label_contains:
            scripts = [s for s in scripts if label_contains.lower() in s.label.lower()]

        scripts_data = [
            {
                "id": s.id,
                "label": s.label,
                "username": s.username,
                "description": _truncate_string(
                    s.description, DESCRIPTION_TRUNCATE_LIMIT
                ),
                "is_public": s.is_public,
                "mine": s.mine,
                "deployments_total": s.deployments_total,
                "deployments_active": s.deployments_active,
                "created": s.created,
                "updated": s.updated,
            }
            for s in scripts
        ]

        response: dict[str, Any] = {
            "count": len(scripts),
            "stackscripts": scripts_data,
        }

        filters: list[str] = []
        if is_public_filter:
            filters.append(f"is_public={is_public_filter}")
        if mine_filter:
            filters.append(f"mine={mine_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve StackScripts", _call)


def _truncate_string(value: str, limit: int) -> str:
    """Truncate a string with ellipsis if it exceeds the limit."""
    if len(value) > limit:
        return value[:limit] + "..."
    return value

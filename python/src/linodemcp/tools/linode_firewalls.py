from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_firewalls_list_tool() -> Tool:
    """Create the linode_firewalls_list tool."""
    return Tool(
        name="linode_firewalls_list",
        description=(
            "Lists all Cloud Firewalls on your account. Can filter by status or label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "status": {
                    "type": "string",
                    "description": (
                        "Filter by firewall status (enabled, disabled, deleted)"
                    ),
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter firewalls by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_firewalls_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewalls_list tool request."""
    status_filter = arguments.get("status", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewalls = await client.list_firewalls()

        if status_filter:
            firewalls = [
                f for f in firewalls if f.status.lower() == status_filter.lower()
            ]

        if label_contains:
            firewalls = [
                f for f in firewalls if label_contains.lower() in f.label.lower()
            ]

        firewalls_data = [
            {
                "id": f.id,
                "label": f.label,
                "status": f.status,
                "rules_inbound_count": len(f.rules.inbound),
                "rules_outbound_count": len(f.rules.outbound),
                "created": f.created,
                "updated": f.updated,
            }
            for f in firewalls
        ]

        response: dict[str, Any] = {
            "count": len(firewalls),
            "firewalls": firewalls_data,
        }

        filters: list[str] = []
        if status_filter:
            filters.append(f"status={status_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve firewalls", _call)

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_nodebalancers_list_tool() -> Tool:
    """Create the linode_nodebalancers_list tool."""
    return Tool(
        name="linode_nodebalancers_list",
        description=(
            "Lists all NodeBalancers on your account. Can filter by region or label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "region": {
                    "type": "string",
                    "description": "Filter by region ID (e.g., us-east, eu-west)",
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter NodeBalancers by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_nodebalancers_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancers_list tool request."""
    region_filter = arguments.get("region", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nodebalancers = await client.list_nodebalancers()

        if region_filter:
            nodebalancers = [
                nb for nb in nodebalancers if nb.region.lower() == region_filter.lower()
            ]

        if label_contains:
            nodebalancers = [
                nb for nb in nodebalancers if label_contains.lower() in nb.label.lower()
            ]

        nodebalancers_data = [
            {
                "id": nb.id,
                "label": nb.label,
                "region": nb.region,
                "hostname": nb.hostname,
                "ipv4": nb.ipv4,
                "created": nb.created,
                "updated": nb.updated,
            }
            for nb in nodebalancers
        ]

        response: dict[str, Any] = {
            "count": len(nodebalancers),
            "nodebalancers": nodebalancers_data,
        }

        filters: list[str] = []
        if region_filter:
            filters.append(f"region={region_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve NodeBalancers", _call)


def create_linode_nodebalancer_get_tool() -> Tool:
    """Create the linode_nodebalancer_get tool."""
    return Tool(
        name="linode_nodebalancer_get",
        description=(
            "Gets detailed information about a specific NodeBalancer by its ID."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "description": "The ID of the NodeBalancer to retrieve (required)",
                },
            },
            "required": ["nodebalancer_id"],
        },
    )


async def handle_linode_nodebalancer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_get tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return _error_response("nodebalancer_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.get_nodebalancer(int(nodebalancer_id))
        return {
            "id": nb.id,
            "label": nb.label,
            "region": nb.region,
            "hostname": nb.hostname,
            "ipv4": nb.ipv4,
            "ipv6": nb.ipv6,
            "client_conn_throttle": nb.client_conn_throttle,
            "transfer": {
                "in": nb.transfer.in_,
                "out": nb.transfer.out,
                "total": nb.transfer.total,
            },
            "tags": nb.tags,
            "created": nb.created,
            "updated": nb.updated,
        }

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer", _call)

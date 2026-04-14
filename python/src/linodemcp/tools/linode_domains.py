from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_domains_list_tool() -> Tool:
    """Create the linode_domains_list tool."""
    return Tool(
        name="linode_domains_list",
        description=(
            "Lists all domains managed by your Linode account. "
            "Can filter by domain name or type (master/slave)."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_contains": {
                    "type": "string",
                    "description": (
                        "Filter domains by name containing this string "
                        "(case-insensitive)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": "Filter by domain type (master, slave)",
                },
            },
        },
    )


async def handle_linode_domains_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domains_list tool request."""
    domain_contains = arguments.get("domain_contains", "")
    type_filter = arguments.get("type", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domains = await client.list_domains()

        if domain_contains:
            domains = [
                d for d in domains if domain_contains.lower() in d.domain.lower()
            ]

        if type_filter:
            domains = [d for d in domains if d.type.lower() == type_filter.lower()]

        domains_data = [
            {
                "id": d.id,
                "domain": d.domain,
                "type": d.type,
                "status": d.status,
                "soa_email": d.soa_email,
                "created": d.created,
                "updated": d.updated,
            }
            for d in domains
        ]

        response: dict[str, Any] = {
            "count": len(domains),
            "domains": domains_data,
        }

        filters: list[str] = []
        if domain_contains:
            filters.append(f"domain_contains={domain_contains}")
        if type_filter:
            filters.append(f"type={type_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve domains", _call)


def create_linode_domain_get_tool() -> Tool:
    """Create the linode_domain_get tool."""
    return Tool(
        name="linode_domain_get",
        description="Gets detailed information about a specific domain by its ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to retrieve (required)",
                },
            },
            "required": ["domain_id"],
        },
    )


async def handle_linode_domain_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_get tool request."""
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.get_domain(int(domain_id))
        return {
            "id": domain.id,
            "domain": domain.domain,
            "type": domain.type,
            "status": domain.status,
            "soa_email": domain.soa_email,
            "description": domain.description,
            "tags": domain.tags,
            "created": domain.created,
            "updated": domain.updated,
        }

    return await execute_tool(cfg, arguments, "retrieve domain", _call)

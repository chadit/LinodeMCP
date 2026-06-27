from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def domain_to_response_dict(domain: Any) -> dict[str, Any]:
    """Shape a Domain dataclass to proto-canonical linode.mcp.v1.Domain form.

    Field order follows the proto field numbers; master_ips, axfr_ips, and tags
    are always emitted as lists.
    """
    return {
        "id": domain.id,
        "domain": domain.domain,
        "type": domain.type,
        "status": domain.status,
        "soa_email": domain.soa_email,
        "description": domain.description,
        "retry_sec": domain.retry_sec,
        "master_ips": domain.master_ips or [],
        "axfr_ips": domain.axfr_ips or [],
        "expire_sec": domain.expire_sec,
        "refresh_sec": domain.refresh_sec,
        "ttl_sec": domain.ttl_sec,
        "tags": domain.tags or [],
        "created": domain.created,
        "updated": domain.updated,
        "group": domain.group,
    }


def create_linode_domain_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_list tool."""
    return Tool(
        name="linode_domain_list",
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
    ), Capability.Read


async def handle_linode_domain_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_list tool request."""
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


def _validate_domain_id(value: Any) -> int | None:
    """Return a valid domain ID or None for invalid input."""
    if type(value) is not int or value <= 0:
        return None
    return value


def create_linode_domain_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_get tool."""
    return Tool(
        name="linode_domain_get",
        description="Gets detailed information about a specific domain by its ID.",
        inputSchema=schema("linode.mcp.v1.DomainGetInput"),
    ), Capability.Read


async def handle_linode_domain_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_get tool request."""
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.get_domain(int(domain_id))
        return domain_to_response_dict(domain)

    return await execute_tool(cfg, arguments, "retrieve domain", _call)


def create_linode_domain_zone_file_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_zone_file_get tool."""
    return Tool(
        name="linode_domain_zone_file_get",
        description="Gets the generated zone file for a specific domain by its ID.",
        inputSchema=schema("linode.mcp.v1.DomainZoneFileGetInput"),
    ), Capability.Read


async def handle_linode_domain_zone_file_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_zone_file_get tool request."""
    domain_id = _validate_domain_id(arguments.get("domain_id"))
    if domain_id is None:
        return error_response("domain_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        zone_file = await client.get_domain_zone_file(domain_id)
        return {"zone_file": zone_file.zone_file or []}

    return await execute_tool(cfg, arguments, "retrieve domain zone file", _call)

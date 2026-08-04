from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import domain_zone_file_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.proto_response import serialize_api_response
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


def _validate_domain_id(value: Any) -> int | None:
    """Return a valid domain ID or None for invalid input."""
    if type(value) is not int or value <= 0:
        return None
    return value


def create_linode_domain_zone_file_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_zone_file_get tool."""
    return Tool(
        name="linode_domain_zone_file_get",
        description="Gets the generated zone file for a specific domain by its ID.",
        input_schema=schema("linode.mcp.v1.DomainZoneFileGetInput"),
    ), Capability.Read


async def handle_linode_domain_zone_file_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_zone_file_get tool request."""
    domain_id = _validate_domain_id(arguments.get("domain_id"))
    if domain_id is None:
        return error_response("domain_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.route_raw("linode_domain_zone_file_get", domain_id)
        return serialize_api_response(raw, domain_zone_file_pb2.DomainZoneFile())

    return await execute_tool(cfg, arguments, "retrieve domain zone file", _call)

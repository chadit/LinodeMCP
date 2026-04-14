from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_domain_records_list_tool() -> Tool:
    """Create the linode_domain_records_list tool."""
    return Tool(
        name="linode_domain_records_list",
        description=(
            "Lists all DNS records for a specific domain. "
            "Can filter by record type or name."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the domain to list records for (required)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": (
                        "Filter by record type (A, AAAA, NS, MX, CNAME, TXT, SRV, CAA)"
                    ),
                },
                "name_contains": {
                    "type": "string",
                    "description": (
                        "Filter records by name containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
            "required": ["domain_id"],
        },
    )


async def handle_linode_domain_records_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_records_list tool request."""
    domain_id = arguments.get("domain_id", 0)
    type_filter = arguments.get("type", "")
    name_contains = arguments.get("name_contains", "")

    if not domain_id:
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        records = await client.list_domain_records(int(domain_id))

        if type_filter:
            records = [r for r in records if r.type.upper() == type_filter.upper()]

        if name_contains:
            records = [r for r in records if name_contains.lower() in r.name.lower()]

        records_data = [
            {
                "id": r.id,
                "type": r.type,
                "name": r.name,
                "target": r.target,
                "priority": r.priority,
                "ttl_sec": r.ttl_sec,
            }
            for r in records
        ]

        response: dict[str, Any] = {
            "count": len(records),
            "domain_id": domain_id,
            "records": records_data,
        }

        filters: list[str] = []
        if type_filter:
            filters.append(f"type={type_filter}")
        if name_contains:
            filters.append(f"name_contains={name_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve domain records", _call)


def create_linode_domain_record_create_tool() -> Tool:
    """Create the linode_domain_record_create tool."""
    return Tool(
        name="linode_domain_record_create",
        description="Creates a new DNS record for a domain.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain (required)",
                },
                "type": {
                    "type": "string",
                    "description": (
                        "Record type: A, AAAA, NS, MX, CNAME, TXT, SRV, CAA (required)"
                    ),
                },
                "name": {
                    "type": "string",
                    "description": "Record name/subdomain (optional)",
                },
                "target": {
                    "type": "string",
                    "description": (
                        "Target value for the record (required for most types)"
                    ),
                },
                "priority": {
                    "type": "integer",
                    "description": "Priority (for MX and SRV records)",
                },
                "weight": {
                    "type": "integer",
                    "description": "Weight (for SRV records)",
                },
                "port": {
                    "type": "integer",
                    "description": "Port (for SRV records)",
                },
                "ttl_sec": {
                    "type": "integer",
                    "description": "TTL in seconds (optional)",
                },
            },
            "required": ["domain_id", "type"],
        },
    )


async def handle_linode_domain_record_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_create tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_type = arguments.get("type", "")

    if not domain_id:
        return _error_response("domain_id is required")
    if not record_type:
        return _error_response("type is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        record = await client.create_domain_record(
            domain_id=int(domain_id),
            record_type=record_type,
            name=arguments.get("name"),
            target=arguments.get("target"),
            priority=arguments.get("priority"),
            weight=arguments.get("weight"),
            port=arguments.get("port"),
            ttl_sec=arguments.get("ttl_sec"),
        )
        return {
            "message": (
                f"DNS record (ID: {record.id}) created successfully "
                f"for domain {domain_id}"
            ),
            "record": {
                "id": record.id,
                "type": record.type,
                "name": record.name,
                "target": record.target,
                "ttl_sec": record.ttl_sec,
            },
        }

    return await execute_tool(cfg, arguments, "create DNS record", _call)


def create_linode_domain_record_update_tool() -> Tool:
    """Create the linode_domain_record_update tool."""
    return Tool(
        name="linode_domain_record_update",
        description="Updates an existing DNS record.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain (required)",
                },
                "record_id": {
                    "type": "integer",
                    "description": "The ID of the record to update (required)",
                },
                "name": {
                    "type": "string",
                    "description": "New record name (optional)",
                },
                "target": {
                    "type": "string",
                    "description": "New target value (optional)",
                },
                "priority": {
                    "type": "integer",
                    "description": "New priority (optional)",
                },
                "weight": {
                    "type": "integer",
                    "description": "New weight (optional)",
                },
                "port": {
                    "type": "integer",
                    "description": "New port (optional)",
                },
                "ttl_sec": {
                    "type": "integer",
                    "description": "New TTL in seconds (optional)",
                },
            },
            "required": ["domain_id", "record_id"],
        },
    )


async def handle_linode_domain_record_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_update tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")
    if not record_id:
        return _error_response("record_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        record = await client.update_domain_record(
            domain_id=int(domain_id),
            record_id=int(record_id),
            name=arguments.get("name"),
            target=arguments.get("target"),
            priority=arguments.get("priority"),
            weight=arguments.get("weight"),
            port=arguments.get("port"),
            ttl_sec=arguments.get("ttl_sec"),
        )
        return {
            "message": f"DNS record {record_id} updated successfully",
            "record": {
                "id": record.id,
                "type": record.type,
                "name": record.name,
                "target": record.target,
                "ttl_sec": record.ttl_sec,
            },
        }

    return await execute_tool(cfg, arguments, "update DNS record", _call)


def create_linode_domain_record_delete_tool() -> Tool:
    """Create the linode_domain_record_delete tool."""
    return Tool(
        name="linode_domain_record_delete",
        description="Deletes a DNS record.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain (required)",
                },
                "record_id": {
                    "type": "integer",
                    "description": "The ID of the record to delete (required)",
                },
            },
            "required": ["domain_id", "record_id"],
        },
    )


async def handle_linode_domain_record_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_delete tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")
    if not record_id:
        return _error_response("record_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain_record(int(domain_id), int(record_id))
        return {
            "message": f"DNS record {record_id} deleted successfully",
            "domain_id": domain_id,
            "record_id": record_id,
        }

    return await execute_tool(cfg, arguments, "delete DNS record", _call)

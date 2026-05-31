from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    PARAM_DRY_RUN,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_domain_records_list_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


def create_linode_domain_record_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_record_get tool."""
    return Tool(
        name="linode_domain_record_get",
        description="Gets a specific DNS record for a domain.",
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
                    "description": "The ID of the record to retrieve (required)",
                },
            },
            "required": ["domain_id", "record_id"],
        },
    ), Capability.Read


async def handle_linode_domain_record_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_get tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return error_response("domain_id is required")
    if not record_id:
        return error_response("record_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        record = await client.get_domain_record(int(domain_id), int(record_id))
        return {
            "domain_id": domain_id,
            "record": {
                "id": record.id,
                "type": record.type,
                "name": record.name,
                "target": record.target,
                "priority": record.priority,
                "ttl_sec": record.ttl_sec,
            },
        }

    return await execute_tool(cfg, arguments, "retrieve domain record", _call)


async def handle_linode_domain_records_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_records_list tool request."""
    domain_id = arguments.get("domain_id", 0)
    type_filter = arguments.get("type", "")
    name_contains = arguments.get("name_contains", "")

    if not domain_id:
        return error_response("domain_id is required")

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


def create_linode_domain_record_create_tool() -> tuple[Tool, Capability]:
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm this operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain_id", "type", "confirm"],
        },
    ), Capability.Write


def _record_create_error(domain_id: Any, record_type: str) -> list[TextContent] | None:
    """Validate record-create args; return an error response or None."""
    if not domain_id:
        return error_response("domain_id is required")
    if not record_type:
        return error_response("type is required")
    return None


async def handle_linode_domain_record_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_create tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_type = arguments.get("type", "")

    if is_dry_run(arguments):
        fields_error = _record_create_error(domain_id, record_type)
        if fields_error is not None:
            return fields_error
        effect = (
            f"A new {record_type} record will be created in domain {int(domain_id)}"
        )
        name = arguments.get("name", "")
        target = arguments.get("target", "")
        if name:
            effect += f" for host {name!r}"
        if target:
            effect += f" targeting {target!r}"
        return build_dry_run_response(
            "linode_domain_record_create",
            arguments.get("environment", ""),
            "POST",
            f"/domains/{int(domain_id)}/records",
            None,
            side_effects=[f"{effect}."],
        )

    if not arguments.get("confirm"):
        return error_response("This creates a DNS record. Set confirm=true to proceed.")

    fields_error = _record_create_error(domain_id, record_type)
    if fields_error is not None:
        return fields_error

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


def create_linode_domain_record_update_tool() -> tuple[Tool, Capability]:
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm this operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain_id", "record_id", "confirm"],
        },
    ), Capability.Write


def _record_update_error(domain_id: Any, record_id: Any) -> list[TextContent] | None:
    """Validate record-update args; return an error response or None."""
    if not domain_id:
        return error_response("domain_id is required")
    if not record_id:
        return error_response("record_id is required")
    return None


def _domain_record_update_side_effects(
    state: Any, new_name: Any, new_target: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for domain record update. Reports the record name
    and target changes against the fetched state.
    """
    side_effects: list[str] = []
    if new_name is not None:
        from_name = getattr(state, "name", "")
        if new_name != from_name:
            side_effects.append(
                f"Record name changes from {from_name!r} to {new_name!r}."
            )
    if new_target:
        from_target = getattr(state, "target", "")
        if new_target != from_target:
            side_effects.append(
                f"Record target changes from {from_target!r} to {new_target!r}."
            )
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_domain_record_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_update tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if is_dry_run(arguments):
        fields_error = _record_update_error(domain_id, record_id)
        if fields_error is not None:
            return fields_error

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain_record(int(domain_id), int(record_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _domain_record_update_side_effects(
                state, arguments.get("name"), arguments.get("target")
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_record_update",
            "PUT",
            f"/domains/{int(domain_id)}/records/{int(record_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This updates a DNS record. Set confirm=true to proceed.")

    fields_error = _record_update_error(domain_id, record_id)
    if fields_error is not None:
        return fields_error

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


def create_linode_domain_record_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_record_delete tool."""
    return Tool(
        name="linode_domain_record_delete",
        description=(
            "Deletes a DNS record. Pass dry_run=true to preview without deleting."
        ),
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm this operation. Ignored when "
                        "dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain_id", "record_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_domain_record_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_delete tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    # ID validation runs before both branches: dry-run and real call both
    # need both IDs, and the spec is explicit that dry-run errors out on
    # missing required args the same way the real call would.
    if not domain_id:
        return error_response("domain_id is required")
    if not record_id:
        return error_response("record_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain_record(int(domain_id), int(record_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_record_delete",
            "DELETE",
            f"/domains/{int(domain_id)}/records/{int(record_id)}",
            _fetch,
        )

    if not arguments.get("confirm"):
        return error_response(
            "This deletes a DNS record and is irreversible. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain_record(int(domain_id), int(record_id))
        return {
            "message": f"DNS record {record_id} deleted successfully",
            "domain_id": domain_id,
            "record_id": record_id,
        }

    return await execute_tool(cfg, arguments, "delete DNS record", _call)

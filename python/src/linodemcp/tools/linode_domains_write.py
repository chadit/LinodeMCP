from __future__ import annotations

from typing import TYPE_CHECKING, Any

import httpx
from mcp.types import TextContent, Tool

from linodemcp.linode import APIError, NetworkError
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


def create_linode_domain_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_create tool."""
    return Tool(
        name="linode_domain_create",
        description=(
            "Creates a new DNS domain. Pass dry_run=true to preview without creating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain": {
                    "type": "string",
                    "description": "The domain name (required)",
                },
                "type": {
                    "type": "string",
                    "description": (
                        "Domain type: 'master' or 'slave' (default: 'master')"
                    ),
                },
                "soa_email": {
                    "type": "string",
                    "description": "SOA email address (required for master domains)",
                },
                "description": {
                    "type": "string",
                    "description": "Description for the domain (optional)",
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
            "required": ["domain", "confirm"],
        },
    ), Capability.Write


async def handle_linode_domain_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_create tool request."""
    domain_name = arguments.get("domain", "")

    if is_dry_run(arguments):
        if not domain_name:
            return error_response("domain is required")
        domain_type = arguments.get("type", "master")
        return build_dry_run_response(
            "linode_domain_create",
            arguments.get("environment", ""),
            "POST",
            "/domains",
            None,
            side_effects=[
                f"A new {domain_type} DNS domain {domain_name!r} will be created."
            ],
        )

    if not arguments.get("confirm"):
        return error_response("This creates a DNS domain. Set confirm=true to proceed.")

    if not domain_name:
        return error_response("domain is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.create_domain(
            domain=domain_name,
            domain_type=arguments.get("type", "master"),
            soa_email=arguments.get("soa_email"),
            description=arguments.get("description"),
        )
        return {
            "message": (
                f"Domain '{domain.domain}' (ID: {domain.id}) created successfully"
            ),
            "domain": {
                "id": domain.id,
                "domain": domain.domain,
                "type": domain.type,
                "status": domain.status,
                "created": domain.created,
            },
        }

    return await execute_tool(cfg, arguments, "create domain", _call)


def create_linode_domain_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_update tool."""
    return Tool(
        name="linode_domain_update",
        description=(
            "Updates an existing DNS domain. Pass dry_run=true to preview "
            "without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to update (required)",
                },
                "domain": {
                    "type": "string",
                    "description": "New domain name (optional)",
                },
                "soa_email": {
                    "type": "string",
                    "description": "New SOA email address (optional)",
                },
                "description": {
                    "type": "string",
                    "description": "New description (optional)",
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
            "required": ["domain_id", "confirm"],
        },
    ), Capability.Write


def _domain_update_side_effects(
    state: Any, new_domain: Any, new_soa: Any, new_description: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for domain update. Reports the domain-name and SOA
    email changes against the fetched state and notes a description change.
    """
    side_effects: list[str] = []
    if new_domain:
        from_domain = getattr(state, "domain", "")
        if from_domain and from_domain != new_domain:
            side_effects.append(
                f"Domain name changes from {from_domain!r} to {new_domain!r}."
            )
        else:
            side_effects.append(f"Domain name is set to {new_domain!r}.")
    if new_soa:
        from_soa = getattr(state, "soa_email", "")
        if new_soa != from_soa:
            side_effects.append(f"SOA email is set to {new_soa!r}.")
    if new_description:
        side_effects.append("The domain description is updated.")
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_domain_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_update tool request."""
    domain_id = arguments.get("domain_id", 0)

    if is_dry_run(arguments):
        if not domain_id:
            return error_response("domain_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain(int(domain_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _domain_update_side_effects(
                state,
                arguments.get("domain"),
                arguments.get("soa_email"),
                arguments.get("description"),
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_update",
            "PUT",
            f"/domains/{int(domain_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This updates a DNS domain. Set confirm=true to proceed.")

    if not domain_id:
        return error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.update_domain(
            domain_id=int(domain_id),
            domain=arguments.get("domain"),
            soa_email=arguments.get("soa_email"),
            description=arguments.get("description"),
        )
        return {
            "message": f"Domain {domain_id} updated successfully",
            "domain": {
                "id": domain.id,
                "domain": domain.domain,
                "type": domain.type,
                "status": domain.status,
                "updated": domain.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update domain", _call)


def create_linode_domain_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_delete tool."""
    return Tool(
        name="linode_domain_delete",
        description=(
            "Deletes a DNS domain. WARNING: This also deletes all associated "
            "records. Pass dry_run=true to preview without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain_id", "confirm"],
        },
    ), Capability.Destroy


async def _domain_delete_dependency_walk(
    client: RetryableClient, domain_id: int
) -> DryRunDetails:
    """Phase 2 Tier A walk for domain delete. Deleting a domain destroys all
    its DNS records; the walk surfaces the NS records (the delegation that
    breaks) as cascade_deleted dependencies and warns with the total record
    count. Best-effort: a failed record list becomes a warning, not a hard
    error.
    """
    details: DryRunDetails = {}
    try:
        records = await client.list_domain_records(domain_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list domain records: {exc}"]
        return details

    dependencies: list[dict[str, Any]] = []
    ns_count = 0
    for record in records:
        if record.type.upper() != "NS":
            continue
        ns_count += 1
        dependencies.append(
            {
                "kind": "ns_record",
                "label": record.target,
                "action": "cascade_deleted",
                "note": f"NS record for {record.name}",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
    if records:
        details["warnings"] = [
            f"Deleting this domain destroys {len(records)} DNS record(s), "
            f"including {ns_count} NS record(s)."
        ]
    return details


async def handle_linode_domain_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_delete tool request."""
    domain_id = arguments.get("domain_id", 0)

    if is_dry_run(arguments):
        if not domain_id:
            return error_response("domain_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain(int(domain_id))

        async def _walk(client: RetryableClient, _state: Any) -> DryRunDetails:
            return await _domain_delete_dependency_walk(client, int(domain_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_delete",
            "DELETE",
            f"/domains/{int(domain_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This is destructive. Set confirm=true to proceed.")

    if not domain_id:
        return error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain(int(domain_id))
        return {
            "message": f"Domain {domain_id} deleted successfully",
            "domain_id": domain_id,
        }

    return await execute_tool(cfg, arguments, "delete domain", _call)

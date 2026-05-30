from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    PARAM_DRY_RUN,
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
        return build_dry_run_response(
            "linode_domain_create",
            arguments.get("environment", ""),
            "POST",
            "/domains",
            None,
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

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_update",
            "PUT",
            f"/domains/{int(domain_id)}",
            _fetch,
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

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_delete",
            "DELETE",
            f"/domains/{int(domain_id)}",
            _fetch,
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

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_domain_create_tool() -> Tool:
    """Create the linode_domain_create tool."""
    return Tool(
        name="linode_domain_create",
        description="Creates a new DNS domain.",
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
            },
            "required": ["domain"],
        },
    )


async def handle_linode_domain_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_create tool request."""
    domain_name = arguments.get("domain", "")

    if not domain_name:
        return _error_response("domain is required")

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


def create_linode_domain_update_tool() -> Tool:
    """Create the linode_domain_update tool."""
    return Tool(
        name="linode_domain_update",
        description="Updates an existing DNS domain.",
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
            },
            "required": ["domain_id"],
        },
    )


async def handle_linode_domain_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_update tool request."""
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")

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


def create_linode_domain_delete_tool() -> Tool:
    """Create the linode_domain_delete tool."""
    return Tool(
        name="linode_domain_delete",
        description=(
            "Deletes a DNS domain. WARNING: This also deletes all associated records."
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
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["domain_id", "confirm"],
        },
    )


async def handle_linode_domain_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_delete tool request."""
    domain_id = arguments.get("domain_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not domain_id:
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain(int(domain_id))
        return {
            "message": f"Domain {domain_id} deleted successfully",
            "domain_id": domain_id,
        }

    return await execute_tool(cfg, arguments, "delete domain", _call)

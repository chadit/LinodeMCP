"""Linode account tool - authenticated user account information."""

from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool


def create_linode_account_tool() -> tuple[Tool, Capability]:
    """Create the linode_account tool."""
    return Tool(
        name="linode_account",
        description=(
            "Retrieves the authenticated user's Linode account information "
            "including billing details and capabilities"
        ),
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_account(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account tool request.

    Args:
        arguments: EnvironmentArgs - environment (optional)
        cfg: Configuration object
    """

    async def _call(client: RetryableClient) -> dict[str, Any]:
        account = await client.get_account()
        return {
            "first_name": account.first_name,
            "last_name": account.last_name,
            "email": account.email,
            "company": account.company,
            "balance": account.balance,
            "balance_uninvoiced": account.balance_uninvoiced,
            "capabilities": account.capabilities,
            "active_since": account.active_since,
        }

    return await execute_tool(cfg, arguments, "retrieve Linode account", _call)


def create_linode_account_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_update tool."""
    return Tool(
        name="linode_account_update",
        description=("Updates Linode account contact and billing-address information."),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "first_name": {"type": "string", "description": "First name"},
                "last_name": {"type": "string", "description": "Last name"},
                "email": {"type": "string", "description": "Contact email"},
                "company": {"type": "string", "description": "Company name"},
                "address_1": {"type": "string", "description": "Address line 1"},
                "address_2": {"type": "string", "description": "Address line 2"},
                "city": {"type": "string", "description": "City"},
                "state": {"type": "string", "description": "State or province"},
                "zip": {"type": "string", "description": "Postal code"},
                "country": {"type": "string", "description": "Country code"},
                "phone": {"type": "string", "description": "Phone number"},
                "tax_id": {"type": "string", "description": "Tax ID"},
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_account_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_update tool request."""
    if not arguments.get("confirm"):
        return error_response(
            "This updates account information. Set confirm=true to proceed."
        )

    update_fields = {
        key: arguments.get(key)
        for key in (
            "first_name",
            "last_name",
            "email",
            "company",
            "address_1",
            "address_2",
            "city",
            "state",
            "zip",
            "country",
            "phone",
            "tax_id",
        )
        if arguments.get(key) is not None
    }
    if not update_fields:
        return error_response("At least one account field is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        account = await client.update_account(**update_fields)
        return {
            "message": "Account updated successfully",
            "account": {
                "first_name": account.first_name,
                "last_name": account.last_name,
                "email": account.email,
                "company": account.company,
                "address_1": account.address_1,
                "address_2": account.address_2,
                "city": account.city,
                "state": account.state,
                "zip": account.zip,
                "country": account.country,
                "phone": account.phone,
            },
        }

    return await execute_tool(cfg, arguments, "update Linode account", _call)

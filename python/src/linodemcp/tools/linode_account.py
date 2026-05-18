"""Linode account tool - authenticated user account information."""

from typing import Any, cast

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


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_account_tags_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tags_list tool."""
    return Tool(
        name="linode_account_tags_list",
        description="Lists tags on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_tags_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tags_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_tags(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account tags", _call)


def create_linode_account_tag_objects_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_objects_list tool."""
    return Tool(
        name="linode_account_tag_objects_list",
        description="Lists objects assigned to a Linode account tag.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tag_label": {
                    "type": "string",
                    "description": "Label of the tag to inspect",
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["tag_label"],
        },
    ), Capability.Read


async def handle_linode_account_tag_objects_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_objects_list tool request."""
    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_tagged_objects(
            tag_label, page=page, page_size=page_size
        )

    return await execute_tool(cfg, arguments, "list tagged objects", _call)


def create_linode_account_tag_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_create tool."""
    return Tool(
        name="linode_account_tag_create",
        description="Creates a Linode account tag and optionally assigns resources.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {"type": "string", "description": "Tag label to create"},
                "domains": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Domain IDs to assign to the tag",
                },
                "linodes": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Linode IDs to assign to the tag",
                },
                "nodebalancers": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "NodeBalancer IDs to assign to the tag",
                },
                "volumes": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Volume IDs to assign to the tag",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


def _optional_int_list_argument(
    arguments: dict[str, Any], name: str
) -> list[int] | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, list):
        raise TypeError(f"{name} must be a list of integers")
    values: list[int] = []
    for item in cast("list[object]", value):
        if not isinstance(item, int) or isinstance(item, bool):
            raise TypeError(f"{name} must be a list of integers")
        if item < 1:
            raise ValueError(f"{name} must contain positive integers")
        values.append(item)
    if not values:
        return None
    return values


async def handle_linode_account_tag_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_create tool request."""
    if arguments.get("confirm") is not True:
        return error_response("This creates a tag. Set confirm=true to proceed.")

    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return error_response("label is required")

    try:
        resource_ids = {
            name: _optional_int_list_argument(arguments, name)
            for name in ("domains", "linodes", "nodebalancers", "volumes")
        }
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        tag = await client.create_tag(label.strip(), **resource_ids)
        return {"message": f"Tag '{label.strip()}' created successfully", "tag": tag}

    return await execute_tool(cfg, arguments, "create Linode tag", _call)


def create_linode_account_tag_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_delete tool."""
    return Tool(
        name="linode_account_tag_delete",
        description="Deletes a Linode account tag by label.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tag_label": {
                    "type": "string",
                    "description": "Label of the tag to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
            },
            "required": ["tag_label", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_account_tag_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_delete tool request."""
    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")
    if not arguments.get("confirm"):
        return error_response("This deletes a tag. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_tag(tag_label)
        return {"message": f"Tag '{tag_label}' deleted successfully"}

    return await execute_tool(cfg, arguments, "delete Linode tag", _call)
